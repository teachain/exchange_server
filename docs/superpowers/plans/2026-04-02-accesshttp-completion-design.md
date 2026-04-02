# Accesshttp Feature Completion Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan.

**Goal:** Complete accesshttp Go implementation to match C version functionality:
- Request timeout tracking with 504 responses
- Monitor HTTP server endpoint
- Backend connection pool with health checks
- Graceful shutdown handling
- Process resource limits (file/core limits)
- Log rotation

**Architecture:**
- Keep HTTP/JSON communication with backends (no binary RPC)
- Keep single-process + goroutine model (no multi-process)
- Add timeout tracking per request using context.WithTimeout
- Add separate monitor server on different port
- Implement connection pool for backends with keep-alive
- Add signal handling for graceful shutdown
- Set file/core limits on startup
- Implement structured logging with rotation

**Tech Stack:** Go, Gin, viper, standard library

---

## File Structure

- Modify: `services/accesshttp/cmd/main.go` - Add limits, logging, graceful shutdown
- Modify: `services/accesshttp/internal/server/server.go` - Add timeout, monitor server
- Modify: `services/accesshttp/internal/proxy/backend.go` - Add connection pool
- Create: `services/accesshttp/internal/server/shutdown.go` - Graceful shutdown handling
- Create: `services/accesshttp/internal/server/monitor.go` - Monitor server
- Create: `services/accesshttp/internal/proxy/pool.go` - Backend connection pool

---

### Task 1: Add process resource limits

**Files:**
- Modify: `services/accesshttp/cmd/main.go`

- [ ] **Step 1: Add resource limit functions**

```go
import (
    "golang.org/x/sys/unix"
)

func setFileLimit(max uint64) {
    var rlimit unix.Rlimit
    rlimit.Cur = max
    rlimit.Max = max
    unix.Setrlimit(unix.RLIMIT_NOFILE, &rlimit)
}

func setCoreLimit(max uint64) {
    var rlimit unix.Rlimit
    rlimit.Cur = max
    rlimit.Max = max
    unix.Setrlimit(unix.RLIMIT_CORE, &rlimit)
}
```

- [ ] **Step 2: Call setFileLimit and setCoreLimit at startup**

Add after config loading, before server creation:
```go
setFileLimit(1000000)
setCoreLimit(1000000000)
```

- [ ] **Step 3: Test - compile**

Run: `cd /Users/dm/Downloads/go-project/services/accesshttp && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add services/accesshttp/cmd/main.go
git commit -m "feat(accesshttp): add process resource limits"
```

---

### Task 2: Implement log rotation

**Files:**
- Modify: `services/accesshttp/cmd/main.go`
- Create: `services/accesshttp/internal/log/logger.go`

- [ ] **Step 1: Create log package**

```go
package log

import (
    "io"
    "os"
    "sync"
    "time"
    "path/filepath"
)

type Level int

const (
    LevelDebug Level = iota
    LevelInfo
    LevelWarn
    LevelError
    LevelFatal
)

type Logger struct {
    mu       sync.Mutex
    dir      string
    prefix   string
    maxSize  int64
    maxFiles int
    level    Level
    current  *os.File
    currentSize int64
}

func NewLogger(dir, prefix string, maxSize int64, maxFiles int) *Logger {
    return &Logger{
        dir:      dir,
        prefix:   prefix,
        maxSize:  maxSize,
        maxFiles: maxFiles,
        level:    LevelInfo,
    }
}

func (l *Logger) Init() error {
    os.MkdirAll(l.dir, 0755)
    return l.rotate()
}

func (l *Logger) rotate() error {
    if l.current != nil {
        l.current.Close()
    }
    
    now := time.Now()
    filename := filepath.Join(l.dir, fmt.Sprintf("%s.%s.log", 
        l.prefix, now.Format("20060102.150405")))
    
    f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return err
    }
    
    l.current = f
    l.currentSize = 0
    
    // Clean old files
    l.cleanOld()
    
    return nil
}

func (l *Logger) cleanOld() {
    pattern := filepath.Join(l.dir, l.prefix+".*.log")
    matches, _ := filepath.Glob(pattern)
    
    if len(matches) > l.maxFiles {
        // Sort and delete oldest
        oldest := matches[:len(matches)-l.maxFiles]
        for _, f := range oldest {
            os.Remove(f)
        }
    }
}

func (l *Logger) Write(p []byte) (n int, err error) {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    if l.currentSize >= l.maxSize {
        l.rotate()
    }
    
    n, err = l.current.Write(p)
    l.currentSize += int64(n)
    
    return n, err
}

func (l *Logger) Close() error {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    if l.current != nil {
        return l.current.Close()
    }
    return nil
}
```

