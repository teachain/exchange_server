package engine

import "sync"

type IDGenerator struct {
	mu    sync.Mutex
	value int64
}

func NewIDGenerator() *IDGenerator {
	return &IDGenerator{
		value: 0,
	}
}

func (g *IDGenerator) NextID() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
	return g.value
}
