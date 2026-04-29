package openai

import "sync"

type Cache struct {
	mu       sync.RWMutex
	cacheMap map[string][]float32
}

func NewCache() *Cache {
	return &Cache{cacheMap: make(map[string][]float32)}
}

func (c *Cache) Get(q string) ([]float32, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.cacheMap[q]
	return v, ok
}

func (c *Cache) Set(q string, v []float32) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cacheMap[q] = v
}

func (c *Cache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cacheMap)
}
