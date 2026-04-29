package vector

import "sync"

type Cache struct {
	mu     sync.RWMutex
	vecMap map[int64][]float32
}

func New() *Cache {
	return &Cache{vecMap: make(map[int64][]float32)}
}

func (c *Cache) Set(id int64, v []float32) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vecMap[id] = v
}

func (c *Cache) Get(id int64) ([]float32, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.vecMap[id]
	return v, ok
}

func (c *Cache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.vecMap)
}
