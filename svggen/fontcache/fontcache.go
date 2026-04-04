// Package fontcache provides a shared, thread-safe cache for loaded font families.
//
// Font loading is expensive (parses TrueType files, ~5-20ms per font), so this
// package provides a singleton cache used by both the SVG builder and the text
// fitting engine. The cache is bounded to 50 entries with LRU eviction to prevent
// unbounded memory growth.
package fontcache

import (
	"container/list"
	"sync"

	"github.com/sebahrens/json2pptx/svggen/fonts"
	"github.com/tdewolff/canvas"
)

// maxEntries is the maximum number of font families kept in the cache.
// When exceeded, the least-recently-used entry is evicted.
const maxEntries = 50

// entry is a single cache entry pairing a name with its loaded font family.
type entry struct {
	key  string
	font *canvas.FontFamily
}

// cache is the package-level singleton font cache.
var cache = &fontCache{
	items: make(map[string]*list.Element),
	order: list.New(),
}

// fontCache is a bounded LRU cache of loaded canvas.FontFamily instances.
type fontCache struct {
	mu    sync.Mutex
	items map[string]*list.Element
	order *list.List // front = most recently used
}

// Get returns a cached font family for the given name, loading it if necessary.
// fallbackName is an optional system font to try when name is not found (pass ""
// to skip). The font loading strategy is:
//
//  1. Try loading name as a system font.
//  2. If that fails and fallbackName is non-empty, try fallbackName as a system font.
//  3. If that fails, cycle through common system fallbacks (Arial, Helvetica, DejaVu Sans).
//  4. If no system font is available, load the embedded Liberation Sans (metric-compatible
//     with Arial, works in headless/Docker environments).
//  5. If all attempts fail, return nil.
//
// Results (including nil) are cached so that repeated lookups for the same name
// are fast. The cache key is the requested name; the fallback name does not
// affect the key.
func Get(name string, fallbackName string) *canvas.FontFamily {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	// Fast path: already cached.
	if elem, ok := cache.items[name]; ok {
		cache.order.MoveToFront(elem)
		return elem.Value.(*entry).font
	}

	// Slow path: load the font.
	ff := loadFont(name, fallbackName)

	// Store in cache and evict if over capacity.
	e := &entry{key: name, font: ff}
	elem := cache.order.PushFront(e)
	cache.items[name] = elem

	if cache.order.Len() > maxEntries {
		cache.evictOldest()
	}

	return ff
}

// loadFont attempts to load a font family using the cascade described in Get.
func loadFont(name string, fallbackName string) *canvas.FontFamily {
	ff := canvas.NewFontFamily(name)

	// 1. Try loading the requested system font.
	if err := ff.LoadSystemFont(name, canvas.FontRegular); err == nil {
		_ = ff.LoadSystemFont(name, canvas.FontBold) // best-effort bold
		return ff
	}

	// 2. Try the explicit fallback name.
	if fallbackName != "" && fallbackName != name {
		if err := ff.LoadSystemFont(fallbackName, canvas.FontRegular); err == nil {
			_ = ff.LoadSystemFont(fallbackName, canvas.FontBold) // best-effort bold
			return ff
		}
	}

	// 3. Try common system fallbacks.
	for _, fb := range []string{"Arial", "Helvetica", "DejaVu Sans"} {
		if fb == name || fb == fallbackName {
			continue
		}
		if err := ff.LoadSystemFont(fb, canvas.FontRegular); err == nil {
			_ = ff.LoadSystemFont(fb, canvas.FontBold) // best-effort bold
			return ff
		}
	}

	// 4. Load embedded Liberation Sans (always available, metric-compatible with Arial).
	if err := ff.LoadFont(fonts.LiberationSansRegular, 0, canvas.FontRegular); err == nil {
		_ = ff.LoadFont(fonts.LiberationSansBold, 0, canvas.FontBold) // best-effort bold
		return ff
	}

	// 5. Nothing worked.
	return nil
}

// evictOldest removes the least-recently-used entry. Caller must hold mu.
func (c *fontCache) evictOldest() {
	oldest := c.order.Back()
	if oldest == nil {
		return
	}
	c.order.Remove(oldest)
	delete(c.items, oldest.Value.(*entry).key)
}

// Len returns the number of entries currently in the cache. Useful for testing.
func Len() int {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.order.Len()
}

// Reset clears the cache. Intended for testing only.
func Reset() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.items = make(map[string]*list.Element)
	cache.order.Init()
}
