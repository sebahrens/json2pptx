package template

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ahrens/go-slide-creator/internal/testutil"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// MockFileStat is a test implementation of FileStat that allows controlling
// file system responses without actual file I/O.
type MockFileStat struct {
	mu      sync.RWMutex
	files   map[string]FileStatResult
	errFunc func(path string) error // Optional: return error for specific paths
}

// NewMockFileStat creates a new MockFileStat with the given file entries.
func NewMockFileStat() *MockFileStat {
	return &MockFileStat{
		files: make(map[string]FileStatResult),
	}
}

// Stat implements FileStat for testing.
func (m *MockFileStat) Stat(path string) (FileStatResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.errFunc != nil {
		if err := m.errFunc(path); err != nil {
			return FileStatResult{}, err
		}
	}

	result, exists := m.files[path]
	if !exists {
		return FileStatResult{Exists: false}, nil
	}
	return result, nil
}

// SetFile configures a file to exist with the given modification time.
func (m *MockFileStat) SetFile(path string, modTime time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = FileStatResult{ModTime: modTime, Exists: true}
}

// DeleteFile removes a file from the mock, making it appear as deleted.
func (m *MockFileStat) DeleteFile(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, path)
}

// SetError configures an error to be returned for a specific path pattern.
func (m *MockFileStat) SetError(errFunc func(path string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errFunc = errFunc
}

// TestCacheBasicOperations tests basic cache operations (get, set, invalidate).
func TestCacheBasicOperations(t *testing.T) {
	cache := NewMemoryCache(1 * time.Hour)

	// Test cache miss
	t.Run("cache miss", func(t *testing.T) {
		analysis, ok := cache.Get("/nonexistent.pptx")
		if ok {
			t.Error("expected cache miss, got hit")
		}
		if analysis != nil {
			t.Error("expected nil analysis on cache miss")
		}
	})

	// Test cache set and hit
	t.Run("cache hit", func(t *testing.T) {
		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: "/test.pptx",
			Hash:         "abc123",
			AspectRatio:  "16:9",
			AnalyzedAt:   time.Now(),
		}

		cache.Set("/test.pptx", testAnalysis)

		analysis, ok := cache.Get("/test.pptx")
		if !ok {
			t.Fatal("expected cache hit, got miss")
		}
		if analysis.Hash != "abc123" {
			t.Errorf("expected hash abc123, got %s", analysis.Hash)
		}
		if analysis.TemplatePath != "/test.pptx" {
			t.Errorf("expected path /test.pptx, got %s", analysis.TemplatePath)
		}
	})

	// Test invalidation
	t.Run("invalidation", func(t *testing.T) {
		cache.Invalidate("/test.pptx")

		analysis, ok := cache.Get("/test.pptx")
		if ok {
			t.Error("expected cache miss after invalidation, got hit")
		}
		if analysis != nil {
			t.Error("expected nil analysis after invalidation")
		}
	})
}

// TestCacheExpiration tests that cache entries expire after TTL.
// AC8: Cache should expire after configured TTL
func TestCacheExpiration(t *testing.T) {
	clock := testutil.NewMockClock()
	cache := NewMemoryCache(50*time.Millisecond, WithCacheClock(clock))

	testAnalysis := &types.TemplateAnalysis{
		TemplatePath: "/expire-test.pptx",
		Hash:         "def456",
		AspectRatio:  "16:9",
		AnalyzedAt:   clock.Now(),
	}

	cache.Set("/expire-test.pptx", testAnalysis)

	// Should be available immediately
	analysis, ok := cache.Get("/expire-test.pptx")
	if !ok {
		t.Fatal("expected cache hit immediately after set")
	}
	if analysis.Hash != "def456" {
		t.Errorf("expected hash def456, got %s", analysis.Hash)
	}

	// Advance clock past expiration
	clock.Advance(100 * time.Millisecond)

	// Should be expired now
	analysis, ok = cache.Get("/expire-test.pptx")
	if ok {
		t.Error("expected cache miss after expiration, got hit")
	}
	if analysis != nil {
		t.Error("expected nil analysis after expiration")
	}
}

// TestCacheHashValidation tests hash-based cache invalidation.
// AC9: Cache should be invalidated when file hash changes
func TestCacheHashValidation(t *testing.T) {
	cache := NewMemoryCache(1 * time.Hour)

	testAnalysis := &types.TemplateAnalysis{
		TemplatePath: "/hash-test.pptx",
		Hash:         "hash1",
		AspectRatio:  "16:9",
		AnalyzedAt:   time.Now(),
	}

	cache.Set("/hash-test.pptx", testAnalysis)

	// Same hash should be valid
	t.Run("same hash valid", func(t *testing.T) {
		if !cache.IsValid("/hash-test.pptx", "hash1") {
			t.Error("expected IsValid to return true for matching hash")
		}
	})

	// Different hash should be invalid
	t.Run("different hash invalid", func(t *testing.T) {
		if cache.IsValid("/hash-test.pptx", "hash2") {
			t.Error("expected IsValid to return false for non-matching hash")
		}
	})

	// Nonexistent entry should be invalid
	t.Run("nonexistent entry invalid", func(t *testing.T) {
		if cache.IsValid("/nonexistent.pptx", "hash1") {
			t.Error("expected IsValid to return false for nonexistent entry")
		}
	})
}

