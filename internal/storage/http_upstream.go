package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPStorage fetches origin images via HTTP/HTTPS with SSRF prevention.
type HTTPStorage struct {
	baseURL    string
	client     *http.Client
	maxBytes   int64
}

// NewHTTPStorage initializes an HTTPStorage backend.
func NewHTTPStorage(baseURL string, policy SSRFPolicy, maxBytes int64, timeout time.Duration) *HTTPStorage {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if maxBytes <= 0 {
		maxBytes = 50 * 1024 * 1024 // 50MB default limit
	}

	return &HTTPStorage{
		baseURL:  strings.TrimRight(baseURL, "/"),
		client:   NewSafeHTTPClient(policy, timeout),
		maxBytes: maxBytes,
	}
}

// buildURL converts key into an absolute HTTP URL.
func (s *HTTPStorage) buildURL(key string) (string, error) {
	cleanKey := strings.TrimPrefix(key, "/")

	// If key is already an absolute HTTP/HTTPS URL
	if strings.HasPrefix(cleanKey, "http://") || strings.HasPrefix(cleanKey, "https://") {
		parsed, err := url.Parse(cleanKey)
		if err != nil {
			return "", err
		}
		return parsed.String(), nil
	}

	if s.baseURL == "" {
		return "", fmt.Errorf("relative image path %q requested but no base upstream URL configured", key)
	}

	return s.baseURL + "/" + cleanKey, nil
}

// Get fetches the image from the remote origin.
func (s *HTTPStorage) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	targetURL, err := s.buildURL(key)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "imgscale-gateway/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, 0, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	contentLength := resp.ContentLength
	if contentLength > s.maxBytes {
		resp.Body.Close()
		return nil, 0, ErrPayloadTooLarge
	}

	// Wrap body in LimitReader to guard against chunked transfer encoding abuse
	limitedReader := io.LimitReader(resp.Body, s.maxBytes+1)
	return &limitedReadCloser{
		Reader: limitedReader,
		Closer: resp.Body,
		limit:  s.maxBytes,
	}, contentLength, nil
}

// Exists sends a HEAD request to check asset existence without downloading the payload.
func (s *HTTPStorage) Exists(ctx context.Context, key string) (bool, error) {
	targetURL, err := s.buildURL(key)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "imgscale-gateway/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("upstream check returned status %d", resp.StatusCode)
}

type limitedReadCloser struct {
	io.Reader
	io.Closer
	read  int64
	limit int64
}

func (l *limitedReadCloser) Read(p []byte) (int, error) {
	n, err := l.Reader.Read(p)
	l.read += int64(n)
	if l.read > l.limit {
		return n, ErrPayloadTooLarge
	}
	return n, err
}
