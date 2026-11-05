package server

import (
	"encoding/json"
	"fmt"
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

	opts := engine.TransformOptions{
		Width:   reqW,
		Height:  reqH,
		Fit:     fit,
		Gravity: gravity,
		Format:  resolvedFormat,
		Quality: quality,
		DPR:     dpr,
		Filters: engine.FilterOptions{
			Blur:      blur,
			Sharpen:   sharpen,
			Grayscale: gray,
			Rotate:    rot,
		},
		Debug:  debug,
		Accept: acceptHeader,
	}

	// Canonical cache key
	cacheKey := fmt.Sprintf("%s|w=%d&h=%d&fit=%s&g=%s&fmt=%s&q=%d&dpr=%.2f&b=%.1f&sh=%.1f&gray=%t&rot=%d&dbg=%s",
		path, reqW, reqH, fit, gravity, resolvedFormat, quality, dpr, blur, sharpen, gray, rot, debug)

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
					presets = append(presets, PresetItem{
						ID:       rel,
						Name:     filepath.Base(p),
						Category: "local",
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
