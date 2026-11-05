package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// DiskMeta contains metadata stored alongside cached image blocks.
type DiskMeta struct {
	Key         string    `json:"key"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	AccessedAt  time.Time `json:"accessed_at"`
}

// L2DiskCache manages content-addressable disk storage with two-level folder sharding.
type L2DiskCache struct {
	dir        string
	hits       uint64
	misses     uint64
	writes     uint64
	totalBytes int64
	fileCount  int64
}

// NewL2DiskCache creates the directory tree and boots disk cache.
func NewL2DiskCache(dir string) (*L2DiskCache, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create L2 disk cache directory: %w", err)
	}

	c := &L2DiskCache{
		dir: absDir,
	}

	// Calculate initial disk usage in background
	go c.calculateUsage()

	return c, nil
}

func (c *L2DiskCache) calculateUsage() {
	var bytes int64
	var count int64

	_ = filepath.Walk(c.dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			bytes += info.Size()
			count++
		}
		return nil
	})

	atomic.StoreInt64(&c.totalBytes, bytes)
	atomic.StoreInt64(&c.fileCount, count)
}

func (c *L2DiskCache) getPaths(key string) (dataPath string, metaPath string, dirPath string) {
	hash := sha256.Sum256([]byte(key))
	hStr := hex.EncodeToString(hash[:])

	// Two-level folder sharding: e.g. cache/a1/b2/a1b2c3d4...
	lvl1 := hStr[:2]
	lvl2 := hStr[2:4]

	dirPath = filepath.Join(c.dir, lvl1, lvl2)
	dataPath = filepath.Join(dirPath, hStr+".bin")
	metaPath = filepath.Join(dirPath, hStr+".meta")
	return dataPath, metaPath, dirPath
}

// Get reads data and metadata from disk if present.
func (c *L2DiskCache) Get(key string) ([]byte, string, bool) {
	dataPath, metaPath, _ := c.getPaths(key)

	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		atomic.AddUint64(&c.misses, 1)
		return nil, "", false
	}

	var meta DiskMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		atomic.AddUint64(&c.misses, 1)
		return nil, "", false
	}

	data, err := os.ReadFile(dataPath)
	if err != nil {
		atomic.AddUint64(&c.misses, 1)
		return nil, "", false
	}

	// Update access time asynchronously
	now := time.Now()
	_ = os.Chtimes(dataPath, now, now)

	atomic.AddUint64(&c.hits, 1)
	return data, meta.ContentType, true
}

// Set writes data and metadata atomically using a temporary file in the target directory.
func (c *L2DiskCache) Set(key string, data []byte, contentType string) error {
	dataPath, metaPath, dirPath := c.getPaths(key)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	// Atomic write for data
	tmpData := dataPath + fmt.Sprintf(".tmp.%d", time.Now().UnixNano())
	if err := os.WriteFile(tmpData, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpData, dataPath); err != nil {
		_ = os.Remove(tmpData)
		return err
	}

	// Atomic write for metadata
	meta := DiskMeta{
		Key:         key,
		ContentType: contentType,
		Size:        int64(len(data)),
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	tmpMeta := metaPath + fmt.Sprintf(".tmp.%d", time.Now().UnixNano())
	if err := os.WriteFile(tmpMeta, metaBytes, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpMeta, metaPath); err != nil {
		_ = os.Remove(tmpMeta)
		return err
	}

	atomic.AddUint64(&c.writes, 1)
	atomic.AddInt64(&c.totalBytes, int64(len(data)+len(metaBytes)))
	atomic.AddInt64(&c.fileCount, 2)
	return nil
}

// PurgePrefix removes cached blocks whose keys match prefix (or all if prefix is empty).
func (c *L2DiskCache) PurgePrefix(prefix string) (int, error) {
	removed := 0
	err := filepath.Walk(c.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".meta" {
			metaBytes, err := os.ReadFile(path)
			if err == nil {
				var meta DiskMeta
				if json.Unmarshal(metaBytes, &meta) == nil {
					if prefix == "" || len(meta.Key) >= len(prefix) && meta.Key[:len(prefix)] == prefix {
						_ = os.Remove(path)
						binFile := path[:len(path)-5] + ".bin"
						_ = os.Remove(binFile)
						removed++
					}
				}
			}
		}
		return nil
	})
	c.calculateUsage()
	return removed, err
}

// L2Stats holds telemetry for the disk cache.
type L2Stats struct {
	Hits       uint64  `json:"hits"`
	Misses     uint64  `json:"misses"`
	Writes     uint64  `json:"writes"`
	TotalBytes int64   `json:"total_bytes"`
	FileCount  int64   `json:"file_count"`
	HitRatio   float64 `json:"hit_ratio"`
}

// Stats returns snapshot telemetry for L2 disk cache.
func (c *L2DiskCache) Stats() L2Stats {
	hits := atomic.LoadUint64(&c.hits)
	misses := atomic.LoadUint64(&c.misses)
	total := hits + misses
	ratio := 0.0
	if total > 0 {
		ratio = float64(hits) / float64(total)
	}

	return L2Stats{
		Hits:       hits,
		Misses:     misses,
		Writes:     atomic.LoadUint64(&c.writes),
		TotalBytes: atomic.LoadInt64(&c.totalBytes),
		FileCount:  atomic.LoadInt64(&c.fileCount),
		HitRatio:   ratio,
	}
}
