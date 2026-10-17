package testutil

import (
	"github.com/sebahrens/json2pptx/internal/types"
	"sync"
)

// TemplateCache is a filesystem-independent cache for handler tests. Unlike the
// production cache it keeps deliberately synthetic analyses without stat calls.
type TemplateCache struct {
	mu   sync.RWMutex
	data map[string]*types.TemplateAnalysis
}

func NewTemplateCache() *TemplateCache {
	return &TemplateCache{data: map[string]*types.TemplateAnalysis{}}
}
func (c *TemplateCache) Get(path string) (*types.TemplateAnalysis, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	a, ok := c.data[path]
	return a, ok
}
func (c *TemplateCache) Set(path string, a *types.TemplateAnalysis) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[path] = a
}
func (c *TemplateCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, path)
}
func (c *TemplateCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = map[string]*types.TemplateAnalysis{}
}
func (c *TemplateCache) Size() int { c.mu.RLock(); defer c.mu.RUnlock(); return len(c.data) }
func (c *TemplateCache) IsValid(path, hash string) bool {
	a, ok := c.Get(path)
	return ok && a.Hash == hash
}
