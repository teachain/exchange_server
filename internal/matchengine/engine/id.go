package engine

import "sync"

type IDGenerator struct {
	mu    sync.Mutex
	value uint64
}

func NewIDGenerator() *IDGenerator {
	return &IDGenerator{
		value: 0,
	}
}

func (g *IDGenerator) NextID() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
	return g.value
}