- [ ] **Step 2: Update main.go to use log package**

Replace standard log with the rotation logger.

- [ ] **Step 3: Test - compile**

Run: `cd /Users/dm/Downloads/go-project/services/accesshttp && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add services/accesshttp/cmd/main.go services/accesshttp/internal/log/logger.go
git commit -m "feat(accesshttp): add log rotation"
```

---

### Task 3: Implement backend connection pool

**Files:**
- Modify: `services/accesshttp/internal/proxy/backend.go`
- Create: `services/accesshttp/internal/proxy/pool.go`

- [ ] **Step 1: Create connection pool**

```go
package proxy

import (
    "net/http"
    "sync"
    "time"
)

type Pool struct {
    backend   string
    clients   []*http.Client
    mu        sync.RWMutex
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
```

- [ ] **Step 2: Update BackendProxy to use pools**

Replace single http.Client with connection pools per backend:
```go
type BackendProxy struct {
    matchenginePool *Pool
    marketpricePool *Pool
    readhistoryPool *Pool
}
```

- [ ] **Step 3: Update ForwardTo* methods to use pools**

Replace `proxy.httpClient.Do(req)` with `proxy.matchenginePool.GetClient().Do(req)`

- [ ] **Step 4: Test - compile**

Run: `cd /Users/dm/Downloads/go-project/services/accesshttp && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add services/accesshttp/internal/proxy/backend.go services/accesshttp/internal/proxy/pool.go
git commit -m "feat(accesshttp): add backend connection pools"
```

---

### Task 4: Add request timeout tracking

**Files:**
- Modify: `services/accesshttp/internal/server/server.go`
- Modify: `services/accesshttp/internal/handler/jsonrpc.go`

- [ ] **Step 1: Add timeout to config**

In `config.go`:
```go
type Config struct {
    // ... existing fields
    Timeout time.Duration `mapstructure:"timeout"`
}
```

In `config.yaml`:
```yaml
timeout: 1s
```

- [ ] **Step 2: Update handler to use context.WithTimeout**

In `HandleJSONRPC`:
```go
func (h *Handler) HandleJSONRPC(c *gin.Context) {
    var req model.JSONRPCRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(200, model.NewParseError())
        return
    }
    
    if req.ID == nil {
        c.JSON(200, model.NewInvalidRequestError())
        return
    }
    
    // Create context with timeout
    ctx, cancel := context.WithTimeout(c.Request.Context(), h.cfg.Timeout)
    defer cancel()
    
    // Create request with timeout context
    req = req.WithContext(ctx)
    
    // ... rest of handler
}
```

- [ ] **Step 3: Update proxy calls to use context**

Update `ForwardTo*` methods to accept context and use `httprequest.WithContext(ctx)`

- [ ] **Step 4: Handle context deadline exceeded**

In proxy, check for timeout error and return 504:
```go
if errors.Is(err, context.DeadlineExceeded) {
    return nil, &model.RPCError{
        Code:    -32001,
        Message: "Gateway timeout",
    }
}
```

- [ ] **Step 5: Test - compile**

Run: `cd /Users/dm/Downloads/go-project/services/accesshttp && go build ./...`

- [ ] **Step 6: Commit**

```bash
git add services/accesshttp/internal/server/server.go services/accesshttp/internal/handler/jsonrpc.go
git commit -m "feat(accesshttp): add request timeout tracking"
```

