package cache

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"
)

// CacheSource identifies where the image was retrieved from.
type CacheSource string

const (
	SourceL1Memory      CacheSource = "L1_MEMORY"
	SourceL2Disk        CacheSource = "L2_DISK"
	SourceOriginCompute CacheSource = "ORIGIN_COMPUTE"
)

// TwoTierCache orchestrates L1 memory, L2 persistent disk, and singleflight request coalescing.
type TwoTierCache struct {
	l1      *L1MemoryCache
	l2      *L2DiskCache
	sweeper *DiskSweeper
	sfGroup singleflight.Group
}

// Config specifies cache sizes and expiration policies.
type Config struct {
	L1MaxBytes int64
	L2Dir      string
	L2MaxBytes int64
	L2MaxTTL   time.Duration
}

// NewTwoTierCache initializes the two-tier caching infrastructure.
func NewTwoTierCache(cfg Config) (*TwoTierCache, error) {
	l1 := NewL1MemoryCache(cfg.L1MaxBytes)
	l2, err := NewL2DiskCache(cfg.L2Dir)
	if err != nil {
		return nil, err
	}

	sweeper := NewDiskSweeper(l2, cfg.L2MaxBytes, cfg.L2MaxTTL, 5*time.Minute)

	return &TwoTierCache{
		l1:      l1,
		l2:      l2,
		sweeper: sweeper,
	}, nil
}

// StartBackgroundSweeper starts the periodic disk cleaner.
func (c *TwoTierCache) StartBackgroundSweeper(ctx context.Context) {
	go c.sweeper.Start(ctx)
}

type computeResult struct {
	data        []byte
	contentType string
	source      CacheSource
}

// GetOrCompute retrieves from L1, then L2, or executes computeFn safely with singleflight deduplication.
func (c *TwoTierCache) GetOrCompute(ctx context.Context, key string, computeFn func() ([]byte, string, error)) ([]byte, string, CacheSource, error) {
	// 1. Check L1 Memory Cache
	if data, ctype, found := c.l1.Get(key); found {
		return data, ctype, SourceL1Memory, nil
	}

	// 2. Check L2 Disk Cache
	if data, ctype, found := c.l2.Get(key); found {
		// Populate L1 cache for subsequent requests
		c.l1.Set(key, data, ctype)
		return data, ctype, SourceL2Disk, nil
	}

	// 3. Cache Miss: Deduplicate concurrent requests with singleflight
	res, err, _ := c.sfGroup.Do(key, func() (interface{}, error) {
		// Re-check caches inside singleflight in case a concurrent request just populated it
		if data, ctype, found := c.l1.Get(key); found {
			return computeResult{data: data, contentType: ctype, source: SourceL1Memory}, nil
		}
		if data, ctype, found := c.l2.Get(key); found {
			c.l1.Set(key, data, ctype)
			return computeResult{data: data, contentType: ctype, source: SourceL2Disk}, nil
		}

		// Perform image transformation
		data, ctype, err := computeFn()
		if err != nil {
			return nil, err
		}

		// Write to L2 disk cache
		_ = c.l2.Set(key, data, ctype)

		// Write to L1 memory cache
		c.l1.Set(key, data, ctype)

		return computeResult{
			data:        data,
			contentType: ctype,
			source:      SourceOriginCompute,
		}, nil
	})

	if err != nil {
		return nil, "", "", err
	}

	cr := res.(computeResult)
	return cr.data, cr.contentType, cr.source, nil
}

// CacheTelemetry holds aggregated statistics across both cache tiers.
type CacheTelemetry struct {
	L1 L1Stats `json:"l1"`
	L2 L2Stats `json:"l2"`
}

// Telemetry returns current operational metrics.
func (c *TwoTierCache) Telemetry() CacheTelemetry {
	return CacheTelemetry{
		L1: c.l1.Stats(),
		L2: c.l2.Stats(),
	}
}
