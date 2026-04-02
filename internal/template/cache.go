package template

import (
	"container/list"
	"os"
	"sync"
	"time"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// FileStatResult contains the result of a file stat operation.
type FileStatResult struct {
	ModTime time.Time
	Exists  bool
}

// FileStat abstracts file system operations for cache validation.
// This interface enables testing without real file system access
// and allows for alternative storage backends.
type FileStat interface {
	// Stat returns file information for the given path.
	// If the file does not exist, it returns FileStatResult{Exists: false} with nil error.
	// Errors are reserved for unexpected I/O failures.
	Stat(path string) (FileStatResult, error)
}

// OSFileStat is the default FileStat implementation using the operating system.
type OSFileStat struct{}

// Stat implements FileStat using os.Stat.
func (OSFileStat) Stat(path string) (FileStatResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileStatResult{Exists: false}, nil
		}
		return FileStatResult{}, err
	}
	return FileStatResult{ModTime: info.ModTime(), Exists: true}, nil
}

// cacheEntry stores a template analysis result with metadata for expiration and invalidation.
type cacheEntry struct {
	analysis  *types.TemplateAnalysis
	hash      string    // File hash at time of caching
	modTime   time.Time // File modification time at time of caching (for fast invalidation)
	expiresAt time.Time // When this entry should be evicted
}

// lruEntry stores a path in the LRU list for eviction ordering.
type lruEntry struct {
	path string
}

// MemoryCache implements an in-memory cache for template analysis results.
// It is thread-safe and supports hash-based invalidation, TTL expiration, and LRU eviction.
type MemoryCache struct {
	mu       sync.RWMutex
	entries  map[string]*cacheEntry
	ttl      time.Duration
	maxSize  int                      // Maximum number of entries (0 = unlimited)
	lruList  *list.List               // Doubly-linked list for LRU ordering (front = most recent)
	lruIndex map[string]*list.Element // Map path -> list element for O(1) access updates
	fileStat FileStat                 // File system abstraction for cache validation
	clock    Clock                    // Clock for time operations (testable)
}

// CacheOption configures a MemoryCache.
type CacheOption func(*MemoryCache)

// WithCacheClock sets the clock used by the cache.
// This is useful for testing to control time progression.
func WithCacheClock(clock Clock) CacheOption {
	return func(c *MemoryCache) {
		c.clock = clock
	}
}

// NewMemoryCache creates a new in-memory template cache with the specified TTL.
// A TTL of 0 means entries never expire (not recommended for production).
// This creates an unbounded cache; use NewMemoryCacheWithMaxSize for bounded caches.
func NewMemoryCache(ttl time.Duration, opts ...CacheOption) *MemoryCache {
	return NewMemoryCacheWithMaxSize(ttl, 0, opts...)
}

// NewMemoryCacheWithMaxSize creates a new in-memory template cache with the specified TTL
// and maximum number of entries. When the cache reaches maxSize, the least recently used
// entry is evicted to make room for new entries.
// A maxSize of 0 means unlimited entries (not recommended for production).
// A TTL of 0 defaults to 24 hours.
func NewMemoryCacheWithMaxSize(ttl time.Duration, maxSize int, opts ...CacheOption) *MemoryCache {
	return NewMemoryCacheWithFileStat(ttl, maxSize, OSFileStat{}, opts...)
}

// NewMemoryCacheWithFileStat creates a new in-memory template cache with a custom FileStat
// implementation. This allows injecting mock file system access for testing or using
// alternative storage backends.
func NewMemoryCacheWithFileStat(ttl time.Duration, maxSize int, fs FileStat, opts ...CacheOption) *MemoryCache {
	if ttl == 0 {
		ttl = 24 * time.Hour // Default to 24 hours as per spec
	}
	cache := &MemoryCache{
		entries:  make(map[string]*cacheEntry),
		ttl:      ttl,
		maxSize:  maxSize,
		lruList:  list.New(),
		lruIndex: make(map[string]*list.Element),
		fileStat: fs,
		clock:    RealClock{},
	}

	// Apply options
	for _, opt := range opts {
		opt(cache)
	}

	return cache
}

