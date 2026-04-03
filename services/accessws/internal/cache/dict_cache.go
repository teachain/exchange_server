package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type CacheEntry struct {
	Value     []byte
	ExpiresAt time.Time
}

type DictCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	ttl     time.Duration
}

func NewDictCache(ttl time.Duration) *DictCache {
	return &DictCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
}

func (c *DictCache) generateKey(command uint32, body []byte) string {
	hash := sha256.Sum256(append([]byte{byte(command >> 24), byte(command >> 16), byte(command >> 8), byte(command)}, body...))
	return hex.EncodeToString(hash[:])
}

func (c *DictCache) Get(command uint32, body []byte) ([]byte, bool) {
	key := c.generateKey(command, body)

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}

	return entry.Value, true
}

func (c *DictCache) Set(command uint32, body []byte, value []byte) {
	key := c.generateKey(command, body)

	c.mu.Lock()
	c.entries[key] = &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *DictCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*CacheEntry)
	c.mu.Unlock()
}
