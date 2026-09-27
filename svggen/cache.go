// Package svggen provides the SVG generation cache implementation.
package svggen

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// Clock provides time-related functions that can be replaced in tests.
type Clock interface {
	Now() time.Time
}

// realClock uses the real system time.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// CacheEntry represents a cached render result with metadata.
type CacheEntry struct {
	Result    *RenderResult
	CreatedAt time.Time
	ExpiresAt time.Time
	HitCount  uint64
	key       string // Cache key for this entry (used by eviction list)
}

// isExpiredAt returns true if the entry has exceeded its TTL relative to the given time.
func (e *CacheEntry) isExpiredAt(now time.Time) bool {
	return now.After(e.ExpiresAt)
}

// IsExpired returns true if the entry has exceeded its TTL.
func (e *CacheEntry) IsExpired() bool {
	return e.isExpiredAt(time.Now())
}

// CacheConfig holds configuration for the render cache.
type CacheConfig struct {
	// TTL is the time-to-live for cache entries.
	// Default: 5 minutes.
	TTL time.Duration

	// MaxEntries is the maximum number of entries to keep.
	// When exceeded, oldest entries are evicted.
	// Default: 1000.
	MaxEntries int

	// MaxBytes caps the approximate total size (SVG + PNG + PDF bytes) of
	// cached results. Oldest entries are evicted until a new entry fits;
	// a single result larger than MaxBytes is not cached.
	// Default: 256 MiB.
	MaxBytes int64

	// CleanupInterval is how often to run background cleanup.
	// Default: 1 minute.
	CleanupInterval time.Duration
}

// DefaultCacheConfig returns the default cache configuration.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		TTL:             5 * time.Minute,
		MaxEntries:      1000,
		MaxBytes:        256 << 20,
		CleanupInterval: 1 * time.Minute,
	}
}

// resultBytes returns the approximate retained size of a render result.
func resultBytes(r *RenderResult) int64 {
	if r == nil {
		return 0
	}
	var n int64
	if r.SVG != nil {
		n += int64(len(r.SVG.Content))
	}
	return n + int64(len(r.PNG)) + int64(len(r.PDF))
}

// CacheStats provides statistics about cache performance.
type CacheStats struct {
	// Hits is the total number of cache hits.
	Hits uint64 `json:"hits"`

	// Misses is the total number of cache misses.
	Misses uint64 `json:"misses"`

	// Entries is the current number of entries in the cache.
	Entries int `json:"entries"`

	// Evictions is the total number of entries evicted.
	Evictions uint64 `json:"evictions"`

	// TotalBytes is the approximate total size of cached data.
	TotalBytes int64 `json:"total_bytes"`
}

// HitRate returns the cache hit rate as a percentage (0-100).
func (s CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// RenderCache is a thread-safe in-memory cache for render results.
type RenderCache struct {
	mu       sync.RWMutex
	entries  map[string]*CacheEntry
	config   CacheConfig
	clock    Clock
	stopChan chan struct{}
	stopped  bool

	// Eviction ordering: oldest entries at back of list, newest at front.
	// This provides O(1) eviction instead of O(n) linear scan.
	evictList  *list.List               // Doubly-linked list for insertion-order tracking
	evictIndex map[string]*list.Element // Map cache key -> list element for O(1) lookup
	totalBytes int64                    // Approximate retained bytes; guarded by mu

	// Atomic counters for lock-free stats tracking
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
}

// NewRenderCache creates a new render cache with the given configuration.
func NewRenderCache(cfg CacheConfig) *RenderCache {
	return NewRenderCacheWithClock(cfg, realClock{})
}

// NewRenderCacheWithClock creates a new render cache with a custom clock for testing.
func NewRenderCacheWithClock(cfg CacheConfig, clock Clock) *RenderCache {
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultCacheConfig().TTL
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = DefaultCacheConfig().MaxEntries
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultCacheConfig().CleanupInterval
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = DefaultCacheConfig().MaxBytes
	}

	cache := &RenderCache{
		entries:    make(map[string]*CacheEntry),
		config:     cfg,
		clock:      clock,
		stopChan:   make(chan struct{}),
		evictList:  list.New(),
		evictIndex: make(map[string]*list.Element),
	}

	// Start background cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a cached result by request key.
// Returns nil if not found or expired.
func (c *RenderCache) Get(req *RequestEnvelope) *RenderResult {
	return c.GetByKey(c.Key(req))
}

// Key returns the stable cache identity for a request at lookup time.
func (c *RenderCache) Key(req *RequestEnvelope) string {
	return c.computeKey(req)
}

// GetByKey retrieves a result using a previously captured request identity.
func (c *RenderCache) GetByKey(key string) *RenderResult {

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		c.misses.Add(1)
		return nil
	}

	if entry.isExpiredAt(c.clock.Now()) {
		c.mu.Lock()
		c.removeLocked(key)
		c.mu.Unlock()
		c.misses.Add(1)
		return nil
	}

	// Increment hit count atomically on the entry
	atomic.AddUint64(&entry.HitCount, 1)
	c.hits.Add(1)

	return entry.Result
}

