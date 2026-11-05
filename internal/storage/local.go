package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage serves images from a local directory tree.
type LocalStorage struct {
	rootDir string
}

// NewLocalStorage initializes a LocalStorage provider anchored to rootDir.
func NewLocalStorage(rootDir string) (*LocalStorage, error) {
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, err
	}
	return &LocalStorage{rootDir: absPath}, nil
}

// Get opens the local file safely, preventing directory traversal.
func (s *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	// Normalize and prevent directory traversal
	cleanKey := filepath.Clean(strings.TrimPrefix(key, "/"))
	if strings.HasPrefix(cleanKey, "..") || filepath.IsAbs(cleanKey) {
		return nil, 0, ErrAccessDenied
	}

	fullPath := filepath.Join(s.rootDir, cleanKey)

	// Ensure the resolved target stays inside rootDir
	rel, err := filepath.Rel(s.rootDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, 0, ErrAccessDenied
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, err
	}

	if info.IsDir() {
		return nil, 0, ErrNotFound
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, 0, err
	}

	return file, info.Size(), nil
}

// Exists checks if a file exists within the rootDir.
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	cleanKey := filepath.Clean(strings.TrimPrefix(key, "/"))
	if strings.HasPrefix(cleanKey, "..") || filepath.IsAbs(cleanKey) {
		return false, ErrAccessDenied
	}

	fullPath := filepath.Join(s.rootDir, cleanKey)
	rel, err := filepath.Rel(s.rootDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false, ErrAccessDenied
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}
