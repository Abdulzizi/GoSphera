package maritime

import "sync"

// Cache is a thread-safe store of ship states keyed by MMSI.
type Cache struct {
	mu   sync.RWMutex
	data map[int]ShipState
}

func NewCache() *Cache {
	return &Cache{data: make(map[int]ShipState)}
}

func (c *Cache) Set(s ShipState) {
	c.mu.Lock()
	c.data[s.MMSI] = s
	c.mu.Unlock()
}

func (c *Cache) GetAll() []ShipState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]ShipState, 0, len(c.data))
	for _, s := range c.data {
		out = append(out, s)
	}
	return out
}

func (c *Cache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}
