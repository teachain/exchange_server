# Matchengine State Recovery Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete state recovery functionality in Go matchengine by wiring up LoadFromSlice to engine, implementing operlog replay, and adding a recovery mode for order processing.

**Architecture:** 
- Add `PutOrderNoLock` method to Engine for recovery mode (skips balance lock, operlog, kafka)
- Add `LoadOrdersFromSlice` method to Engine to restore orders to orderbooks
- Add `LoadBalancesFromSlice` method to BalanceManager to restore balances
- Wire up `InitFromDB` in main.go to load slice + replay operlogs
- Track last operlog ID in slice_history table

**Tech Stack:** Go, MySQL, existing persist.OperLogWriter

---

## File Structure

- Modify: `services/matchengine/internal/engine/engine.go` - Add recovery methods
- Modify: `services/matchengine/internal/engine/matching.go` - Add `PutOrderNoLock`
- Modify: `services/matchengine/internal/balance/balance.go` - Add `SetBalance` variants for recovery
- Modify: `services/matchengine/internal/order/orderbook.go` - Add method to put order directly
- Modify: `services/matchengine/internal/persist/slice.go` - Fix `LoadFromSlice` to return data
- Modify: `services/matchengine/cmd/main.go` - Wire up state recovery

---

### Task 1: Add PutOrderNoLock to Engine

**Files:**
- Modify: `services/matchengine/internal/engine/matching.go`
- Modify: `services/matchengine/internal/engine/engine.go`

- [ ] **Step 1: Read current matching.go to understand ProcessOrder**

```go
// ProcessOrder currently:
// 1. Locks balance
// 2. Sends Kafka order event
// 3. Writes operlog
// 4. Calls match()
// 5. Appends order history if finished
```

- [ ] **Step 2: Add PutOrderNoLock method in matching.go**

```go
func (e *Engine) PutOrderNoLock(incoming *order.Order) ([]*Trade, error) {
    ob := e.getOrCreateOrderBookLocked(incoming.Market)
    
    trades, err := e.match(ob, incoming)
    if err != nil {
        return nil, err
    }
    
    // For recovery: don't append history, don't send kafka
    // Just put order in book if not fully matched
    if incoming.Left.IsPositive() && !incoming.IsFinished() {
        incoming.Status = order.OrderStatusPartial
        ob.Add(incoming)
    }
    
    return trades, nil
}
```

- [ ] **Step 3: Add PutOrderNoLock to Engine struct in engine.go**

```go
func (e *Engine) PutOrderNoLock(incoming *order.Order) ([]*Trade, error)
```

- [ ] **Step 4: Test - compile to check syntax**

Run: `cd /Users/dm/Downloads/go-project/services/matchengine && go build ./...`
Expected: Compiles without errors

- [ ] **Step 5: Commit**

```bash
git add services/matchengine/internal/engine/matching.go services/matchengine/internal/engine/engine.go
git commit -m "feat(matchengine): add PutOrderNoLock for recovery mode"
```

---

### Task 2: Add LoadOrdersToEngine and LoadBalancesToEngine

**Files:**
- Modify: `services/matchengine/internal/engine/engine.go`
- Modify: `services/matchengine/internal/order/orderbook.go`

- [ ] **Step 1: Add PutOrderToBook method in orderbook.go**

```go
func (ob *OrderBook) PutOrder(o *order.Order) {
    ob.Add(o)
}
```

- [ ] **Step 2: Add LoadOrdersToEngine in engine.go**

```go
func (e *Engine) LoadOrdersToEngine(orders map[string][]*order.Order) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    
    for market, marketOrders := range orders {
        ob := e.getOrCreateOrderBookLocked(market)
        for _, ord := range marketOrders {
            if ord.IsActive() {
                ob.PutOrder(ord)
            }
        }
    }
    return nil
}
```

- [ ] **Step 3: Add LoadBalancesToEngine in engine.go**