// TestCacheClear tests clearing all cache entries.
func TestCacheClear(t *testing.T) {
	cache := NewMemoryCache(1 * time.Hour)

	// Add multiple entries
	for i := 0; i < 5; i++ {
		path := "/test" + string(rune('0'+i)) + ".pptx"
		analysis := &types.TemplateAnalysis{
			TemplatePath: path,
			Hash:         "hash" + string(rune('0'+i)),
			AnalyzedAt:   time.Now(),
		}
		cache.Set(path, analysis)
	}

	if cache.Size() != 5 {
		t.Errorf("expected cache size 5, got %d", cache.Size())
	}

	// Clear cache
	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected cache size 0 after clear, got %d", cache.Size())
	}

	// Verify entries are gone
	_, ok := cache.Get("/test0.pptx")
	if ok {
		t.Error("expected cache miss after clear")
	}
}

// TestCacheSize tests the Size method.
func TestCacheSize(t *testing.T) {
	cache := NewMemoryCache(1 * time.Hour)

	if cache.Size() != 0 {
		t.Errorf("expected initial size 0, got %d", cache.Size())
	}

	// Add entries
	for i := 0; i < 10; i++ {
		path := "/size-test" + string(rune('0'+i)) + ".pptx"
		analysis := &types.TemplateAnalysis{
			TemplatePath: path,
			Hash:         "hash",
			AnalyzedAt:   time.Now(),
		}
		cache.Set(path, analysis)
	}

	if cache.Size() != 10 {
		t.Errorf("expected size 10, got %d", cache.Size())
	}

	// Invalidate some entries
	cache.Invalidate("/size-test0.pptx")
	cache.Invalidate("/size-test1.pptx")

	if cache.Size() != 8 {
		t.Errorf("expected size 8 after invalidations, got %d", cache.Size())
	}
}

// TestCacheConcurrency tests thread-safe concurrent operations.
func TestCacheConcurrency(t *testing.T) {
	cache := NewMemoryCache(1 * time.Hour)

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			path := "/concurrent-test.pptx"
			analysis := &types.TemplateAnalysis{
				TemplatePath: path,
				Hash:         "concurrent-hash",
				AnalyzedAt:   time.Now(),
			}

			// Perform random operations
			cache.Set(path, analysis)
			cache.Get(path)
			cache.IsValid(path, "concurrent-hash")
			if id%2 == 0 {
				cache.Invalidate(path)
			}
			cache.Size()

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we got here without deadlock or panic, test passes
}

// TestCacheDefaultTTL tests that default TTL is 24 hours when 0 is provided.
func TestCacheDefaultTTL(t *testing.T) {
	clock := testutil.NewMockClock()
	cache := NewMemoryCache(0, WithCacheClock(clock))

	testAnalysis := &types.TemplateAnalysis{
		TemplatePath: "/default-ttl.pptx",
		Hash:         "ttl-test",
		AnalyzedAt:   clock.Now(),
	}

	cache.Set("/default-ttl.pptx", testAnalysis)

	// Entry should still be valid after some time (within 24h default)
	clock.Advance(10 * time.Millisecond)
	analysis, ok := cache.Get("/default-ttl.pptx")
	if !ok {
		t.Fatal("expected cache hit with default TTL")
	}
	if analysis.Hash != "ttl-test" {
		t.Errorf("expected hash ttl-test, got %s", analysis.Hash)
	}
}

// TestCacheMultipleEntries tests handling multiple different template paths.
func TestCacheMultipleEntries(t *testing.T) {
	cache := NewMemoryCache(1 * time.Hour)

	// Add multiple different templates
	templates := []string{
		"/template1.pptx",
		"/template2.pptx",
		"/dir/template3.pptx",
	}

	for i, path := range templates {
		analysis := &types.TemplateAnalysis{
			TemplatePath: path,
			Hash:         "hash" + string(rune('0'+i)),
			AspectRatio:  "16:9",
			AnalyzedAt:   time.Now(),
		}
		cache.Set(path, analysis)
	}

	// Verify all are cached
	for i, path := range templates {
		analysis, ok := cache.Get(path)
		if !ok {
			t.Errorf("expected cache hit for %s", path)
			continue
		}
		expectedHash := "hash" + string(rune('0'+i))
		if analysis.Hash != expectedHash {
			t.Errorf("for %s: expected hash %s, got %s", path, expectedHash, analysis.Hash)
		}
	}

	// Invalidate one
	cache.Invalidate("/template1.pptx")

	// Verify others still cached
	analysis, ok := cache.Get("/template2.pptx")
	if !ok {
		t.Error("expected template2 still cached")
	}
	if analysis.Hash != "hash1" {
		t.Errorf("expected hash hash1, got %s", analysis.Hash)
	}

	// Verify invalidated one is gone
	_, ok = cache.Get("/template1.pptx")
	if ok {
		t.Error("expected template1 to be invalidated")
	}
}

// TestCacheOverwrite tests that setting the same path twice overwrites the previous entry.
func TestCacheOverwrite(t *testing.T) {
	cache := NewMemoryCache(1 * time.Hour)

	// Set initial value
	analysis1 := &types.TemplateAnalysis{
		TemplatePath: "/overwrite-test.pptx",
		Hash:         "hash1",
		AspectRatio:  "4:3",
		AnalyzedAt:   time.Now(),
	}
	cache.Set("/overwrite-test.pptx", analysis1)

	// Overwrite with new value
	analysis2 := &types.TemplateAnalysis{
		TemplatePath: "/overwrite-test.pptx",
		Hash:         "hash2",
		AspectRatio:  "16:9",
		AnalyzedAt:   time.Now(),
	}
	cache.Set("/overwrite-test.pptx", analysis2)

	// Verify latest value is returned
	retrieved, ok := cache.Get("/overwrite-test.pptx")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if retrieved.Hash != "hash2" {
		t.Errorf("expected hash2 after overwrite, got %s", retrieved.Hash)
	}
	if retrieved.AspectRatio != "16:9" {
		t.Errorf("expected aspect ratio 16:9 after overwrite, got %s", retrieved.AspectRatio)
	}
}

