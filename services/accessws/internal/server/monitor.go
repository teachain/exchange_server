package server

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"
)

type MonitorServer struct {
	addr     string
	httpSrv  *http.Server
	mu       sync.RWMutex
	srvStats *Stats
}

type Stats struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	StartTime int64  `json:"start_time"`
}

func NewMonitorServer(addr string) *MonitorServer {
	m := &MonitorServer{
		addr: addr,
		srvStats: &Stats{
			Service:   "accessws",
			Version:   "1.0.0",
			StartTime: time.Now().Unix(),
		},
	}
	return m
}

func (m *MonitorServer) setupRoutes() {
}

func (m *MonitorServer) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (m *MonitorServer) status(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"goroutines": runtime.NumGoroutine(),
		"memory":     memStats.Alloc,
		"timestamp":  time.Now().Unix(),
	})
}

func (m *MonitorServer) stats(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.srvStats)
}

func (m *MonitorServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", m.health)
	mux.HandleFunc("/status", m.status)
	mux.HandleFunc("/stats", m.stats)

	m.httpSrv = &http.Server{
		Addr:    m.addr,
		Handler: mux,
	}
	return m.httpSrv.ListenAndServe()
}

func (m *MonitorServer) Shutdown() error {
	if m.httpSrv == nil {
		return nil
	}
	return m.httpSrv.Shutdown(context.Background())
}
