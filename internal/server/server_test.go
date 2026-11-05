package server

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/imgscale/internal/cache"
	"github.com/alexandrmotologa/imgscale/internal/config"
	"github.com/alexandrmotologa/imgscale/internal/storage"
)

func createTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func setupTestServer(t *testing.T, secret string, enforceSig bool) (http.Handler, string, func()) {
	tempDir, err := os.MkdirTemp("", "imgscale-server-test-*")
	if err != nil {
		t.Fatal(err)
	}

	imgDir := filepath.Join(tempDir, "images")
	cacheDir := filepath.Join(tempDir, "cache")
	_ = os.MkdirAll(imgDir, 0755)
	_ = os.MkdirAll(cacheDir, 0755)

	// Write sample image
	raw := createTestPNG(400, 300)
	if err := os.WriteFile(filepath.Join(imgDir, "sample.png"), raw, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 8080, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second},
		Storage: config.StorageConfig{
			Backend: "local",
			Local:   config.LocalStorageConfig{RootDir: imgDir},
		},
		Cache: config.CacheConfig{
			L1MaxBytes: 10 * 1024 * 1024,
			L2Dir:      cacheDir,
			L2MaxBytes: 50 * 1024 * 1024,
			L2MaxTTL:   24 * time.Hour,
		},
		Security: config.SecurityConfig{
			EnforceSignature: enforceSig,
			Secret:           secret,
		},
		Limits: config.LimitsConfig{
			MaxWidth: 4096, MaxHeight: 4096, MaxPayloadBytes: 10 * 1024 * 1024,
		},
	}

	store, err := storage.NewLocalStorage(imgDir)
	if err != nil {
		t.Fatal(err)
	}

	tc, err := cache.NewTwoTierCache(cache.Config{
		L1MaxBytes: cfg.Cache.L1MaxBytes,
		L2Dir:      cfg.Cache.L2Dir,
		L2MaxBytes: cfg.Cache.L2MaxBytes,
		L2MaxTTL:   cfg.Cache.L2MaxTTL,
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := NewGatewayHandler(cfg, store, tc)
	router := SetupRouter(handler, nil)

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return router, imgDir, cleanup
}

func TestHealthAndStats(t *testing.T) {
	router, _, cleanup := setupTestServer(t, "", false)
	defer cleanup()

	// 1. Healthz
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 for healthz, got %d", rr.Code)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if health["status"] != "ok" {
		t.Errorf("Expected status=ok, got %v", health["status"])
	}

	// 2. Stats
	req = httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 for stats, got %d", rr.Code)
	}
}

func TestImageTransformationAndCaching(t *testing.T) {
	router, _, cleanup := setupTestServer(t, "", false)
	defer cleanup()

	// 1. Initial Transformation Request
	req := httptest.NewRequest(http.MethodGet, "/v1/images/sample.png?w=200&h=150&fit=cover&format=webp", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Content-Type") != "image/webp" {
		t.Errorf("Expected image/webp, got %s", rr.Header().Get("Content-Type"))
	}
	etag := rr.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header")
	}
	cacheHeader := rr.Header().Get("X-Cache")
	if cacheHeader != "ORIGIN_COMPUTE" {
		t.Errorf("Expected X-Cache=ORIGIN_COMPUTE on first run, got %s", cacheHeader)
	}

	// 2. Second request should hit cache
	req = httptest.NewRequest(http.MethodGet, "/v1/images/sample.png?w=200&h=150&fit=cover&format=webp", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Header().Get("X-Cache") != "L1_MEMORY" {
		t.Errorf("Expected X-Cache=L1_MEMORY on second run, got %s", rr.Header().Get("X-Cache"))
	}

	// 3. Conditional 304 Not Modified
	req = httptest.NewRequest(http.MethodGet, "/v1/images/sample.png?w=200&h=150&fit=cover&format=webp", nil)
	req.Header.Set("If-None-Match", etag)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotModified {
		t.Errorf("Expected 304 Not Modified, got %d", rr.Code)
	}
}

func TestHMACSignatureEnforcement(t *testing.T) {
	secret := "test-secret-key-1234"
	router, _, cleanup := setupTestServer(t, secret, true)
	defer cleanup()

	// 1. Unsigned request should fail with 403 Forbidden
	req := httptest.NewRequest(http.MethodGet, "/v1/images/sample.png?w=100&h=100", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden without signature, got %d", rr.Code)
	}

	// 2. Signed request should succeed
	signedURL, err := SignURL("http://localhost/v1/images/sample.png?w=100&h=100", secret)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(signedURL)

	req = httptest.NewRequest(http.MethodGet, u.RequestURI(), nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 with valid signature, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Tampered parameter should fail
	tamperedURI := u.RequestURI() + "0"
	req = httptest.NewRequest(http.MethodGet, tamperedURI, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden with tampered query, got %d", rr.Code)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	router, _, cleanup := setupTestServer(t, "", false)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 for /metrics, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !bytes.Contains([]byte(body), []byte("imgscale_uptime_seconds")) {
		t.Errorf("Expected Prometheus metrics output, got: %s", body)
	}
}

func TestPurgeEndpoint(t *testing.T) {
	router, _, cleanup := setupTestServer(t, "", false)
	defer cleanup()

	// 1. Populate cache
	req := httptest.NewRequest(http.MethodGet, "/v1/images/sample.png?w=100&h=100", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// 2. Purge
	reqPurge := httptest.NewRequest(http.MethodPost, "/api/cache/purge?path=sample.png", nil)
	rrPurge := httptest.NewRecorder()
	router.ServeHTTP(rrPurge, reqPurge)

	if rrPurge.Code != http.StatusOK {
		t.Fatalf("Expected 200 for purge, got %d", rrPurge.Code)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rrPurge.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if res["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", res["status"])
	}
}