// Set stores a render result in the cache.
func (c *RenderCache) Set(req *RequestEnvelope, result *RenderResult) {
	c.SetByKey(c.Key(req), result)
}

// SetByKey stores a result under the identity captured before rendering. This
// prevents renderer defaulting or normalization from changing the insertion key.
func (c *RenderCache) SetByKey(key string, result *RenderResult) {
	now := c.clock.Now()

	size := resultBytes(result)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Drop any existing entry for this key (update case); re-added at front.
	c.removeLocked(key)

	// A result larger than the whole byte budget is never cached.
	if size > c.config.MaxBytes {
		return
	}

	// Evict until both the entry and byte caps have room.
	for c.evictList.Len() > 0 && (len(c.entries) >= c.config.MaxEntries || c.totalBytes+size > c.config.MaxBytes) {
		c.evictOldestLocked()
	}

	entry := &CacheEntry{
		Result:    result,
		CreatedAt: now,
		ExpiresAt: now.Add(c.config.TTL),
		HitCount:  0,
		key:       key,
	}
	c.entries[key] = entry
	c.totalBytes += size

	// Add to front of eviction list (newest entries at front)
	elem := c.evictList.PushFront(entry)
	c.evictIndex[key] = elem
}

// Invalidate removes a specific entry from the cache.
func (c *RenderCache) Invalidate(req *RequestEnvelope) {
	key := c.computeKey(req)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.removeLocked(key)
}

// removeLocked deletes key from every cache structure and releases its bytes.
// Must be called with the write lock held.
func (c *RenderCache) removeLocked(key string) {
	if entry, ok := c.entries[key]; ok {
		c.totalBytes -= resultBytes(entry.Result)
		delete(c.entries, key)
	}
	if elem, ok := c.evictIndex[key]; ok {
		c.evictList.Remove(elem)
		delete(c.evictIndex, key)
	}
}

// Clear removes all entries from the cache.
func (c *RenderCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.evictList = list.New()
	c.evictIndex = make(map[string]*list.Element)
	c.totalBytes = 0
}

// Stats returns current cache statistics.
func (c *RenderCache) Stats() CacheStats {
	// Read atomic counters without holding lock
	stats := CacheStats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
	}

	// Only hold lock briefly to count entries and calculate bytes
	c.mu.RLock()
	stats.Entries = len(c.entries)

	// Calculate approximate total bytes
	var totalBytes int64
	for _, entry := range c.entries {
		if entry.Result.SVG != nil {
			totalBytes += int64(len(entry.Result.SVG.Content))
		}
		totalBytes += int64(len(entry.Result.PNG))
		totalBytes += int64(len(entry.Result.PDF))
	}
	stats.TotalBytes = totalBytes
	c.mu.RUnlock()

	return stats
}

// Stop gracefully stops the background cleanup goroutine.
func (c *RenderCache) Stop() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.stopped = true
	c.mu.Unlock()

	close(c.stopChan)
}

// computeKey hashes the canonical JSON representation of the complete request.
// encoding/json sorts map keys, and its length-delimited syntax preserves type
// and element boundaries that delimiter-based encodings cannot distinguish.
func (c *RenderCache) computeKey(req *RequestEnvelope) string {
	encoded, err := json.Marshal(req)
	if err != nil {
		encoded = []byte("unencodable-request")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// evictOldestLocked removes the oldest entry. Must be called with lock held.
// Uses O(1) list-based eviction instead of O(n) linear scan.
func (c *RenderCache) evictOldestLocked() {
	// Oldest entry is at back of list
	elem := c.evictList.Back()
	if elem == nil {
		return
	}

	entry, ok := elem.Value.(*CacheEntry)
	if !ok {
		return
	}

	// Remove from all data structures
	c.removeLocked(entry.key)
	c.evictions.Add(1)
}

// cleanupLoop runs periodic cleanup of expired entries.
func (c *RenderCache) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpired()
		case <-c.stopChan:
			return
		}
	}
}

// cleanupExpired removes all expired entries.
func (c *RenderCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			c.removeLocked(key)
			c.evictions.Add(1)
		}
	}
}
