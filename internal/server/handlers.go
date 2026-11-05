package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/imgscale/internal/cache"
	"github.com/alexandrmotologa/imgscale/internal/config"
	"github.com/alexandrmotologa/imgscale/internal/engine"
	"github.com/alexandrmotologa/imgscale/internal/storage"
)

// GatewayHandler holds shared runtime dependencies for HTTP endpoints.
type GatewayHandler struct {
	cfg       *config.Config
	storage   storage.StorageSource
	cache     *cache.TwoTierCache
	startTime time.Time
}

// NewGatewayHandler initializes the HTTP gateway handler state.
func NewGatewayHandler(cfg *config.Config, store storage.StorageSource, tc *cache.TwoTierCache) *GatewayHandler {
	return &GatewayHandler{
		cfg:       cfg,
		storage:   store,
		cache:     tc,
		startTime: time.Now(),
	}
}

// HandleImage handles dynamic image transformation requests at /v1/images/*
func (h *GatewayHandler) HandleImage(w http.ResponseWriter, r *http.Request) {
	// Extract key/path after /v1/images/ or /v1/entropy/
	path := strings.TrimPrefix(r.URL.Path, "/v1/images/")
	path = strings.TrimPrefix(path, "/v1/entropy/")
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		http.Error(w, "missing image path in URL", http.StatusBadRequest)
		return
	}

	q := r.URL.Query()

	// Parse dimensions
	reqW := parseQueryInt(q, "w", 0)
	reqH := parseQueryInt(q, "h", 0)
	if reqW > h.cfg.Limits.MaxWidth {
		reqW = h.cfg.Limits.MaxWidth
	}
	if reqH > h.cfg.Limits.MaxHeight {
		reqH = h.cfg.Limits.MaxHeight
	}

	// Parse parameters
	fit := engine.FitMode(strings.ToLower(q.Get("fit")))
	if fit == "" {
		fit = engine.FitCover
	}

	gravity := engine.ParseGravity(q.Get("gravity"))
	fpX := parseQueryFloat(q, "fp_x", 0.5)
	fpY := parseQueryFloat(q, "fp_y", 0.5)

	reqFormat := engine.ParseFormat(q.Get("format"))
	acceptHeader := r.Header.Get("Accept")
	resolvedFormat := engine.NegotiateFormat(reqFormat, acceptHeader)

	quality := parseQueryInt(q, "q", 80)
	dpr := parseQueryFloat(q, "dpr", 1.0)
	blur := parseQueryFloat(q, "blur", 0.0)
	sharpen := parseQueryFloat(q, "sharpen", 0.0)
	gray := parseQueryBool(q, "gray", false)
	rot := parseQueryInt(q, "rot", 0)
	debug := q.Get("debug")
	lqip := q.Get("lqip")
	autoOrient := parseQueryBool(q, "auto_orient", true)

	// Watermarking parameters
	wmImage := q.Get("wm_image")
	wmGravity := engine.ParseGravity(q.Get("wm_gravity"))
	wmOpacity := parseQueryFloat(q, "wm_opacity", 0.85)
	wmScale := parseQueryFloat(q, "wm_scale", 20.0)
	wmPad := parseQueryInt(q, "wm_pad", 16)

	opts := engine.TransformOptions{
		Width:       reqW,
		Height:      reqH,
		Fit:         fit,
		Gravity:     gravity,
		FocalPointX: fpX,
		FocalPointY: fpY,
		Format:      resolvedFormat,
		Quality:     quality,
		DPR:         dpr,
		Filters: engine.FilterOptions{
			Blur:      blur,
			Sharpen:   sharpen,
			Grayscale: gray,
			Rotate:    rot,
		},
		LQIP:       lqip,
		AutoOrient: autoOrient,
		Debug:      debug,
		Accept:     acceptHeader,
	}

	// Canonical cache key
	cacheKey := fmt.Sprintf("%s|w=%d&h=%d&fit=%s&g=%s&fpx=%.2f&fpy=%.2f&fmt=%s&q=%d&dpr=%.2f&b=%.1f&sh=%.1f&gray=%t&rot=%d&dbg=%s&lqip=%s&ao=%t&wm=%s&wmg=%s&wmo=%.2f&wms=%.1f&wmp=%d",
		path, reqW, reqH, fit, gravity, fpX, fpY, resolvedFormat, quality, dpr, blur, sharpen, gray, rot, debug, lqip, autoOrient, wmImage, wmGravity, wmOpacity, wmScale, wmPad)

	etag := cache.GenerateETag(path, cacheKey, 0)

	// RFC 9111 ETag evaluation
	if cache.CheckNoneMatch(r.Header.Get("If-None-Match"), etag) {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	start := time.Now()

	data, contentType, cacheSource, err := h.cache.GetOrCompute(r.Context(), cacheKey, func() ([]byte, string, error) {
		// Cache MISS: Fetch from upstream storage
		rc, size, err := h.storage.Get(r.Context(), path)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()

		if size > h.cfg.Limits.MaxPayloadBytes {
			return nil, "", storage.ErrPayloadTooLarge
		}

		// If watermark is requested, fetch and decode watermark overlay image
		if wmImage != "" {
			wmRc, _, wmErr := h.storage.Get(r.Context(), wmImage)
			if wmErr == nil {
				defer wmRc.Close()
				if wmImg, _, decErr := engine.DecodeImage(wmRc); decErr == nil {
					opts.Watermark = engine.WatermarkOptions{
						Watermark: wmImg,
						Gravity:   wmGravity,
						Opacity:   wmOpacity,
						Scale:     wmScale,
						Padding:   wmPad,
					}
				}
			}
		}

		res, err := engine.Transform(rc, opts)
		if err != nil {
			return nil, "", err
		}

		return res.Data, res.ContentType, nil
	})

	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "origin image not found", http.StatusNotFound)
			return
		}
		if err == storage.ErrAccessDenied || err == storage.ErrSSRFBlocked {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, fmt.Sprintf("transformation failed: %v", err), http.StatusInternalServerError)
		return
	}

	duration := time.Since(start)

	// Response Headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Cache", string(cacheSource))
	w.Header().Set("X-Transform-Duration-Ms", fmt.Sprintf("%.2f", float64(duration.Microseconds())/1000.0))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// HandleEntropy shortcut endpoint to inspect Sobel edge entropy map.
