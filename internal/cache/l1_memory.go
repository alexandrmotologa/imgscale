package cache

import (
	"container/list"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

const numShards = 16

// MemoryEntry represents a cached item in RAM.
type MemoryEntry struct {
	key         string
	data        []byte
	contentType string
	size        int64
	accessedAt  time.Time
}

type memoryShard struct {
	mu       sync.Mutex
	items    map[string]*list.Element
	lru      *list.List
	curBytes int64
	maxBytes int64
}

// L1MemoryCache is a concurrently sharded LRU memory cache.
type L1MemoryCache struct {
	shards    [numShards]*memoryShard
	hits      uint64
	misses    uint64
	evictions uint64
}

// NewL1MemoryCache initializes the sharded in-memory cache with a total byte budget.
func NewL1MemoryCache(maxBytes int64) *L1MemoryCache {
	if maxBytes <= 0 {
		maxBytes = 64 * 1024 * 1024 // 64MB default
	}

	shardMax := maxBytes / int64(numShards)
	if shardMax < 1024*1024 {
		shardMax = 1024 * 1024
	}

	c := &L1MemoryCache{}
	for i := 0; i < numShards; i++ {
		c.shards[i] = &memoryShard{
			items:    make(map[string]*list.Element),
			lru:      list.New(),
			maxBytes: shardMax,
		}
	}
	return c
}

func (c *L1MemoryCache) getShard(key string) *memoryShard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return c.shards[h.Sum32()%numShards]
}

// Get fetches data from the L1 cache.
func (c *L1MemoryCache) Get(key string) ([]byte, string, bool) {
	shard := c.getShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	if elem, found := shard.items[key]; found {
		shard.lru.MoveToFront(elem)
		entry := elem.Value.(*MemoryEntry)
		entry.accessedAt = time.Now()
		atomic.AddUint64(&c.hits, 1)
		return entry.data, entry.contentType, true
	}

	atomic.AddUint64(&c.misses, 1)
	return nil, "", false
}

// Set stores data in the L1 cache, evicting LRU items if over capacity.
func (c *L1MemoryCache) Set(key string, data []byte, contentType string) {
	entrySize := int64(len(data))
	shard := c.getShard(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// If entry alone exceeds shard capacity, do not store in L1 RAM
	if entrySize > shard.maxBytes {
		return
	}

	// Update existing item
	if elem, found := shard.items[key]; found {
		oldEntry := elem.Value.(*MemoryEntry)
		shard.curBytes -= oldEntry.size
		shard.lru.Remove(elem)
		delete(shard.items, key)
	}

	// Evict oldest items until under capacity
	for shard.curBytes+entrySize > shard.maxBytes && shard.lru.Len() > 0 {
		oldest := shard.lru.Back()
		if oldest == nil {
			break
		}
		oldEntry := oldest.Value.(*MemoryEntry)
		shard.curBytes -= oldEntry.size
		shard.lru.Remove(oldest)
		delete(shard.items, oldEntry.key)
		atomic.AddUint64(&c.evictions, 1)
	}

	entry := &MemoryEntry{
		key:         key,
		data:        data,
		contentType: contentType,
		size:        entrySize,
		accessedAt:  time.Now(),
	}

	elem := shard.lru.PushFront(entry)
	shard.items[key] = elem
	shard.curBytes += entrySize
}

// PurgePrefix removes all items whose keys start with prefix (or all if prefix is empty).
func (c *L1MemoryCache) PurgePrefix(prefix string) int {
	removedCount := 0
	for i := 0; i < numShards; i++ {
		shard := c.shards[i]
		shard.mu.Lock()
		for k, elem := range shard.items {
			if prefix == "" || len(k) >= len(prefix) && k[:len(prefix)] == prefix {
				entry := elem.Value.(*MemoryEntry)
				shard.curBytes -= entry.size
				shard.lru.Remove(elem)
				delete(shard.items, k)
				removedCount++
			}
		}
		shard.mu.Unlock()
	}
	return removedCount
}

// L1Stats holds telemetry for the memory cache.
type L1Stats struct {
	Hits       uint64 `json:"hits"`
	Misses     uint64 `json:"misses"`
	Evictions  uint64 `json:"evictions"`
	TotalBytes int64  `json:"total_bytes"`
	ItemCount  int    `json:"item_count"`
	HitRatio   float64`json:"hit_ratio"`
}

// Stats computes snapshot metrics across all memory shards.
func (c *L1MemoryCache) Stats() L1Stats {
	var totalBytes int64
	var itemCount int

	for i := 0; i < numShards; i++ {
		c.shards[i].mu.Lock()
		totalBytes += c.shards[i].curBytes
		itemCount += len(c.shards[i].items)
		c.shards[i].mu.Unlock()
	}

	hits := atomic.LoadUint64(&c.hits)
	misses := atomic.LoadUint64(&c.misses)
	total := hits + misses
	ratio := 0.0
	if total > 0 {
		ratio = float64(hits) / float64(total)
	}

	return L1Stats{
		Hits:       hits,
		Misses:     misses,
		Evictions:  atomic.LoadUint64(&c.evictions),
		TotalBytes: totalBytes,
		ItemCount:  itemCount,
		HitRatio:   ratio,
	}
}
