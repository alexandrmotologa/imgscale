package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// S3Config holds S3 / Cloudflare R2 / MinIO connection parameters.
type S3Config struct {
	Endpoint        string // e.g. "https://s3.amazonaws.com" or "https://<account>.r2.cloudflarestorage.com"
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	PathStyle       bool // Force path-style (http://endpoint/bucket/key) vs virtual-host (http://bucket.endpoint/key)
}

// S3Storage provides read access to S3-compatible object stores.
type S3Storage struct {
	cfg        S3Config
	httpClient *http.Client
}

// NewS3Storage initializes an S3-compatible storage backend.
func NewS3Storage(cfg S3Config, timeout time.Duration) *S3Storage {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://s3.amazonaws.com"
	}

	return &S3Storage{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// buildURL constructs the target S3 object URL.
func (s *S3Storage) buildURL(key string) (*url.URL, error) {
	cleanKey := strings.TrimPrefix(key, "/")

	ep := strings.TrimRight(s.cfg.Endpoint, "/")
	parsed, err := url.Parse(ep)
	if err != nil {
		return nil, err
	}

	if s.cfg.PathStyle || strings.Contains(parsed.Host, "localhost") || strings.Contains(parsed.Host, "127.0.0.1") {
		// Path style: endpoint/bucket/key
		parsed.Path = "/" + s.cfg.Bucket + "/" + cleanKey
	} else {
		// Virtual host style: bucket.endpoint/key
		parsed.Host = s.cfg.Bucket + "." + parsed.Host
		parsed.Path = "/" + cleanKey
	}

	return parsed, nil
}

// signV4 signs the HTTP request using standard AWS Signature Version 4.
func (s *S3Storage) signV4(req *http.Request, now time.Time) {
	if s.cfg.AccessKeyID == "" || s.cfg.SecretAccessKey == "" {
		return // anonymous access
	}

	dateStamp := now.UTC().Format("20060102")
	amzDate := now.UTC().Format("20060102T150405Z")

	req.Header.Set("x-amz-date", amzDate)
	payloadHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // SHA256 of empty body
	req.Header.Set("x-amz-content-sha256", payloadHash)

	// Canonical headers
	host := req.URL.Host
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", host, payloadHash, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	canonicalRequestHash := sha256Hex([]byte(canonicalRequest))

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.cfg.Region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		canonicalRequestHash,
	)

	// Derive signing key
	kSecret := []byte("AWS4" + s.cfg.SecretAccessKey)
	kDate := hmacSHA256(kSecret, []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(s.cfg.Region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))

	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cfg.AccessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)

	req.Header.Set("Authorization", authHeader)
}

// Get fetches the object from the S3 bucket.
func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	u, err := s.buildURL(key)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}

	s.signV4(req, time.Now())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, 0, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("s3 upstream returned HTTP %d", resp.StatusCode)
	}

	return resp.Body, resp.ContentLength, nil
}

// Exists checks object existence with HEAD.
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	u, err := s.buildURL(key)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
	if err != nil {
		return false, err
	}

	s.signV4(req, time.Now())

	resp, err := s.httpClient.Do(req)
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
	return false, fmt.Errorf("s3 check returned HTTP %d", resp.StatusCode)
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