```go
func (e *Engine) LoadBalancesToEngine(balances map[string]*balance.Balance) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    
    for key, bal := range balances {
        parts := strings.Split(key, ":")
        if len(parts) != 2 {
            continue
        }
        userID, _ := strconv.ParseUint(parts[0], 10, 32)
        e.balances.SetBalance(uint32(userID), bal.Asset, bal.Available, bal.Frozen)
    }
    return nil
}
```

- [ ] **Step 4: Test - compile to check syntax**

Run: `cd /Users/dm/Downloads/go-project/services/matchengine && go build ./...`
Expected: Compiles without errors

- [ ] **Step 5: Commit**

```bash
git add services/matchengine/internal/engine/engine.go services/matchengine/internal/order/orderbook.go
git commit -m "feat(matchengine): add LoadOrdersToEngine and LoadBalancesToEngine"
```

---

### Task 3: Fix LoadFromSlice to return data and track slice info

**Files:**
- Modify: `services/matchengine/internal/persist/slice.go`

- [ ] **Step 1: Modify LoadFromSlice signature to return loaded data**

```go
func (sm *SliceManager) LoadFromSlice() (*LoadedSlice, error) {
    var si sliceInfo
    err := sm.db.QueryRow(
        "SELECT id, timestamp, path FROM slice_history ORDER BY timestamp DESC LIMIT 1",
    ).Scan(&si.ID, &si.Timestamp, &si.Path)

    if err == sql.ErrNoRows {
        return nil, nil // No slice found, cold start
    }
    if err != nil {
        return nil, fmt.Errorf("failed to find latest slice: %w", err)
    }

    orders, err := sm.LoadOrdersFromFile(filepath.Join(si.Path, "orders"))
    if err != nil {
        return nil, fmt.Errorf("failed to load orders: %w", err)
    }

    balances, err := sm.LoadBalancesFromFile(filepath.Join(si.Path, "balances"))
    if err != nil {
        return nil, fmt.Errorf("failed to load balances: %w", err)
    }

    // Get last operlog ID at slice time
    var lastOperLogID int64
    sm.db.QueryRow(
        "SELECT end_oper_id FROM slice_history WHERE id = ?", si.ID,
    ).Scan(&lastOperLogID)

    return &LoadedSlice{
        SliceID:      si.ID,
        Timestamp:    si.Timestamp,
        Orders:       orders,
        Balances:     balances,
        LastOperLogID: lastOperLogID,
    }, nil
}

type LoadedSlice struct {
    SliceID      int64
    Timestamp    time.Time
    Orders       map[string][]*order.Order
    Balances     map[string]*balance.Balance
    LastOperLogID int64
}
```

- [ ] **Step 2: Test - compile to check syntax**

Run: `cd /Users/dm/Downloads/go-project/services/matchengine && go build ./...`
Expected: Compiles without errors

- [ ] **Step 3: Commit**

```bash
git add services/matchengine/internal/persist/slice.go
git commit -m "feat(matchengine): fix LoadFromSlice to return loaded data"
```

---

### Task 4: Implement operlog replay with handler

**Files:**
- Modify: `services/matchengine/internal/engine/engine.go`
- Modify: `services/matchengine/internal/persist/operlog.go`

- [ ] **Step 1: Create OperLogReplayHandler in engine.go**

