package storage

import (
	"context"
	"errors"
	"io"
)

var (
	ErrNotFound      = errors.New("image not found in storage")
	ErrAccessDenied  = errors.New("access denied / path traversal blocked")
	ErrSSRFBlocked   = errors.New("target host or IP address is blocked by SSRF policy")
	ErrPayloadTooLarge = errors.New("source image exceeds maximum allowed payload size")
)

// StorageSource defines the unified interface for image origin backends.
type StorageSource interface {
	// Get retrieves the source image reader and content length for a given path/key.
	Get(ctx context.Context, key string) (io.ReadCloser, int64, error)
	// Exists checks if an asset exists at key.
	Exists(ctx context.Context, key string) (bool, error)
}
