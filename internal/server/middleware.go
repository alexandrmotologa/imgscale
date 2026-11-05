package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// BuildSigningPayload constructs canonical signing string "{path}?{sorted_query_without_s}".
func BuildSigningPayload(path string, query url.Values) string {
	cleanPath := path
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	keys := make([]string, 0, len(query))
	for k := range query {
		if k != "s" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}

	if len(pairs) == 0 {
		return cleanPath
	}
	return cleanPath + "?" + strings.Join(pairs, "&")
}

// ComputeSignature calculates the HMAC-SHA256 hex digest for a given payload and secret.
func ComputeSignature(payload, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// SignURL appends a valid &s= parameter to target URL.
func SignURL(rawURL, secret string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	payload := BuildSigningPayload(parsed.Path, parsed.Query())
	sig := ComputeSignature(payload, secret)

	q := parsed.Query()
	q.Set("s", sig)
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

// VerifySignature validates request signature using constant-time comparison.
func VerifySignature(path string, query url.Values, secret string) bool {
	providedSig := query.Get("s")
	if providedSig == "" {
		return false
	}

	payload := BuildSigningPayload(path, query)
	expectedSig := ComputeSignature(payload, secret)

	providedBytes, err := hex.DecodeString(providedSig)
	if err != nil {
		return false
	}
	expectedBytes, err := hex.DecodeString(expectedSig)
	if err != nil {
		return false
	}

	return hmac.Equal(providedBytes, expectedBytes)
}

// HMACMiddleware enforces URL signature verification if enabled or if secret is configured.
func HMACMiddleware(secret string, enforce bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enforce && secret == "" {
				next.ServeHTTP(w, r)
				return
			}

			// If signature is required, or if client provided a signature parameter
			if enforce || r.URL.Query().Get("s") != "" {
				if !VerifySignature(r.URL.Path, r.URL.Query(), secret) {
					http.Error(w, "invalid or missing HMAC signature", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
