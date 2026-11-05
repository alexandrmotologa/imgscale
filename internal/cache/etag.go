package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// GenerateETag computes a deterministic RFC 9111 ETag for an image and its transformations.
func GenerateETag(key string, transformParams string, srcLength int64) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%d", key, transformParams, srcLength)
	digest := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("%q", digest[:16])
}

// CheckNoneMatch evaluates whether an incoming If-None-Match header matches the current ETag.
func CheckNoneMatch(ifNoneMatchHeader, currentETag string) bool {
	if ifNoneMatchHeader == "" {
		return false
	}

	header := strings.TrimSpace(ifNoneMatchHeader)
	if header == "*" {
		return true
	}

	currentETag = strings.Trim(currentETag, `"W/`)

	for _, token := range strings.Split(header, ",") {
		cleanToken := strings.TrimSpace(token)
		cleanToken = strings.Trim(cleanToken, `"W/`)
		if cleanToken == currentETag {
			return true
		}
	}
	return false
}