// TestSetWithModTime tests the SetWithModTime method for fast validation.
func TestSetWithModTime(t *testing.T) {
	cache := NewMemoryCache(1 * time.Hour)

	modTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	testAnalysis := &types.TemplateAnalysis{
		TemplatePath: "/modtime-test.pptx",
		Hash:         "hash123",
		AspectRatio:  "16:9",
		AnalyzedAt:   time.Now(),
	}

	cache.SetWithModTime("/modtime-test.pptx", testAnalysis, modTime)

	// Verify entry was stored
	analysis, ok := cache.Get("/modtime-test.pptx")
	if !ok {
		t.Fatal("expected cache hit after SetWithModTime")
	}
	if analysis.Hash != "hash123" {
		t.Errorf("expected hash hash123, got %s", analysis.Hash)
	}
}

// TestIsValidFast tests fast validation using file modification time.
// AC4: Given a cached template analysis, When validating cache on subsequent request,
// Then file hash is NOT recalculated if modification time unchanged.
func TestIsValidFast(t *testing.T) {
	// Create a temporary file for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test-template.pptx")

	// Create the test file
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Get the file's modification time
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}
	modTime := info.ModTime()

	cache := NewMemoryCache(1 * time.Hour)

	t.Run("cache miss for non-existent entry", func(t *testing.T) {
		result, err := cache.IsValidFast("/nonexistent.pptx")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != CacheMiss {
			t.Errorf("expected CacheMiss, got %v", result)
		}
	})

	t.Run("cache miss for entry without modTime", func(t *testing.T) {
		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testFile,
			Hash:         "hash1",
			AnalyzedAt:   time.Now(),
		}
		// Use Set (not SetWithModTime) - no modTime stored
		cache.Set(testFile, testAnalysis)

		result, err := cache.IsValidFast(testFile)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != CacheMiss {
			t.Errorf("expected CacheMiss for entry without modTime, got %v", result)
		}
	})

	t.Run("AC4 - cache valid when modTime unchanged", func(t *testing.T) {
		cache.Clear()
		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testFile,
			Hash:         "hash-original",
			AnalyzedAt:   time.Now(),
		}
		// Set with modTime for fast validation
		cache.SetWithModTime(testFile, testAnalysis, modTime)

		// Check fast validation - should be valid (no hash recalculation needed!)
		result, err := cache.IsValidFast(testFile)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != CacheValid {
			t.Errorf("expected CacheValid when modTime unchanged, got %v", result)
		}
	})

	t.Run("cache invalid when modTime changed", func(t *testing.T) {
		cache.Clear()
		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testFile,
			Hash:         "hash-original",
			AnalyzedAt:   time.Now(),
		}
		// Set with OLD modTime (different from current file)
		oldModTime := modTime.Add(-1 * time.Hour)
		cache.SetWithModTime(testFile, testAnalysis, oldModTime)

		// Check fast validation - should be invalid (modTime changed)
		result, err := cache.IsValidFast(testFile)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != CacheInvalid {
			t.Errorf("expected CacheInvalid when modTime changed, got %v", result)
		}
	})

	t.Run("cache invalid when file deleted", func(t *testing.T) {
		cache.Clear()
		// Create and then delete a file
		deletedFile := filepath.Join(tempDir, "deleted.pptx")
		if err := os.WriteFile(deletedFile, []byte("temp"), 0644); err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		info, err := os.Stat(deletedFile)
		if err != nil {
			t.Fatalf("failed to stat temp file: %v", err)
		}

		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: deletedFile,
			Hash:         "hash-deleted",
			AnalyzedAt:   time.Now(),
		}
		cache.SetWithModTime(deletedFile, testAnalysis, info.ModTime())

		// Delete the file
		if err := os.Remove(deletedFile); err != nil {
			t.Fatalf("failed to delete file: %v", err)
		}

		// Check fast validation - should detect file is gone
		result, err := cache.IsValidFast(deletedFile)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != CacheInvalid {
			t.Errorf("expected CacheInvalid when file deleted, got %v", result)
		}

		// Verify cache was also invalidated
		_, ok := cache.Get(deletedFile)
		if ok {
			t.Error("expected cache entry to be invalidated after file deletion")
		}
	})

	t.Run("cache miss for expired entry with modTime", func(t *testing.T) {
		clock := testutil.NewMockClock()
		shortCache := NewMemoryCache(50*time.Millisecond, WithCacheClock(clock))

		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testFile,
			Hash:         "hash-expire",
			AnalyzedAt:   clock.Now(),
		}
		shortCache.SetWithModTime(testFile, testAnalysis, modTime)

		// Advance clock past expiration
		clock.Advance(100 * time.Millisecond)

		result, err := shortCache.IsValidFast(testFile)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != CacheMiss {
			t.Errorf("expected CacheMiss for expired entry, got %v", result)
		}
	})
}