```go
type OperLogReplayHandler struct {
    engine *Engine
}

func (h *OperLogReplayHandler) HandleOrderCreate(data []byte) error {
    var logData struct {
        OrderID  uint64 `json:"order_id"`
        UserID   uint32 `json:"user_id"`
        Market   string `json:"market"`
        Side     uint8  `json:"side"`
        Type     uint8  `json:"type"`
        Price    string `json:"price"`
        Amount   string `json:"amount"`
        Left     string `json:"left"`
        TakerFee string `json:"taker_fee"`
    }
    if err := json.Unmarshal(data, &logData); err != nil {
        return err
    }

    price, _ := decimal.NewFromString(logData.Price)
    amount, _ := decimal.NewFromString(logData.Amount)
    left, _ := decimal.NewFromString(logData.Left)
    takerFee, _ := decimal.NewFromString(logData.TakerFee)

    ord := &order.Order{
        ID:       logData.OrderID,
        UserID:   logData.UserID,
        Market:   logData.Market,
        Side:     order.Side(logData.Side),
        Type:     order.OrderType(logData.Type),
        Price:    price,
        Amount:   amount,
        Left:     left,
        TakerFee: takerFee,
        Status:   order.OrderStatusPending,
    }

    // Use PutOrderNoLock to skip balance lock, kafka, operlog
    _, err := h.engine.PutOrderNoLock(ord)
    return err
}

func (h *OperLogReplayHandler) HandleOrderDeal(data []byte) error {
    // During replay, we skip deal handling - orders already restored from slice
    // Deal history will be rebuilt from order states
    return nil
}

func (h *OperLogReplayHandler) HandleOrderCancel(data []byte) error {
    var logData struct {
        OrderID uint64 `json:"order_id"`
        UserID  uint32 `json:"user_id"`
        Market  string `json:"market"`
    }
    if err := json.Unmarshal(data, &logData); err != nil {
        return err
    }

    h.engine.mu.Lock()
    defer h.engine.mu.Unlock()
    
    ob, ok := h.engine.orderBooks[logData.Market]
    if !ok {
        return nil
    }
    ord, ok := ob.GetOrder(logData.OrderID)
    if !ok {
        return nil
    }
    ob.Remove(logData.OrderID)
    ord.Status = order.OrderStatusCanceled
    return nil
}

func (h *OperLogReplayHandler) HandleBalanceChange(data []byte) error {
    // Balance changes during operlog replay are handled by LoadBalancesToEngine
    // which restores the final state - no need to replay incremental changes
    return nil
}
```

- [ ] **Step 2: Add ReplayOperLogs method to Engine in engine.go**

```go
func (e *Engine) ReplayOperLogs(fromID int64, operLogWriter *persist.OperLogWriter) error {
    handler := &OperLogReplayHandler{engine: e}
    return operLogWriter.Replay(fromID, handler)
}
```

- [ ] **Step 3: Add missing import in engine.go**

```go
import (
    "encoding/json"  // Add this
    "strings"       // Add this for parsing balance key
    "strconv"       // Add this for parsing user ID
    // ... existing imports
)
```

- [ ] **Step 4: Test - compile to check syntax**

Run: `cd /Users/dm/Downloads/go-project/services/matchengine && go build ./...`
Expected: Compiles without errors

- [ ] **Step 5: Commit**

```bash
git add services/matchengine/internal/engine/engine.go services/matchengine/internal/persist/operlog.go
git commit -m "feat(matchengine): implement operlog replay handler"
```

---

### Task 5: Wire up state recovery in main.go

**Files:**
- Modify: `services/matchengine/cmd/main.go`

- [ ] **Step 1: Replace loadBalanceFromDB with InitFromDB**

```go
func InitFromDB(db *sqlx.DB, e *engine.Engine, sm *persist.SliceManager, operLogWriter *persist.OperLogWriter) error {
    // Try to load from slice
    loadedSlice, err := sm.LoadFromSlice()
    if err != nil {
        return fmt.Errorf("load from slice failed: %w", err)
    }
    
    if loadedSlice == nil {
        // Cold start - no slice found
        fmt.Println("no slice found, starting fresh")
        return nil
    }
    
    fmt.Printf("loaded slice id=%d, timestamp=%s\n", loadedSlice.SliceID, loadedSlice.Timestamp)
    
    // Restore balances
    if err := e.LoadBalancesToEngine(loadedSlice.Balances); err != nil {
        return fmt.Errorf("load balances failed: %w", err)
    }
    fmt.Printf("loaded %d balance records\n", len(loadedSlice.Balances))
    
    // Restore orders
    if err := e.LoadOrdersToEngine(loadedSlice.Orders); err != nil {
        return fmt.Errorf("load orders failed: %w", err)
    }
    
    orderCount := 0
    for _, orders := range loadedSlice.Orders {
        orderCount += len(orders)
    }
    fmt.Printf("loaded %d orders\n", orderCount)
    
    // Replay operlogs since slice
    if loadedSlice.LastOperLogID > 0 {
        fmt.Printf("replaying operlogs from id=%d\n", loadedSlice.LastOperLogID)
        if err := e.ReplayOperLogs(loadedSlice.LastOperLogID, operLogWriter); err != nil {
            return fmt.Errorf("replay operlogs failed: %w", err)
        }
        fmt.Println("operlog replay completed")
    }
    
    return nil
}
```

