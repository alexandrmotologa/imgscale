package storage

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "imgscale-storage-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create test file
	testData := []byte("fake image data 12345")
	testPath := filepath.Join(tempDir, "sample.jpg")
	if err := os.WriteFile(testPath, testData, 0644); err != nil {
		t.Fatal(err)
	}

	storage, err := NewLocalStorage(tempDir)
	if err != nil {
		t.Fatalf("NewLocalStorage failed: %v", err)
	}

	ctx := context.Background()

	// 1. Successful read
	rc, size, err := storage.Get(ctx, "sample.jpg")
	if err != nil {
		t.Fatalf("Failed to get sample.jpg: %v", err)
	}
	defer rc.Close()

	if size != int64(len(testData)) {
		t.Errorf("Expected size %d, got %d", len(testData), size)
	}
	content, _ := io.ReadAll(rc)
	if string(content) != string(testData) {
		t.Errorf("Content mismatch: %s != %s", string(content), string(testData))
	}

	// 2. Traversal attempt
	_, _, err = storage.Get(ctx, "../secret.txt")
	if err != ErrAccessDenied {
		t.Errorf("Expected ErrAccessDenied, got %v", err)
	}

	// 3. Not found
	_, _, err = storage.Get(ctx, "nonexistent.png")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}

	// 4. Exists
	exists, err := storage.Exists(ctx, "sample.jpg")
	if err != nil || !exists {
		t.Errorf("Expected sample.jpg to exist, got %v, err=%v", exists, err)
	}
	exists, err = storage.Exists(ctx, "nonexistent.png")
	if err != nil || exists {
		t.Errorf("Expected nonexistent.png not to exist, got %v, err=%v", exists, err)
	}
}

func TestSSRFPolicy(t *testing.T) {
	// 1. IP restrictions
	restrictedIPs := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254",
		"169.254.1.1",
		"::1",
		"0.0.0.0",
	}

	for _, ipStr := range restrictedIPs {
		ip := net.ParseIP(ipStr)
		if !IsRestrictedIP(ip) {
			t.Errorf("Expected IP %s to be restricted", ipStr)
		}
	}

	publicIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"140.82.121.3", // github
	}

	for _, ipStr := range publicIPs {
		ip := net.ParseIP(ipStr)
		if IsRestrictedIP(ip) {
			t.Errorf("Expected IP %s to be allowed", ipStr)
		}
	}

	// 2. Domain whitelist
	policy := SSRFPolicy{
		AllowedDomains: []string{
			"images.unsplash.com",
			"*.example.org",
		},
	}

	if !policy.IsDomainAllowed("images.unsplash.com") {
		t.Error("Expected images.unsplash.com to be allowed")
	}
	if !policy.IsDomainAllowed("cdn.example.org") {
		t.Error("Expected cdn.example.org to match *.example.org")
	}
	if policy.IsDomainAllowed("evil.com") {
		t.Error("Expected evil.com to be rejected")
	}
}

func TestHTTPStorage(t *testing.T) {
	// Setup test HTTP server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test.png" {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("image-data-payload"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// When testing against httptest loopback server, we test HTTPStorage with custom transport or direct client
	client := srv.Client()
	httpStore := &HTTPStorage{
		baseURL:  srv.URL,
		client:   client,
		maxBytes: 1024 * 1024,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rc, size, err := httpStore.Get(ctx, "test.png")
	if err != nil {
		t.Fatalf("Failed to fetch test.png: %v", err)
	}
	defer rc.Close()

	data, _ := io.ReadAll(rc)
	if string(data) != "image-data-payload" {
		t.Errorf("Payload mismatch: %s", string(data))
	}
	if size != int64(len("image-data-payload")) {
		t.Errorf("Size mismatch: %d", size)
	}
}
