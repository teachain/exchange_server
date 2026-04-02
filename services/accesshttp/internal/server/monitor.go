package server

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

type MonitorServer struct {
	addr    string
	router  *gin.Engine
	httpSrv *http.Server
}

func NewMonitorServer(addr string) *MonitorServer {
	router := gin.New()
	router.Use(gin.Recovery())

	m := &MonitorServer{
		addr:   addr,
		router: router,
	}

	m.setupRoutes()

	return m
}

func (m *MonitorServer) setupRoutes() {
	m.router.GET("/health", m.health)
	m.router.GET("/status", m.status)
	m.router.GET("/stats", m.stats)
}

func (m *MonitorServer) health(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

func (m *MonitorServer) status(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	c.JSON(200, gin.H{
		"goroutines": runtime.NumGoroutine(),
		"memory":     memStats.Alloc,
		"timestamp":  time.Now().Unix(),
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
