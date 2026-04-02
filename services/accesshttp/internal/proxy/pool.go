package proxy

import (
	"net/http"
	"sync"
	"time"
)

type Pool struct {
	backend   string
	clients   []*http.Client
	mu        sync.Mutex
	index     int
	transport *http.Transport
}

func NewPool(backend string, size int) *Pool {
	transport := &http.Transport{
		MaxIdleConns:        size * 2,
		MaxIdleConnsPerHost: size,
		IdleConnTimeout:     90 * time.Second,
	}

	clients := make([]*http.Client, size)
	for i := range clients {
		clients[i] = &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		}
	}

	return &Pool{
		backend:   backend,
		clients:   clients,
		transport: transport,
	}
}

func (p *Pool) GetClient() *http.Client {
	p.mu.Lock()
	defer p.mu.Unlock()

	client := p.clients[p.index]
	p.index = (p.index + 1) % len(p.clients)

	return client
}

func (p *Pool) Close() {
	p.transport.CloseIdleConnections()
}