func (h *GatewayHandler) HandleEntropy(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	q.Set("debug", "entropy")
	r.URL.RawQuery = q.Encode()
	h.HandleImage(w, r)
}

// HandleHealth provides a liveness and readiness probe.
func (h *GatewayHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"uptime_sec": int(time.Since(h.startTime).Seconds()),
		"version":    "1.0.0",
	})
}

// HandleStats returns real-time cache and memory metrics.
func (h *GatewayHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	telemetry := h.cache.Telemetry()

	resp := map[string]interface{}{
		"uptime_sec": int(time.Since(h.startTime).Seconds()),
		"cache":      telemetry,
		"system": map[string]interface{}{
			"alloc_mb":       float64(m.Alloc) / (1024 * 1024),
			"sys_mb":         float64(m.Sys) / (1024 * 1024),
			"heap_in_use_mb": float64(m.HeapInuse) / (1024 * 1024),
			"num_goroutine":  runtime.NumGoroutine(),
			"num_gc":         m.NumGC,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleMetrics outputs standard Prometheus / OpenMetrics scraping format.
func (h *GatewayHandler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	telemetry := h.cache.Telemetry()
	uptime := int(time.Since(h.startTime).Seconds())

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	out := fmt.Sprintf(`# HELP imgscale_uptime_seconds Total daemon uptime in seconds
# TYPE imgscale_uptime_seconds gauge
imgscale_uptime_seconds %d

# HELP imgscale_cache_hits_total Cache hit count by tier
# TYPE imgscale_cache_hits_total counter
imgscale_cache_hits_total{tier="l1"} %d
imgscale_cache_hits_total{tier="l2"} %d

# HELP imgscale_cache_misses_total Cache miss count by tier
# TYPE imgscale_cache_misses_total counter
imgscale_cache_misses_total{tier="l1"} %d
imgscale_cache_misses_total{tier="l2"} %d

# HELP imgscale_cache_bytes Memory or disk footprint in bytes
# TYPE imgscale_cache_bytes gauge
imgscale_cache_bytes{tier="l1"} %d
imgscale_cache_bytes{tier="l2"} %d

# HELP imgscale_cache_items Stored items count
# TYPE imgscale_cache_items gauge
imgscale_cache_items{tier="l1"} %d
imgscale_cache_items{tier="l2"} %d

# HELP imgscale_memory_alloc_bytes Process memory allocation in bytes
# TYPE imgscale_memory_alloc_bytes gauge
imgscale_memory_alloc_bytes %d

# HELP imgscale_goroutines Current active goroutines
# TYPE imgscale_goroutines gauge
imgscale_goroutines %d
`,
		uptime,
		telemetry.L1.Hits,
		telemetry.L2.Hits,
		telemetry.L1.Misses,
		telemetry.L2.Misses,
		telemetry.L1.TotalBytes,
		telemetry.L2.TotalBytes,
		telemetry.L1.ItemCount,
		telemetry.L2.FileCount,
		m.Alloc,
		runtime.NumGoroutine(),
	)

	_, _ = w.Write([]byte(out))
}

// HandlePurge invalidates cached images across L1 and L2 by path prefix.
func (h *GatewayHandler) HandlePurge(w http.ResponseWriter, r *http.Request) {
	targetPath := r.URL.Query().Get("path")

	if r.Method == http.MethodPost && targetPath == "" {
		var req struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		targetPath = req.Path
	}

	purgedCount, err := h.cache.Purge(targetPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("purge failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ok",
		"target_path":  targetPath,
		"purged_count": purgedCount,
	})
}

// HandleUpload allows web studio users to upload custom images into storage.
func (h *GatewayHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 32MB max upload memory
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		file, header, err = r.FormFile("file")
		if err != nil {
			http.Error(w, "missing 'image' or 'file' form field", http.StatusBadRequest)
			return
		}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read uploaded file", http.StatusInternalServerError)
		return
	}

	// Validate that file is a valid image
	img, detectedFmt, err := engine.DecodeImage(strings.NewReader(string(data)))
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid or unsupported image format: %v", err), http.StatusBadRequest)
		return
	}

	// Save into local storage root uploads/
	uploadDir := filepath.Join(h.cfg.Storage.Local.RootDir, "uploads")
	_ = os.MkdirAll(uploadDir, 0755)

	hash := sha256.Sum256(data)
	hashName := hex.EncodeToString(hash[:8])
	cleanExt := detectedFmt.Extension()
	fileName := fmt.Sprintf("%s_%s.%s", time.Now().Format("20060102150405"), hashName, cleanExt)
	destPath := filepath.Join(uploadDir, fileName)

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		http.Error(w, fmt.Sprintf("failed to save uploaded file: %v", err), http.StatusInternalServerError)
		return
	}

	relPath := "uploads/" + fileName
	bounds := img.Bounds()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"path":     relPath,
		"name":     header.Filename,
		"format":   string(detectedFmt),
		"width":    bounds.Dx(),
		"height":   bounds.Dy(),
		"size":     len(data),
	})
}