- [ ] **Step 2: Update slice_history table schema to store end_oper_id**

The current schema doesn't have `end_oper_id`. Need to update `InitDB` in slice.go:

```go
func (sm *SliceManager) InitDB() error {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS slice_history (
            id BIGINT AUTO_INCREMENT PRIMARY KEY,
            timestamp DATETIME NOT NULL,
            path VARCHAR(512) NOT NULL,
            end_oper_id BIGINT NOT NULL DEFAULT 0,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )`,
        // ... rest unchanged
    }
    // ...
}
```

- [ ] **Step 3: Update MakeSlice to record end_oper_id**

In `MakeSlice` and `DumpOrders/DumpBalances`, record the current operlog ID:

```go
func (sm *SliceManager) MakeSlice() error {
    // ... existing code ...
    
    // Get current operlog ID before snapshot
    var endOperLogID int64
    if sm.db != nil {
        sm.db.QueryRow("SELECT MAX(id) FROM operlog").Scan(&endOperLogID)
    }
    
    // Insert with end_oper_id
    _, err := sm.db.Exec(
        "INSERT INTO slice_history (timestamp, path, end_oper_id) VALUES (?, ?, ?)",
        timestamp, slicePath, endOperLogID,
    )
    // ...
}
```

- [ ] **Step 4: Update main() to use InitFromDB**

```go
func main() {
    // ... existing config loading ...

    e := engine.NewEngine()

    // ... database connection ...

    // Initialize slice manager first (before operlog writer)
    sm := persist.NewSliceManager(db, e, sliceInterval, sliceKeepTime, sliceDir)
    if err := sm.InitDB(); err != nil {
        fmt.Println("init slice db failed:", err.Error())
        os.Exit(1)
    }

    // Initialize operlog writer
    operLogWriter := persist.NewOperLogWriter(db)
    e.SetOperLogWriter(operLogWriter)

    // State recovery - load from slice + replay operlogs
    if err := InitFromDB(db, e, sm, operLogWriter); err != nil {
        fmt.Println("state recovery failed:", err.Error())
        os.Exit(1)
    }

    // Start periodic slices after recovery
    go sm.StartPeriodicSlices(sliceInterval, e)

    // ... rest unchanged ...
}
```

- [ ] **Step 5: Test - compile to check syntax**

Run: `cd /Users/dm/Downloads/go-project/services/matchengine && go build ./...`
Expected: Compiles without errors

- [ ] **Step 6: Commit**

```bash
git add services/matchengine/cmd/main.go services/matchengine/internal/persist/slice.go
git commit -m "feat(matchengine): wire up state recovery in main"
```

---

### Task 6: Verify and test the complete flow

- [ ] **Step 1: Run tests**

Run: `cd /Users/dm/Downloads/go-project/services/matchengine && go test ./...`
Expected: All tests pass

- [ ] **Step 2: Build the binary**

Run: `cd /Users/dm/Downloads/go-project/services/matchengine && go build -o matchengine ./cmd/main.go`
Expected: Builds successfully

- [ ] **Step 3: Verify the flow**

The complete state recovery flow should now be:
1. On startup, `InitFromDB` is called
2. `LoadFromSlice` finds the latest slice from `slice_history`
3. Balances are loaded from slice files and restored to `BalanceManager`
4. Orders are loaded from slice files and restored to order books
5. `ReplayOperLogs` replays all operlogs since the slice's `end_oper_id`
6. Engine is now in consistent state matching the last snapshot

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(matchengine): complete state recovery implementation"
```