// Get retrieves a cached template analysis by file path.
// Returns (nil, false) if:
// - The entry doesn't exist
// - The entry has expired
// - The file hash has changed (cache invalidation)
// On successful retrieval, the entry is moved to the front of the LRU list.
//
// Optimization: Uses RLock for initial lookup, then promotes to Lock only
// when modification is needed (LRU touch or expiration deletion). Skips LRU
// update entirely if entry is already at front of the list (most recently used).
func (c *MemoryCache) Get(path string) (*types.TemplateAnalysis, bool) {
	// Fast path: read-only check with RLock
	c.mu.RLock()
	entry, exists := c.entries[path]
	if !exists {
		c.mu.RUnlock()
		return nil, false
	}

	// Check expiration while holding RLock
	now := c.clock.Now()
	expired := now.After(entry.expiresAt)
	analysis := entry.analysis // Copy reference while holding lock

	// Check if entry is already at front of LRU (can skip write lock)
	elem := c.lruIndex[path]
	atFront := c.lruList != nil && c.lruList.Front() == elem
	c.mu.RUnlock()

	if expired {
		// Slow path: need write lock to delete expired entry
		c.mu.Lock()
		// Re-check entry still exists and is still expired (another goroutine may have modified it)
		entry, exists = c.entries[path]
		if exists && c.clock.Now().After(entry.expiresAt) {
			c.removeFromLRU(path)
			delete(c.entries, path)
		}
		c.mu.Unlock()
		return nil, false
	}

	// Fast path: skip LRU update if already at front
	if atFront {
		return analysis, true
	}

	// Slow path: need write lock to update LRU
	c.mu.Lock()
	// Re-check entry still exists (another goroutine may have deleted it)
	if _, exists = c.entries[path]; exists {
		c.touchLRUIfNotFront(path)
	}
	c.mu.Unlock()

	return analysis, true
}

// Set stores a template analysis result in the cache.
// The entry will expire after the cache's TTL duration.
// If the cache is bounded and at capacity, the least recently used entry is evicted.
// For fast cache validation, use SetWithModTime to also store the file modification time.
func (c *MemoryCache) Set(path string, analysis *types.TemplateAnalysis) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.setLocked(path, &cacheEntry{
		analysis:  analysis,
		hash:      analysis.Hash,
		expiresAt: c.clock.Now().Add(c.ttl),
	})
}

// setLocked is the internal set implementation that assumes the lock is held.
// It handles LRU updates and eviction as needed.
func (c *MemoryCache) setLocked(path string, entry *cacheEntry) {
	// Check if entry already exists (update case)
	if _, exists := c.entries[path]; exists {
		// Update existing entry and move to front of LRU
		c.entries[path] = entry
		c.touchLRU(path)
		return
	}

	// New entry - check if we need to evict
	if c.maxSize > 0 && len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	// Add new entry
	c.entries[path] = entry
	c.addToLRU(path)
}

// SetWithModTime stores a template analysis result in the cache along with file modification time.
// This enables fast cache validation using modTime check before falling back to hash comparison.
// If the cache is bounded and at capacity, the least recently used entry is evicted.
func (c *MemoryCache) SetWithModTime(path string, analysis *types.TemplateAnalysis, modTime time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.setLocked(path, &cacheEntry{
		analysis:  analysis,
		hash:      analysis.Hash,
		modTime:   modTime,
		expiresAt: c.clock.Now().Add(c.ttl),
	})
}

// Invalidate removes an entry from the cache.
// This is useful when a file is known to have changed.
func (c *MemoryCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.removeFromLRU(path)
	delete(c.entries, path)
}

// IsValid checks if a cached entry is still valid by comparing file hashes.
// Returns true if the cached hash matches the provided hash.
func (c *MemoryCache) IsValid(path string, currentHash string) bool {
	c.mu.RLock()
	entry, exists := c.entries[path]
	c.mu.RUnlock()

	if !exists {
		return false
	}

	// Check expiration
	if c.clock.Now().After(entry.expiresAt) {
		return false
	}

	// Check hash match
	return entry.hash == currentHash
}

// CacheValidationResult indicates the result of a fast cache validation check.
type CacheValidationResult int

const (
	// CacheValid indicates the cached entry is valid (modTime unchanged).
	CacheValid CacheValidationResult = iota
	// CacheInvalid indicates the cache entry should be refreshed (modTime changed or file missing).
	CacheInvalid
	// CacheMiss indicates no cache entry exists or it has expired.
	CacheMiss
)

