package cache

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type diskFileInfo struct {
	path    string
	size    int64
	modTime time.Time
}

// DiskSweeper manages periodic background quota enforcement and TTL eviction.
type DiskSweeper struct {
	diskCache *L2DiskCache
	maxBytes  int64
	maxTTL    time.Duration
	interval  time.Duration
}

// NewDiskSweeper initializes the background cleaner.
func NewDiskSweeper(diskCache *L2DiskCache, maxBytes int64, maxTTL time.Duration, interval time.Duration) *DiskSweeper {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if maxTTL <= 0 {
		maxTTL = 30 * 24 * time.Hour // 30 days default
	}

	return &DiskSweeper{
		diskCache: diskCache,
		maxBytes:  maxBytes,
		maxTTL:    maxTTL,
		interval:  interval,
	}
}

// Start runs the sweeper loop until context is canceled.
func (s *DiskSweeper) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Initial run
	s.Sweep()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Sweep()
		}
	}
}

// Sweep audits all cached files on disk, pruning expired files and oldest items if over quota.
func (s *DiskSweeper) Sweep() {
	var files []diskFileInfo
	var totalBytes int64
	now := time.Now()

	// 1. Walk directory and collect file info
	_ = filepath.Walk(s.diskCache.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// TTL Expiration Check
		if s.maxTTL > 0 && now.Sub(info.ModTime()) > s.maxTTL {
			_ = os.Remove(path)
			return nil
		}

		files = append(files, diskFileInfo{
			path:    path,
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		totalBytes += info.Size()
		return nil
	})

	// 2. If total size exceeds max quota, evict oldest files until under 80% capacity
	if s.maxBytes > 0 && totalBytes > s.maxBytes {
		targetBytes := int64(float64(s.maxBytes) * 0.80)

		// Sort by oldest access/mod time first
		sort.Slice(files, func(i, j int) bool {
			return files[i].modTime.Before(files[j].modTime)
		})

		for _, f := range files {
			if totalBytes <= targetBytes {
				break
			}
			if err := os.Remove(f.path); err == nil {
				totalBytes -= f.size
				// Also remove companion .meta or .bin file
				if strings.HasSuffix(f.path, ".bin") {
					companion := strings.TrimSuffix(f.path, ".bin") + ".meta"
					if cInfo, err := os.Stat(companion); err == nil {
						_ = os.Remove(companion)
						totalBytes -= cInfo.Size()
					}
				}
			}
		}
	}

	// Update cached usage count
	s.diskCache.calculateUsage()
}
