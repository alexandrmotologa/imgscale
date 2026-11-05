package cache

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestETag(t *testing.T) {
	etag1 := GenerateETag("avatar.png", "w=200&h=200", 12345)
	etag2 := GenerateETag("avatar.png", "w=200&h=200", 12345)
	etag3 := GenerateETag("avatar.png", "w=400&h=400", 12345)

	if etag1 != etag2 {
		t.Errorf("Expected deterministic ETag, got %s != %s", etag1, etag2)
	}
	if etag1 == etag3 {
		t.Errorf("Expected different ETags for different params")
	}

	if !CheckNoneMatch(etag1, etag1) {
		t.Error("Expected matching ETag")
	}
	if !CheckNoneMatch(`"other", `+etag1, etag1) {
		t.Error("Expected matching comma-separated ETag list")
	}
	if CheckNoneMatch(`"other"`, etag1) {
		t.Error("Expected no match")
	}
}

func TestL1MemoryLRU(t *testing.T) {
	// 100KB cache
	l1 := NewL1MemoryCache(100 * 1024)

	data := make([]byte, 10*1024) // 10KB
	l1.Set("item1", data, "image/jpeg")

	val, ctype, found := l1.Get("item1")
	if !found || len(val) != len(data) || ctype != "image/jpeg" {
		t.Fatalf("Failed to retrieve stored item")
	}

	// Stats
	stats := l1.Stats()
	if stats.Hits != 1 || stats.Misses != 0 {
		t.Errorf("Unexpected stats: %+v", stats)
	}

	// Miss
	_, _, found = l1.Get("nonexistent")
	if found {
		t.Error("Expected miss")
	}
	stats = l1.Stats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}

func TestTwoTierCacheAndSingleflight(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "imgscale-cache-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	tc, err := NewTwoTierCache(Config{
		L1MaxBytes: 10 * 1024 * 1024,
		L2Dir:      tempDir,
		L2MaxBytes: 50 * 1024 * 1024,
		L2MaxTTL:   24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewTwoTierCache failed: %v", err)
	}

	ctx := context.Background()
	var computeCounter int32

	computeFn := func() ([]byte, string, error) {
		time.Sleep(50 * time.Millisecond) // Simulate heavy resize
		atomic.AddInt32(&computeCounter, 1)
		return []byte("transformed-image-bytes"), "image/webp", nil
	}

	// 1. Concurrently request the same uncached image 10 times
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, ctype, source, err := tc.GetOrCompute(ctx, "test-hero.jpg?w=800", computeFn)
			if err != nil {
				t.Errorf("GetOrCompute error: %v", err)
				return
			}
			if string(data) != "transformed-image-bytes" || ctype != "image/webp" {
				t.Errorf("Payload mismatch: %s, %s", string(data), ctype)
			}
			if source != SourceOriginCompute && source != SourceL1Memory && source != SourceL2Disk {
				t.Errorf("Unexpected source: %s", source)
			}
		}()
	}
	wg.Wait()

	// Singleflight should have coalesced all 10 concurrent requests into 1 execution
	count := atomic.LoadInt32(&computeCounter)
	if count != 1 {
		t.Errorf("Expected singleflight to run computation exactly 1 time, but ran %d times", count)
	}

	// 2. Subsequent request should be served from L1 memory
	data, ctype, source, err := tc.GetOrCompute(ctx, "test-hero.jpg?w=800", computeFn)
	if err != nil {
		t.Fatal(err)
	}
	if source != SourceL1Memory {
		t.Errorf("Expected SourceL1Memory, got %s", source)
	}
	if string(data) != "transformed-image-bytes" || ctype != "image/webp" {
		t.Errorf("Payload mismatch on L1 hit")
	}

	// 3. Clear L1 memory, next request should hit L2 disk
	tc.l1 = NewL1MemoryCache(10 * 1024 * 1024)
	data, ctype, source, err = tc.GetOrCompute(ctx, "test-hero.jpg?w=800", computeFn)
	if err != nil {
		t.Fatal(err)
	}
	if source != SourceL2Disk {
		t.Errorf("Expected SourceL2Disk after L1 flush, got %s", source)
	}
	if string(data) != "transformed-image-bytes" || ctype != "image/webp" {
		t.Errorf("Payload mismatch on L2 hit")
	}

	// Telemetry verification
	telemetry := tc.Telemetry()
	if telemetry.L2.Hits == 0 {
		t.Errorf("Expected L2 hits > 0")
	}
}

func TestDiskSweeperQuota(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "imgscale-sweeper-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	l2, err := NewL2DiskCache(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	// Write 5 items of 10KB each (total ~50KB)
	for i := 0; i < 5; i++ {
		data := make([]byte, 10*1024)
		key := string(rune('a' + i))
		if err := l2.Set(key, data, "image/jpeg"); err != nil {
			t.Fatal(err)
		}
	}

	// Set sweeper max quota to 25KB, should evict oldest until <= 80% (20KB)
	sweeper := NewDiskSweeper(l2, 25*1024, 24*time.Hour, 10*time.Millisecond)
	sweeper.Sweep()

	stats := l2.Stats()
	if stats.TotalBytes > 25*1024 {
		t.Errorf("Expected sweeper to prune under 25KB, current bytes: %d", stats.TotalBytes)
	}
}

func TestCachePurge(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "imgscale-purge-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	tc, err := NewTwoTierCache(Config{
		L1MaxBytes: 10 * 1024 * 1024,
		L2Dir:      tempDir,
		L2MaxBytes: 50 * 1024 * 1024,
		L2MaxTTL:   24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, _, _, _ = tc.GetOrCompute(ctx, "photo1.jpg?w=100", func() ([]byte, string, error) {
		return []byte("data1"), "image/jpeg", nil
	})
	_, _, _, _ = tc.GetOrCompute(ctx, "photo1.jpg?w=200", func() ([]byte, string, error) {
		return []byte("data2"), "image/jpeg", nil
	})
	_, _, _, _ = tc.GetOrCompute(ctx, "other.png?w=300", func() ([]byte, string, error) {
		return []byte("data3"), "image/png", nil
	})

	// Purge all variants of photo1.jpg
	purged, err := tc.Purge("photo1.jpg")
	if err != nil {
		t.Fatalf("Purge failed: %v", err)
	}
	if purged < 2 {
		t.Errorf("Expected at least 2 purged items, got %d", purged)
	}

	// Verify photo1 is gone from L1
	if _, _, found := tc.l1.Get("photo1.jpg?w=100"); found {
		t.Error("photo1.jpg?w=100 should be purged from L1")
	}
	// other.png should remain
	if _, _, found := tc.l1.Get("other.png?w=300"); !found {
		t.Error("other.png?w=300 should still exist in L1")
	}
}
