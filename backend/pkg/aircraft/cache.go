package aircraft

import "sync"

// Cache is a thread-safe store for the latest aircraft snapshot.
type Cache struct {
	mu   sync.RWMutex
	data []AircraftState
}

func NewCache() *Cache { return &Cache{} }

func (c *Cache) GetAll() []AircraftState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]AircraftState, len(c.data))
	copy(out, c.data)
	return out
}

func (c *Cache) Set(states []AircraftState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make([]AircraftState, len(states))
	copy(c.data, states)
}

func (c *Cache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}
