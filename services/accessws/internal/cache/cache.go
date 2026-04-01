package cache

import (
	"sync"
	"time"
)

type CacheValue struct {
	Data      []byte
	Timestamp time.Time
}

type Cache struct {
	data      map[string]*CacheValue
	ttl       time.Duration
	mu        sync.RWMutex
	cleanupCh chan struct{}
}

func NewCache(ttlSeconds float64) *Cache {
	c := &Cache{
		data:      make(map[string]*CacheValue),
		ttl:       time.Duration(ttlSeconds * float64(time.Second)),
		cleanupCh: make(chan struct{}),
	}
	go c.cleanupLoop()
	return c
}

func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	if !ok {
		return nil, false
	}
	if time.Since(v.Timestamp) > c.ttl {
		return nil, false
	}
	return v.Data, true
}

func (c *Cache) Set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = &CacheValue{
		Data:      data,
		Timestamp: time.Now(),
	}
}

func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(60 * time.Second)
	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.cleanupCh:
			ticker.Stop()
			return
		}
	}
}

func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.data {
		if now.Sub(v.Timestamp) > c.ttl {
			delete(c.data, k)
		}
	}
}

func (c *Cache) Close() {
	close(c.cleanupCh)
}