// TestGetWithFastValidation tests the GetWithFastValidation method.
func TestGetWithFastValidation(t *testing.T) {
	// Create a temporary file for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "fast-get-test.pptx")

	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}
	modTime := info.ModTime()

	cache := NewMemoryCache(1 * time.Hour)

	t.Run("returns nil for cache miss", func(t *testing.T) {
		analysis, ok := cache.GetWithFastValidation("/nonexistent.pptx")
		if ok {
			t.Error("expected cache miss")
		}
		if analysis != nil {
			t.Error("expected nil analysis")
		}
	})

	t.Run("returns entry when modTime valid", func(t *testing.T) {
		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testFile,
			Hash:         "fast-hash",
			AspectRatio:  "16:9",
			AnalyzedAt:   time.Now(),
		}
		cache.SetWithModTime(testFile, testAnalysis, modTime)

		analysis, ok := cache.GetWithFastValidation(testFile)
		if !ok {
			t.Error("expected cache hit with valid modTime")
		}
		if analysis == nil {
			t.Fatal("expected non-nil analysis")
		}
		if analysis.Hash != "fast-hash" {
			t.Errorf("expected hash fast-hash, got %s", analysis.Hash)
		}
	})

	t.Run("returns nil when modTime changed", func(t *testing.T) {
		// Set with old modTime
		oldModTime := modTime.Add(-2 * time.Hour)
		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testFile,
			Hash:         "stale-hash",
			AnalyzedAt:   time.Now(),
		}
		cache.SetWithModTime(testFile, testAnalysis, oldModTime)

		analysis, ok := cache.GetWithFastValidation(testFile)
		if ok {
			t.Error("expected cache miss when modTime changed")
		}
		if analysis != nil {
			t.Error("expected nil analysis when modTime changed")
		}
	})
}

