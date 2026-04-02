package persist

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/matchengine/internal/balance"
	"github.com/viabtc/go-project/services/matchengine/internal/order"
)

type SliceManager struct {
	db            *sql.DB
	sliceInterval time.Duration
	sliceKeepTime time.Duration
	sliceDir      string
}

type sliceInfo struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
}

func NewSliceManager(db *sql.DB, sliceInterval, sliceKeepTime time.Duration, sliceDir string) *SliceManager {
	return &SliceManager{
		db:            db,
		sliceInterval: sliceInterval,
		sliceKeepTime: sliceKeepTime,
		sliceDir:      sliceDir,
	}
}

func (sm *SliceManager) InitDB() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS slice_history (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			timestamp DATETIME NOT NULL,
			path VARCHAR(512) NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS slice_order (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			slice_id BIGINT NOT NULL,
			order_data TEXT NOT NULL,
			FOREIGN KEY (slice_id) REFERENCES slice_history(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS slice_balance (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			slice_id BIGINT NOT NULL,
			balance_data TEXT NOT NULL,
			FOREIGN KEY (slice_id) REFERENCES slice_history(id) ON DELETE CASCADE
		)`,
	}

	for _, q := range queries {
		if _, err := sm.db.Exec(q); err != nil {
			return fmt.Errorf("failed to create slice table: %w", err)
		}
	}

	return nil
}

func (sm *SliceManager) MakeSlice() error {
	timestamp := time.Now()

	slicePath := filepath.Join(sm.sliceDir, fmt.Sprintf("slice_%d", timestamp.Unix()))

	if err := os.MkdirAll(slicePath, 0755); err != nil {
		return fmt.Errorf("failed to create slice directory: %w", err)
	}

	ordersPath := filepath.Join(slicePath, "orders")
	balancesPath := filepath.Join(slicePath, "balances")

	if err := sm.DumpOrdersToFile(ordersPath); err != nil {
		return fmt.Errorf("failed to dump orders: %w", err)
	}

	if err := sm.DumpBalancesToFile(balancesPath); err != nil {
		return fmt.Errorf("failed to dump balances: %w", err)
	}

	_, err := sm.db.Exec(
		"INSERT INTO slice_history (timestamp, path) VALUES (?, ?)",
		timestamp, slicePath,
	)
	if err != nil {
		return fmt.Errorf("failed to record slice: %w", err)
	}

	go sm.ClearOldSlices()

	return nil
}

func (sm *SliceManager) DumpOrdersToFile(path string) error {
	return nil
}

func (sm *SliceManager) DumpBalancesToFile(path string) error {
	return nil
}

func (sm *SliceManager) ClearOldSlices() error {
	if sm.sliceKeepTime == 0 {
		return nil
	}

	cutoff := time.Now().Add(-sm.sliceKeepTime)

	rows, err := sm.db.Query(
		"SELECT id, path FROM slice_history WHERE timestamp < ?",
		cutoff,
	)
	if err != nil {
		return fmt.Errorf("failed to query old slices: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var si sliceInfo
		if err := rows.Scan(&si.ID, &si.Path); err != nil {
			continue
		}

		os.RemoveAll(si.Path)

		sm.db.Exec("DELETE FROM slice_order WHERE slice_id = ?", si.ID)
		sm.db.Exec("DELETE FROM slice_balance WHERE slice_id = ?", si.ID)
		sm.db.Exec("DELETE FROM slice_history WHERE id = ?", si.ID)
	}

	return nil
}

func (sm *SliceManager) LoadFromSlice() error {
	var si sliceInfo
	err := sm.db.QueryRow(
		"SELECT id, timestamp, path FROM slice_history ORDER BY timestamp DESC LIMIT 1",
	).Scan(&si.ID, &si.Timestamp, &si.Path)

	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to find latest slice: %w", err)
	}

	ordersPath := filepath.Join(si.Path, "orders")
	balancesPath := filepath.Join(si.Path, "balances")

	if err := sm.LoadOrdersFromFile(ordersPath); err != nil {
		return fmt.Errorf("failed to load orders: %w", err)
	}

	if err := sm.LoadBalancesFromFile(balancesPath); err != nil {
		return fmt.Errorf("failed to load balances: %w", err)
	}

	return nil
}

func (sm *SliceManager) LoadOrdersFromFile(path string) error {
	return nil
}

func (sm *SliceManager) LoadBalancesFromFile(path string) error {
	return nil
}

type orderData struct {
	ID         uint64 `json:"id"`
	UserID     uint32 `json:"user_id"`
	Market     string `json:"market"`
	Side       uint8  `json:"side"`
	Type       uint8  `json:"type"`
	Price      string `json:"price"`
	Amount     string `json:"amount"`
	Left       string `json:"left"`
	Freeze     string `json:"freeze"`
	Status     uint8  `json:"status"`
	Source     string `json:"source"`
	CreateTime int64  `json:"create_time"`
	UpdateTime int64  `json:"update_time"`
}

func (sm *SliceManager) SerializeOrder(ord *order.Order) ([]byte, error) {
	data := orderData{
		ID:         ord.ID,
		UserID:     ord.UserID,
		Market:     ord.Market,
		Side:       uint8(ord.Side),
		Type:       uint8(ord.Type),
		Price:      ord.Price.String(),
		Amount:     ord.Amount.String(),
		Left:       ord.Left.String(),
		Freeze:     ord.Freeze.String(),
		Status:     uint8(ord.Status),
		Source:     ord.Source,
		CreateTime: ord.CreateTime.Unix(),
		UpdateTime: ord.UpdateTime.Unix(),
	}
	return json.Marshal(data)
}

func (sm *SliceManager) DeserializeOrder(data []byte) (*order.Order, error) {
	var od orderData
	if err := json.Unmarshal(data, &od); err != nil {
		return nil, err
	}

	price, _ := decimal.NewFromString(od.Price)
	amount, _ := decimal.NewFromString(od.Amount)
	left, _ := decimal.NewFromString(od.Left)
	freeze, _ := decimal.NewFromString(od.Freeze)

	return &order.Order{
		ID:         od.ID,
		UserID:     od.UserID,
		Market:     od.Market,
		Side:       order.Side(od.Side),
		Type:       order.OrderType(od.Type),
		Price:      price,
		Amount:     amount,
		Left:       left,
		Freeze:     freeze,
		Status:     order.OrderStatus(od.Status),
		Source:     od.Source,
		CreateTime: time.Unix(od.CreateTime, 0),
		UpdateTime: time.Unix(od.UpdateTime, 0),
	}, nil
}

type balanceData struct {
	UserID    uint32 `json:"user_id"`
	Asset     string `json:"asset"`
	Available string `json:"available"`
	Frozen    string `json:"frozen"`
}

func (sm *SliceManager) SerializeBalance(bal *balance.Balance) ([]byte, error) {
	data := balanceData{
		UserID:    bal.UserID,
		Asset:     bal.Asset,
		Available: bal.Available.String(),
		Frozen:    bal.Frozen.String(),
	}
	return json.Marshal(data)
}

func (sm *SliceManager) DeserializeBalance(data []byte) (*balance.Balance, error) {
	var bd balanceData
	if err := json.Unmarshal(data, &bd); err != nil {
		return nil, err
	}

	available, _ := decimal.NewFromString(bd.Available)
	frozen, _ := decimal.NewFromString(bd.Frozen)

	return &balance.Balance{
		UserID:    bd.UserID,
		Asset:     bd.Asset,
		Available: available,
		Frozen:    frozen,
	}, nil
}

func (sm *SliceManager) ForkAndDump(suffix string) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(execPath, "-dump", suffix)
	cmd.Start()

	return nil
}