// IsValidFast performs a fast cache validity check using file modification time.
// This avoids the expensive hash calculation on every request.
//
// Returns:
//   - CacheValid: modTime unchanged, cache is valid, no hash recalculation needed
//   - CacheInvalid: modTime changed, caller should recalculate hash to confirm
//   - CacheMiss: no cache entry exists or entry has expired
//
// If CacheInvalid is returned and the caller confirms the content actually changed
// (via hash comparison), they should call Invalidate() before setting new data.
func (c *MemoryCache) IsValidFast(path string) (CacheValidationResult, error) {
	c.mu.RLock()
	entry, exists := c.entries[path]
	c.mu.RUnlock()

	if !exists {
		return CacheMiss, nil
	}

	// Check TTL expiration
	if c.clock.Now().After(entry.expiresAt) {
		return CacheMiss, nil
	}

	// If no modTime was stored (entry was set via Set, not SetWithModTime),
	// we can't do fast validation, so return CacheMiss to trigger re-analysis
	if entry.modTime.IsZero() {
		return CacheMiss, nil
	}

	// Fast check: file modification time
	result, err := c.fileStat.Stat(path)
	if err != nil {
		return CacheMiss, err
	}

	if !result.Exists {
		// File was deleted, invalidate cache
		c.Invalidate(path)
		return CacheInvalid, nil
	}

	// Compare modification times
	if result.ModTime.Equal(entry.modTime) {
		return CacheValid, nil // No hash calculation needed!
	}

	// ModTime changed - file may have been modified
	return CacheInvalid, nil
}

// GetWithFastValidation retrieves a cached entry only if it passes fast modTime validation.
// This is the preferred method for production use as it avoids hash calculation.
//
// Returns:
//   - (analysis, true): Cache hit with valid modTime
//   - (nil, false): Cache miss, modTime changed, or entry expired
func (c *MemoryCache) GetWithFastValidation(path string) (*types.TemplateAnalysis, bool) {
	result, err := c.IsValidFast(path)
	if err != nil || result != CacheValid {
		return nil, false
	}

	// Entry is valid, return it
	return c.Get(path)
}

// Clear removes all entries from the cache.
// This is primarily useful for testing.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
	c.lruList = list.New()
	c.lruIndex = make(map[string]*list.Element)
}

// Size returns the number of entries currently in the cache.
// This includes expired entries that haven't been evicted yet.
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// MaxSize returns the maximum number of entries this cache can hold.
// A value of 0 means unlimited.
func (c *MemoryCache) MaxSize() int {
	return c.maxSize
}

// --- LRU helper methods (must be called with lock held) ---

// addToLRU adds a path to the front of the LRU list.
func (c *MemoryCache) addToLRU(path string) {
	if c.lruList == nil {
		return
	}
	elem := c.lruList.PushFront(&lruEntry{path: path})
	c.lruIndex[path] = elem
}

// touchLRU moves a path to the front of the LRU list (most recently used).
func (c *MemoryCache) touchLRU(path string) {
	if c.lruList == nil {
		return
	}
	if elem, exists := c.lruIndex[path]; exists {
		c.lruList.MoveToFront(elem)
	}
}

// touchLRUIfNotFront moves a path to the front only if it's not already there.
// This optimization avoids unnecessary list operations for frequently accessed entries.
func (c *MemoryCache) touchLRUIfNotFront(path string) {
	if c.lruList == nil {
		return
	}
	elem, exists := c.lruIndex[path]
	if !exists {
		return
	}
	// Skip if already at front
	if c.lruList.Front() == elem {
		return
	}
	c.lruList.MoveToFront(elem)
}

// removeFromLRU removes a path from the LRU list.
func (c *MemoryCache) removeFromLRU(path string) {
	if c.lruList == nil {
		return
	}
	if elem, exists := c.lruIndex[path]; exists {
		c.lruList.Remove(elem)
		delete(c.lruIndex, path)
	}
}

// evictLRU removes the least recently used entry from the cache.
func (c *MemoryCache) evictLRU() {
	if c.lruList == nil || c.lruList.Len() == 0 {
		return
	}

	// Back of the list is least recently used
	oldest := c.lruList.Back()
	if oldest == nil {
		return
	}

	entry, ok := oldest.Value.(*lruEntry)
	if !ok {
		return
	}
	path := entry.path

	// Remove from all data structures
	c.lruList.Remove(oldest)
	delete(c.lruIndex, path)
	delete(c.entries, path)
}
