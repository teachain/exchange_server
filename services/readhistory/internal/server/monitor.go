package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type MonitorServer struct {
	engine    *gin.Engine
	port      int
	requests  uint64
	startTime time.Time
	mu        int64
}

type HealthResp struct {
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

type StatusResp struct {
	Uptime     int64  `json:"uptime"`
	GoVersion  string `json:"go_version"`
	NumGoroute int    `json:"num_goroutine"`
	Requests   uint64 `json:"requests"`
}

type StatsResp struct {
	MemAlloc     uint64 `json:"mem_alloc"`
	MemSys       uint64 `json:"mem_sys"`
	NumGC        uint32 `json:"num_gc"`
	GoVersion    string `json:"go_version"`
	NumGoroutine int    `json:"num_goroutine"`
}

func NewMonitorServer(port int) *MonitorServer {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	ms := &MonitorServer{
		engine:    engine,
		port:      port,
		startTime: time.Now(),
	}

	ms.setupRoutes()
	return ms
}

func (ms *MonitorServer) setupRoutes() {
	ms.engine.GET("/health", ms.health)
	ms.engine.GET("/status", ms.status)
	ms.engine.GET("/stats", ms.stats)
}

func (ms *MonitorServer) health(c *gin.Context) {
	resp := HealthResp{
		Status:    "ok",
		Timestamp: time.Now().Unix(),
	}
	c.JSON(http.StatusOK, resp)
}

func (ms *MonitorServer) status(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	resp := StatusResp{
		Uptime:     int64(time.Since(ms.startTime).Seconds()),
		GoVersion:  runtime.Version(),
		NumGoroute: runtime.NumGoroutine(),
		Requests:   atomic.LoadUint64(&ms.requests),
	}
	c.JSON(http.StatusOK, resp)
}

func (ms *MonitorServer) stats(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	resp := StatsResp{
		MemAlloc:     m.Alloc,
		MemSys:       m.Sys,
		NumGC:        m.NumGC,
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
	}
	c.JSON(http.StatusOK, resp)
}

func (ms *MonitorServer) Start() error {
	addr := fmt.Sprintf(":%d", ms.port)
	return ms.engine.Run(addr)
}

func (ms *MonitorServer) StartOnAddr(addr string) error {
	return ms.engine.Run(addr)
}

func (ms *MonitorServer) IncrementRequests() {
	atomic.AddUint64(&ms.requests, 1)
}

func (ms *MonitorServer) GetRequestCount() uint64 {
	return atomic.LoadUint64(&ms.requests)
}

func (ms *MonitorServer) GetEngine() *gin.Engine {
	return ms.engine
}

func (ms *MonitorServer) GetPort() int {
	return ms.port
}

func (ms *MonitorServer) MarshalJSON() ([]byte, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return json.Marshal(map[string]interface{}{
		"uptime":        int64(time.Since(ms.startTime).Seconds()),
		"go_version":    runtime.Version(),
		"num_goroutine": runtime.NumGoroutine(),
		"requests":      atomic.LoadUint64(&ms.requests),
		"mem_alloc":     m.Alloc,
		"mem_sys":       m.Sys,
		"num_gc":        m.NumGC,
	})
}
