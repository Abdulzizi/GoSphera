package aggregator

import "sync"

// FireRecord is a single active-fire detection from NASA FIRMS.
type FireRecord struct {
	Lat        float64 `json:"latitude"`
	Lon        float64 `json:"longitude"`
	Brightness float64 `json:"brightness"`
	AcqDate    string  `json:"acq_date"`
	Confidence string  `json:"confidence"`
}

// Cache is a thread-safe store for fire records.
type Cache struct {
	mu   sync.RWMutex
	data []FireRecord
}

// NewCache creates an empty Cache.
func NewCache() *Cache {
	return &Cache{data: []FireRecord{}}
}

// GetAll returns a snapshot copy of all cached records.
func (c *Cache) GetAll() []FireRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]FireRecord, len(c.data))
	copy(out, c.data)
	return out
}

// Set replaces the entire cache with a deep copy of records.
func (c *Cache) Set(records []FireRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make([]FireRecord, len(records))
	copy(c.data, records)
}

// Count returns the number of cached records.
func (c *Cache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}