---

### Task 5: Add monitor server

**Files:**
- Create: `services/accesshttp/internal/server/monitor.go`

- [ ] **Step 1: Create monitor server**

```go
package server

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "runtime"
    "time"
)

type MonitorServer struct {
    addr   string
    router *gin.Engine
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
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    c.JSON(200, gin.H{
        "goroutines": runtime.NumGoroutine(),
        "memory":     m.Alloc,
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
    return http.ListenAndServe(m.addr, m.router)
}
```

- [ ] **Step 2: Start monitor server in main.go**

```go
// Start monitor server
monitorAddr := viper.GetString("monitor.bind")
if monitorAddr == "" {
    monitorAddr = ":8081"
}
monitorServer := server.NewMonitorServer(monitorAddr)
go monitorServer.Start()
```

- [ ] **Step 3: Test - compile**

Run: `cd /Users/dm/Downloads/go-project/services/accesshttp && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add services/accesshttp/internal/server/monitor.go
git add services/accesshttp/cmd/main.go
git commit -m "feat(accesshttp): add monitor server"
```

---

### Task 6: Implement graceful shutdown

**Files:**
- Create: `services/accesshttp/internal/server/shutdown.go`
- Modify: `services/accesshttp/cmd/main.go`

- [ ] **Step 1: Create shutdown handler**

```go
package server

import (
    "context"
    "net/http"
    "sync"
    "time"
)

type GracefulShutdown struct {
    srv           *http.Server
    timeout       time.Duration
    wg            sync.WaitGroup
    shutdownOnce  sync.Once
}

func NewGracefulShutdown(addr string, handler http.Handler, timeout time.Duration) *GracefulShutdown {
    return &GracefulShutdown{
        srv: &http.Server{
            Addr:    addr,
            Handler: handler,
        },
        timeout: timeout,
    }
}

func (gs *GracefulShutdown) Start() error {
    return gs.srv.ListenAndServe()
}

func (gs *GracefulShutdown) Shutdown() error {
    var err error
    gs.shutdownOnce.Do(func() {
        ctx, cancel := context.WithTimeout(context.Background(), gs.timeout)
        defer cancel()
        
        err = gs.srv.Shutdown(ctx)
        gs.wg.Wait()
    })
    return err
}
```

- [ ] **Step 2: Update main.go with signal handling**

```go
func main() {
    // ... existing setup ...
    
    // Create channel to receive signals
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    
    // Start servers in goroutines
    go func() {
        srv.Start(addr)
    }()
    
    // Wait for shutdown signal
    <-sigCh
    
    log.Println("Shutting down gracefully...")
    
    // Give time for in-flight requests to complete
    time.Sleep(5 * time.Second)
    
    // Shutdown monitor server
    if err := monitorServer.Shutdown(); err != nil {
        log.Printf("Monitor shutdown error: %v", err)
    }
    
    // Shutdown main server
    if err := srv.Shutdown(); err != nil {
        log.Printf("Server shutdown error: %v", err)
    }
    
    log.Println("Shutdown complete")
}
```

- [ ] **Step 3: Test - compile**

Run: `cd /Users/dm/Downloads/go-project/services/accesshttp && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add services/accesshttp/internal/server/shutdown.go services/accesshttp/cmd/main.go
git commit -m "feat(accesshttp): add graceful shutdown"
```

---

### Task 7: Integration and final verification

- [ ] **Step 1: Full compile**

Run: `cd /Users/dm/Downloads/go-project/services/accesshttp && go build ./...`

- [ ] **Step 2: Run tests**

Run: `cd /Users/dm/Downloads/go-project/services/accesshttp && go test ./...`

- [ ] **Step 3: Final review**

Verify all features are wired correctly:
- Resource limits set at startup
- Log rotation working
- Connection pools created
- Request timeout tracking enabled
- Monitor server running
- Graceful shutdown on SIGINT/SIGTERM

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(accesshttp): complete all missing features"
```
