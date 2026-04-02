package server

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/viabtc/go-project/services/marketprice/internal/kline"
	"github.com/viabtc/go-project/services/marketprice/internal/market"
)

type MonitorServer struct {
	router    *gin.Engine
	port      int
	startTime time.Time
	marketMgr *market.Manager
	klineMgr  *kline.KlineManager
}

func NewMonitorServer(port int, marketMgr *market.Manager, klineMgr *kline.KlineManager) *MonitorServer {
	gin.SetMode(gin.ReleaseMode)
	return &MonitorServer{
		router:    gin.New(),
		port:      port,
		startTime: time.Now(),
		marketMgr: marketMgr,
		klineMgr:  klineMgr,
	}
}

func (m *MonitorServer) Start() error {
	m.setupRoutes()
	addr := fmt.Sprintf(":%d", m.port)
	return m.router.Run(addr)
}

func (m *MonitorServer) setupRoutes() {
	m.router.GET("/health", m.handleHealth)
	m.router.GET("/status", m.handleStatus)
	m.router.GET("/stats", m.handleStats)
}

func (m *MonitorServer) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	})
}

func (m *MonitorServer) handleStatus(c *gin.Context) {
	uptime := time.Since(m.startTime).Seconds()
	markets := m.marketMgr.ListMarkets()

	c.JSON(http.StatusOK, gin.H{
		"uptime":  uptime,
		"markets": len(markets),
		"status":  "running",
	})
}

func (m *MonitorServer) handleStats(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := time.Since(m.startTime).Seconds()
	markets := m.marketMgr.ListMarkets()

	stats := gin.H{
		"uptime":    uptime,
		"markets":   len(markets),
		"goroutine": runtime.NumGoroutine(),
		"memory": gin.H{
			"alloc":       memStats.Alloc,
			"total_alloc": memStats.TotalAlloc,
			"sys":         memStats.Sys,
			"num_gc":      memStats.NumGC,
		},
	}

	c.JSON(http.StatusOK, stats)
}