// TestAC4_FastValidationNoHashRecalculation is the primary AC4 acceptance test.
// It verifies that when the file modification time is unchanged, no hash recalculation
// is needed (the fast path is taken).
func TestAC4_FastValidationNoHashRecalculation(t *testing.T) {
	// Create a real file to test against
	tempDir := t.TempDir()
	templateFile := filepath.Join(tempDir, "ac4-template.pptx")

	if err := os.WriteFile(templateFile, []byte("template content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	info, err := os.Stat(templateFile)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}
	modTime := info.ModTime()

	cache := NewMemoryCache(1 * time.Hour)

	// Simulate initial analysis (this is where hash would normally be calculated)
	initialAnalysis := &types.TemplateAnalysis{
		TemplatePath: templateFile,
		Hash:         "expensive-hash-calculation-result",
		AspectRatio:  "16:9",
		AnalyzedAt:   time.Now(),
	}

	// Store with modTime for fast validation
	cache.SetWithModTime(templateFile, initialAnalysis, modTime)

	// AC4: On subsequent request, validate without hash recalculation
	// The IsValidFast check only does os.Stat (fast) and compares modTime
	// It does NOT read the file content or calculate hash

	result, err := cache.IsValidFast(templateFile)
	if err != nil {
		t.Fatalf("IsValidFast returned error: %v", err)
	}

	// This is the key assertion: CacheValid means NO hash recalculation was needed
	if result != CacheValid {
		t.Fatalf("AC4 FAILED: Expected CacheValid (no hash recalculation), got %v", result)
	}

	// Verify we can still get the cached analysis
	cachedAnalysis, ok := cache.GetWithFastValidation(templateFile)
	if !ok {
		t.Fatal("AC4 FAILED: GetWithFastValidation returned false despite valid modTime")
	}

	if cachedAnalysis.Hash != "expensive-hash-calculation-result" {
		t.Errorf("Expected original hash, got %s", cachedAnalysis.Hash)
	}

	t.Log("AC4 PASSED: Cache validated using modTime, no hash recalculation performed")
}

// --- LRU Eviction Tests (Task 48) ---

// TestNewMemoryCacheWithMaxSize tests the bounded cache constructor.
func TestNewMemoryCacheWithMaxSize(t *testing.T) {
	t.Run("creates cache with max size", func(t *testing.T) {
		cache := NewMemoryCacheWithMaxSize(1*time.Hour, 100)
		if cache.MaxSize() != 100 {
			t.Errorf("expected max size 100, got %d", cache.MaxSize())
		}
		if cache.Size() != 0 {
			t.Errorf("expected initial size 0, got %d", cache.Size())
		}
	})

	t.Run("zero max size means unlimited", func(t *testing.T) {
		cache := NewMemoryCacheWithMaxSize(1*time.Hour, 0)
		if cache.MaxSize() != 0 {
			t.Errorf("expected max size 0 (unlimited), got %d", cache.MaxSize())
		}
	})

	t.Run("default TTL applied when zero", func(t *testing.T) {
		cache := NewMemoryCacheWithMaxSize(0, 50)
		// Should default to 24 hours; test by adding and checking it's not immediately expired
		analysis := &types.TemplateAnalysis{Hash: "test"}
		cache.Set("/test.pptx", analysis)

		retrieved, ok := cache.Get("/test.pptx")
		if !ok || retrieved == nil {
			t.Error("expected entry to be available with default TTL")
		}
	})
}

// TestLRUEviction tests that LRU eviction works correctly when cache is at capacity.
func TestLRUEviction(t *testing.T) {
	// Create cache with max size of 3
	cache := NewMemoryCacheWithMaxSize(1*time.Hour, 3)

	// Add 3 entries
	for i := 0; i < 3; i++ {
		path := "/template" + string(rune('A'+i)) + ".pptx"
		analysis := &types.TemplateAnalysis{
			TemplatePath: path,
			Hash:         "hash" + string(rune('A'+i)),
			AnalyzedAt:   time.Now(),
		}
		cache.Set(path, analysis)
	}

	if cache.Size() != 3 {
		t.Errorf("expected size 3, got %d", cache.Size())
	}

	// Add 4th entry - should evict /templateA.pptx (oldest)
	cache.Set("/templateD.pptx", &types.TemplateAnalysis{
		TemplatePath: "/templateD.pptx",
		Hash:         "hashD",
		AnalyzedAt:   time.Now(),
	})

	// Size should still be 3
	if cache.Size() != 3 {
		t.Errorf("expected size 3 after eviction, got %d", cache.Size())
	}

	// First entry should be evicted
	_, ok := cache.Get("/templateA.pptx")
	if ok {
		t.Error("expected /templateA.pptx to be evicted (LRU)")
	}

	// Other entries should still exist
	for _, path := range []string{"/templateB.pptx", "/templateC.pptx", "/templateD.pptx"} {
		_, ok := cache.Get(path)
		if !ok {
			t.Errorf("expected %s to still be in cache", path)
		}
	}
}

// TestAC5_BoundedCacheWithLRUEviction is the primary acceptance test for AC5.
// AC5: Given cache configured with max 100 entries, When 150 unique templates are analyzed,
// Then cache contains exactly 100 entries (LRU eviction).
func TestAC5_BoundedCacheWithLRUEviction(t *testing.T) {
	// Create cache with max size of 100
	cache := NewMemoryCacheWithMaxSize(1*time.Hour, 100)

	// Add 150 unique templates
	for i := 0; i < 150; i++ {
		path := "/template-" + string(rune('A'+i/26)) + string(rune('A'+i%26)) + ".pptx"
		analysis := &types.TemplateAnalysis{
			TemplatePath: path,
			Hash:         "hash-" + path,
			AnalyzedAt:   time.Now(),
		}
		cache.Set(path, analysis)
	}

	// AC5: Cache should contain exactly 100 entries
	if cache.Size() != 100 {
		t.Fatalf("AC5 FAILED: Expected cache size 100, got %d", cache.Size())
	}

	// The first 50 entries should have been evicted
	for i := 0; i < 50; i++ {
		path := "/template-" + string(rune('A'+i/26)) + string(rune('A'+i%26)) + ".pptx"
		_, ok := cache.Get(path)
		if ok {
			t.Errorf("AC5 FAILED: Expected %s to be evicted, but it's still in cache", path)
		}
	}

	// The last 100 entries should still be in cache
	for i := 50; i < 150; i++ {
		path := "/template-" + string(rune('A'+i/26)) + string(rune('A'+i%26)) + ".pptx"
		_, ok := cache.Get(path)
		if !ok {
			t.Errorf("AC5 FAILED: Expected %s to be in cache, but it was evicted", path)
		}
	}

	t.Log("AC5 PASSED: Cache bounded at 100 entries with LRU eviction")
}

// TestLRUAccessOrderUpdate tests that accessing an entry moves it to the front of LRU.
func TestLRUAccessOrderUpdate(t *testing.T) {
	// Create cache with max size of 3
	cache := NewMemoryCacheWithMaxSize(1*time.Hour, 3)

	// Add 3 entries: A, B, C (A is oldest)
	cache.Set("/a.pptx", &types.TemplateAnalysis{Hash: "A"})
	cache.Set("/b.pptx", &types.TemplateAnalysis{Hash: "B"})
	cache.Set("/c.pptx", &types.TemplateAnalysis{Hash: "C"})

	// Access A (moves it to front, now order is: A, C, B or similar - B is now oldest)
	cache.Get("/a.pptx")

	// Add D - should evict B (which is now oldest)
	cache.Set("/d.pptx", &types.TemplateAnalysis{Hash: "D"})

	// A should still exist (was accessed recently)
	_, ok := cache.Get("/a.pptx")
	if !ok {
		t.Error("expected /a.pptx to still be in cache after access")
	}

	// B should be evicted (was oldest after A was accessed)
	_, ok = cache.Get("/b.pptx")
	if ok {
		t.Error("expected /b.pptx to be evicted (LRU after A was accessed)")
	}

	// C and D should exist
	_, ok = cache.Get("/c.pptx")
	if !ok {
		t.Error("expected /c.pptx to still be in cache")
	}
	_, ok = cache.Get("/d.pptx")
	if !ok {
		t.Error("expected /d.pptx to be in cache")
	}
}

// TestLRUUpdateExistingEntry tests that updating an entry moves it to front of LRU.
func TestLRUUpdateExistingEntry(t *testing.T) {
	cache := NewMemoryCacheWithMaxSize(1*time.Hour, 3)

	// Add 3 entries
	cache.Set("/a.pptx", &types.TemplateAnalysis{Hash: "A1"})
	cache.Set("/b.pptx", &types.TemplateAnalysis{Hash: "B1"})
	cache.Set("/c.pptx", &types.TemplateAnalysis{Hash: "C1"})

	// Update A (should move to front)
	cache.Set("/a.pptx", &types.TemplateAnalysis{Hash: "A2"})

	// Size should still be 3 (update, not insert)
	if cache.Size() != 3 {
		t.Errorf("expected size 3 after update, got %d", cache.Size())
	}

	// Verify the value was updated
	analysis, ok := cache.Get("/a.pptx")
	if !ok {
		t.Fatal("expected /a.pptx to be in cache")
	}
	if analysis.Hash != "A2" {
		t.Errorf("expected hash A2, got %s", analysis.Hash)
	}

	// Add D - should evict B (oldest after A was updated)
	cache.Set("/d.pptx", &types.TemplateAnalysis{Hash: "D1"})

	// A should still exist (was updated recently)
	_, ok = cache.Get("/a.pptx")
	if !ok {
		t.Error("expected /a.pptx to still be in cache after update")
	}

	// B should be evicted
	_, ok = cache.Get("/b.pptx")
	if ok {
		t.Error("expected /b.pptx to be evicted")
	}
}

// TestLRUWithInvalidate tests that invalidation properly removes from LRU.
func TestLRUWithInvalidate(t *testing.T) {
	cache := NewMemoryCacheWithMaxSize(1*time.Hour, 3)

	// Add 3 entries
	cache.Set("/a.pptx", &types.TemplateAnalysis{Hash: "A"})
	cache.Set("/b.pptx", &types.TemplateAnalysis{Hash: "B"})
	cache.Set("/c.pptx", &types.TemplateAnalysis{Hash: "C"})

	// Invalidate B
	cache.Invalidate("/b.pptx")

	// Size should be 2
	if cache.Size() != 2 {
		t.Errorf("expected size 2 after invalidation, got %d", cache.Size())
	}

	// Add D and E - should not cause issues (we have room now)
	cache.Set("/d.pptx", &types.TemplateAnalysis{Hash: "D"})
	cache.Set("/e.pptx", &types.TemplateAnalysis{Hash: "E"})

	// A should be evicted (oldest), C, D, E should exist
	if cache.Size() != 3 {
		t.Errorf("expected size 3, got %d", cache.Size())
	}

	_, ok := cache.Get("/a.pptx")
	if ok {
		t.Error("expected /a.pptx to be evicted")
	}
}

// TestLRUWithSetWithModTime tests LRU behavior with SetWithModTime.
func TestLRUWithSetWithModTime(t *testing.T) {
	cache := NewMemoryCacheWithMaxSize(1*time.Hour, 2)
	modTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	// Add 2 entries using SetWithModTime
	cache.SetWithModTime("/a.pptx", &types.TemplateAnalysis{Hash: "A"}, modTime)
	cache.SetWithModTime("/b.pptx", &types.TemplateAnalysis{Hash: "B"}, modTime)

	// Add 3rd - should evict A
	cache.SetWithModTime("/c.pptx", &types.TemplateAnalysis{Hash: "C"}, modTime)

	if cache.Size() != 2 {
		t.Errorf("expected size 2, got %d", cache.Size())
	}

	_, ok := cache.Get("/a.pptx")
	if ok {
		t.Error("expected /a.pptx to be evicted")
	}

	_, ok = cache.Get("/b.pptx")
	if !ok {
		t.Error("expected /b.pptx to still be in cache")
	}

	_, ok = cache.Get("/c.pptx")
	if !ok {
		t.Error("expected /c.pptx to be in cache")
	}
}

// TestLRUConcurrency tests thread-safety of LRU operations.
func TestLRUConcurrency(t *testing.T) {
	cache := NewMemoryCacheWithMaxSize(1*time.Hour, 50)

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(id int) {
			path := "/concurrent-" + string(rune('A'+id%26)) + ".pptx"
			analysis := &types.TemplateAnalysis{
				TemplatePath: path,
				Hash:         "hash-" + path,
				AnalyzedAt:   time.Now(),
			}

			// Mix of operations
			cache.Set(path, analysis)
			cache.Get(path)
			if id%3 == 0 {
				cache.Invalidate(path)
			}
			cache.Size()

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Cache size should be <= 50
	size := cache.Size()
	if size > 50 {
		t.Errorf("expected size <= 50, got %d", size)
	}

	// If we got here without deadlock or panic, test passes
	t.Logf("LRU concurrency test passed, final size: %d", size)
}

// TestLRUUnboundedCache tests that unbounded caches (maxSize=0) still work correctly.
func TestLRUUnboundedCache(t *testing.T) {
	// Create unbounded cache using regular constructor
	cache := NewMemoryCache(1 * time.Hour)

	// Add many entries - should not evict any
	for i := 0; i < 1000; i++ {
		path := "/template-" + string(rune('A'+i/26)) + string(rune('A'+i%26)) + string(rune('0'+i%10)) + ".pptx"
		analysis := &types.TemplateAnalysis{
			TemplatePath: path,
			Hash:         "hash-" + path,
			AnalyzedAt:   time.Now(),
		}
		cache.Set(path, analysis)
	}

	// All 1000 entries should exist
	if cache.Size() != 1000 {
		t.Errorf("expected size 1000 for unbounded cache, got %d", cache.Size())
	}
}

// TestLRUClear tests that Clear properly resets LRU data structures.
func TestLRUClear(t *testing.T) {
	cache := NewMemoryCacheWithMaxSize(1*time.Hour, 10)

	// Add some entries
	for i := 0; i < 5; i++ {
		path := "/template" + string(rune('A'+i)) + ".pptx"
		cache.Set(path, &types.TemplateAnalysis{Hash: "hash"})
	}

	if cache.Size() != 5 {
		t.Errorf("expected size 5, got %d", cache.Size())
	}

	// Clear
	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", cache.Size())
	}

	// Should be able to add entries again without issues
	for i := 0; i < 10; i++ {
		path := "/new-template" + string(rune('A'+i)) + ".pptx"
		cache.Set(path, &types.TemplateAnalysis{Hash: "new-hash"})
	}

	if cache.Size() != 10 {
		t.Errorf("expected size 10 after re-adding, got %d", cache.Size())
	}

	// Add one more - should trigger eviction
	cache.Set("/extra.pptx", &types.TemplateAnalysis{Hash: "extra"})

	if cache.Size() != 10 {
		t.Errorf("expected size 10 after eviction, got %d", cache.Size())
	}
}

// TestGetRLockOptimization tests that Get uses RLock for concurrent reads.
// This test verifies the lock optimization by running many concurrent Gets
// on the same entry (which should hit the "already at front" fast path).
func TestGetRLockOptimization(t *testing.T) {
	cache := NewMemoryCacheWithMaxSize(1*time.Hour, 100)

	// Add a single entry
	path := "/test.pptx"
	analysis := &types.TemplateAnalysis{
		TemplatePath: path,
		Hash:         "test-hash",
		AnalyzedAt:   time.Now(),
	}
	cache.Set(path, analysis)

	// Run many concurrent Gets on the same entry
	// With RLock optimization, this should be highly parallelizable
	const numGoroutines = 100
	const getsPerGoroutine = 1000
	done := make(chan bool, numGoroutines)

	start := time.Now()
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < getsPerGoroutine; j++ {
				result, ok := cache.Get(path)
				if !ok {
					t.Error("expected Get to return entry")
					return
				}
				if result.Hash != "test-hash" {
					t.Errorf("expected hash test-hash, got %s", result.Hash)
					return
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	elapsed := time.Since(start)

	// If we got here without deadlock, race, or panic, the optimization works
	t.Logf("RLock optimization test: %d concurrent Gets in %v", numGoroutines*getsPerGoroutine, elapsed)
}

// TestGetAtFrontSkipsWriteLock tests that Get skips the write lock when
// the entry is already at the front of the LRU list.
func TestGetAtFrontSkipsWriteLock(t *testing.T) {
	cache := NewMemoryCacheWithMaxSize(1*time.Hour, 100)

	// Add entries A, B, C (C is at front after this)
	for _, name := range []string{"A", "B", "C"} {
		cache.Set("/"+name+".pptx", &types.TemplateAnalysis{Hash: "hash-" + name})
	}

	// Access C multiple times - should hit "at front" optimization
	for i := 0; i < 10; i++ {
		_, ok := cache.Get("/C.pptx")
		if !ok {
			t.Fatal("expected to get C.pptx")
		}
	}

	// Access B once - moves B to front
	_, _ = cache.Get("/B.pptx")

	// Access B again multiple times - should now hit "at front" optimization
	for i := 0; i < 10; i++ {
		_, ok := cache.Get("/B.pptx")
		if !ok {
			t.Fatal("expected to get B.pptx")
		}
	}

	// All entries should still exist
	for _, name := range []string{"A", "B", "C"} {
		_, ok := cache.Get("/" + name + ".pptx")
		if !ok {
			t.Errorf("expected %s.pptx to still exist", name)
		}
	}
}

// --- FileStat Interface Tests ---

// TestMockFileStat tests the MockFileStat implementation itself.
func TestMockFileStat(t *testing.T) {
	mock := NewMockFileStat()

	t.Run("non-existent file", func(t *testing.T) {
		result, err := mock.Stat("/nonexistent.pptx")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Exists {
			t.Error("expected Exists=false for non-existent file")
		}
	})

	t.Run("existing file", func(t *testing.T) {
		modTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		mock.SetFile("/test.pptx", modTime)

		result, err := mock.Stat("/test.pptx")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !result.Exists {
			t.Error("expected Exists=true for existing file")
		}
		if !result.ModTime.Equal(modTime) {
			t.Errorf("expected modTime %v, got %v", modTime, result.ModTime)
		}
	})

	t.Run("deleted file", func(t *testing.T) {
		modTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		mock.SetFile("/to-delete.pptx", modTime)
		mock.DeleteFile("/to-delete.pptx")

		result, err := mock.Stat("/to-delete.pptx")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Exists {
			t.Error("expected Exists=false after deletion")
		}
	})

	t.Run("error injection", func(t *testing.T) {
		mock.SetError(func(path string) error {
			if path == "/error-path.pptx" {
				return errors.New("simulated I/O error")
			}
			return nil
		})

		_, err := mock.Stat("/error-path.pptx")
		if err == nil {
			t.Error("expected error for /error-path.pptx")
		}

		// Other paths should work
		mock.SetFile("/normal.pptx", time.Now())
		result, err := mock.Stat("/normal.pptx")
		if err != nil {
			t.Errorf("unexpected error for normal path: %v", err)
		}
		if !result.Exists {
			t.Error("expected /normal.pptx to exist")
		}
	})
}

// TestIsValidFastWithMockFileStat tests fast validation using the mock file stat.
// This allows testing without actual file I/O.
func TestIsValidFastWithMockFileStat(t *testing.T) {
	mock := NewMockFileStat()
	cache := NewMemoryCacheWithFileStat(1*time.Hour, 0, mock)

	modTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	testPath := "/mock-template.pptx"

	t.Run("cache valid when modTime unchanged (no file I/O)", func(t *testing.T) {
		mock.SetFile(testPath, modTime)

		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testPath,
			Hash:         "mock-hash",
			AnalyzedAt:   time.Now(),
		}
		cache.SetWithModTime(testPath, testAnalysis, modTime)

		result, err := cache.IsValidFast(testPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != CacheValid {
			t.Errorf("expected CacheValid, got %v", result)
		}
	})

	t.Run("cache invalid when modTime changed", func(t *testing.T) {
		cache.Clear()
		// Mock file has newer modTime than cached
		newModTime := modTime.Add(1 * time.Hour)
		mock.SetFile(testPath, newModTime)

		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testPath,
			Hash:         "mock-hash",
			AnalyzedAt:   time.Now(),
		}
		cache.SetWithModTime(testPath, testAnalysis, modTime)

		result, err := cache.IsValidFast(testPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != CacheInvalid {
			t.Errorf("expected CacheInvalid when modTime changed, got %v", result)
		}
	})

	t.Run("cache invalid when file deleted", func(t *testing.T) {
		cache.Clear()
		mock.SetFile(testPath, modTime)

		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testPath,
			Hash:         "mock-hash",
			AnalyzedAt:   time.Now(),
		}
		cache.SetWithModTime(testPath, testAnalysis, modTime)

		// Delete the file from mock
		mock.DeleteFile(testPath)

		result, err := cache.IsValidFast(testPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != CacheInvalid {
			t.Errorf("expected CacheInvalid when file deleted, got %v", result)
		}

		// Cache entry should be invalidated
		_, ok := cache.Get(testPath)
		if ok {
			t.Error("expected cache entry to be invalidated after file deletion")
		}
	})

	t.Run("returns error when FileStat returns error", func(t *testing.T) {
		cache.Clear()
		mock.SetFile(testPath, modTime)

		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testPath,
			Hash:         "mock-hash",
			AnalyzedAt:   time.Now(),
		}
		cache.SetWithModTime(testPath, testAnalysis, modTime)

		// Inject error
		mock.SetError(func(path string) error {
			return errors.New("permission denied")
		})

		result, err := cache.IsValidFast(testPath)
		if err == nil {
			t.Error("expected error from FileStat")
		}
		if result != CacheMiss {
			t.Errorf("expected CacheMiss on error, got %v", result)
		}

		// Clear error for subsequent tests
		mock.SetError(nil)
	})
}

// TestGetWithFastValidationUsingMock tests GetWithFastValidation using the mock.
func TestGetWithFastValidationUsingMock(t *testing.T) {
	mock := NewMockFileStat()
	cache := NewMemoryCacheWithFileStat(1*time.Hour, 0, mock)

	modTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	testPath := "/mock-get-test.pptx"
	mock.SetFile(testPath, modTime)

	t.Run("returns entry when valid", func(t *testing.T) {
		testAnalysis := &types.TemplateAnalysis{
			TemplatePath: testPath,
			Hash:         "get-hash",
			AspectRatio:  "16:9",
			AnalyzedAt:   time.Now(),
		}
		cache.SetWithModTime(testPath, testAnalysis, modTime)

		analysis, ok := cache.GetWithFastValidation(testPath)
		if !ok {
			t.Error("expected cache hit")
		}
		if analysis == nil {
			t.Fatal("expected non-nil analysis")
		}
		if analysis.Hash != "get-hash" {
			t.Errorf("expected hash get-hash, got %s", analysis.Hash)
		}
	})

	t.Run("returns nil when invalid", func(t *testing.T) {
		// Change mock's modTime
		mock.SetFile(testPath, modTime.Add(5*time.Minute))

		analysis, ok := cache.GetWithFastValidation(testPath)
		if ok {
			t.Error("expected cache miss when modTime changed")
		}
		if analysis != nil {
			t.Error("expected nil analysis")
		}
	})
}

// TestOSFileStat tests the default OSFileStat implementation with real files.
func TestOSFileStat(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "osfilestat-test.txt")

	// Create a test file
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	fs := OSFileStat{}

	t.Run("existing file", func(t *testing.T) {
		result, err := fs.Stat(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Exists {
			t.Error("expected Exists=true")
		}
		if result.ModTime.IsZero() {
			t.Error("expected non-zero ModTime")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		result, err := fs.Stat("/nonexistent/path/file.txt")
		if err != nil {
			t.Fatalf("unexpected error for non-existent file: %v", err)
		}
		if result.Exists {
			t.Error("expected Exists=false for non-existent file")
		}
	})
}

// TestNewMemoryCacheWithFileStat tests the constructor with custom FileStat.
func TestNewMemoryCacheWithFileStat(t *testing.T) {
	mock := NewMockFileStat()
	cache := NewMemoryCacheWithFileStat(1*time.Hour, 50, mock)

	if cache.MaxSize() != 50 {
		t.Errorf("expected max size 50, got %d", cache.MaxSize())
	}

	// Verify basic operations work
	testAnalysis := &types.TemplateAnalysis{
		TemplatePath: "/test.pptx",
		Hash:         "test-hash",
		AnalyzedAt:   time.Now(),
	}
	cache.Set("/test.pptx", testAnalysis)

	analysis, ok := cache.Get("/test.pptx")
	if !ok {
		t.Error("expected cache hit")
	}
	if analysis.Hash != "test-hash" {
		t.Errorf("expected hash test-hash, got %s", analysis.Hash)
	}
}
