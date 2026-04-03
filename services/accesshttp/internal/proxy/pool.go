package proxy

import (
	"context"
	"errors"
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
	available bool
}

func (p *Pool) IsAvailable() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.available
}

func (p *Pool) SetAvailable(available bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.available = available
}

func (p *Pool) CheckHealth() error {
	if p == nil {
		return errors.New("pool is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.backend+"/health", nil)
	if err != nil {
		p.SetAvailable(false)
		return err
	}

	resp, err := p.clients[0].Do(req)
	if err != nil {
		p.SetAvailable(false)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.SetAvailable(false)
		return errors.New("health check failed")
	}

	p.SetAvailable(true)
	return nil
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
