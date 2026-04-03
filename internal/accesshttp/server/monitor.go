package server

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type MonitorServer struct {
	addr         string
	router       *gin.Engine
	httpSrv      *http.Server
	backendProxy interface {
		GetBackendStatus() map[string]bool
	}
	mu sync.RWMutex
}

func NewMonitorServer(addr string, backendProxy interface {
	GetBackendStatus() map[string]bool
}) *MonitorServer {
	router := gin.New()
	router.Use(gin.Recovery())

	m := &MonitorServer{
		addr:         addr,
		router:       router,
		backendProxy: backendProxy,
	}

	m.setupRoutes()

	return m
}

func (m *MonitorServer) setupRoutes() {
	m.router.GET("/health", m.health)
	m.router.GET("/status", m.status)
	m.router.GET("/stats", m.stats)
	m.router.GET("/backends", m.backends)
}

func (m *MonitorServer) health(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

func (m *MonitorServer) backends(c *gin.Context) {
	status := m.backendProxy.GetBackendStatus()
	allHealthy := true
	for _, v := range status {
		if !v {
			allHealthy = false
			break
		}
	}

	c.JSON(200, gin.H{
		"backends": status,
		"healthy":  allHealthy,
	})
}

func (m *MonitorServer) status(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	status := m.backendProxy.GetBackendStatus()
	allHealthy := true
	for _, v := range status {
		if !v {
			allHealthy = false
			break
		}
	}

	c.JSON(200, gin.H{
		"goroutines": runtime.NumGoroutine(),
		"memory":     memStats.Alloc,
		"timestamp":  time.Now().Unix(),
		"backends":   status,
		"healthy":    allHealthy,
	})
}

func (m *MonitorServer) stats(c *gin.Context) {
	c.JSON(200, gin.H{
		"service": "accesshttp",
		"version": "1.0.0",
	})
}

func (m *MonitorServer) Start() error {
	m.httpSrv = &http.Server{
		Addr:    m.addr,
		Handler: m.router,
	}
	return m.httpSrv.ListenAndServe()
}

func (m *MonitorServer) Shutdown() error {
	if m.httpSrv == nil {
		return nil
	}
	return m.httpSrv.Shutdown(context.Background())
}