// HandleSign generates a signed URL given path and parameters.
func (h *GatewayHandler) HandleSign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL    string `json:"url"`
		Secret string `json:"secret"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	secret := req.Secret
	if secret == "" {
		secret = h.cfg.Security.Secret
	}
	if secret == "" {
		http.Error(w, "no signing secret provided or configured", http.StatusBadRequest)
		return
	}

	signedURL, err := SignURL(req.URL, secret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parsed, _ := url.Parse(signedURL)
	sig := parsed.Query().Get("s")

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"signed_url": signedURL,
		"signature":  sig,
	})
}

// HandlePresets returns test images available in local storage for the studio cockpit.
func (h *GatewayHandler) HandlePresets(w http.ResponseWriter, r *http.Request) {
	type PresetItem struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Category string `json:"category"`
		Path     string `json:"path"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
	}

	var presets []PresetItem

	// Scan storage directory for sample files
	if h.cfg.Storage.Backend == "local" {
		root := h.cfg.Storage.Local.RootDir
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				ext := strings.ToLower(filepath.Ext(p))
				if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
					rel, _ := filepath.Rel(root, p)
					rel = filepath.ToSlash(rel)
					category := "testdata"
					if strings.HasPrefix(rel, "uploads/") {
						category = "uploads"
					}
					presets = append(presets, PresetItem{
						ID:       rel,
						Name:     filepath.Base(p),
						Category: category,
						Path:     rel,
					})
				}
			}
			return nil
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(presets)
}

func parseQueryInt(q url.Values, key string, defaultVal int) int {
	if val := q.Get(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func parseQueryFloat(q url.Values, key string, defaultVal float64) float64 {
	if val := q.Get(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

func parseQueryBool(q url.Values, key string, defaultVal bool) bool {
	if val := q.Get(key); val != "" {
		v := strings.ToLower(val)
		return v == "1" || v == "true" || v == "yes"
	}
	return defaultVal
}
