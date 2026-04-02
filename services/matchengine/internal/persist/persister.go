package persist

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/viabtc/go-project/services/matchengine/internal/balance"
	"github.com/viabtc/go-project/services/matchengine/internal/order"
)

type Persister interface {
	GetAllOrders() map[string][]*order.Order
	GetAllBalances() map[string]*balance.Balance
}

type StateSnapshot struct {
	Orders   map[string][]*order.Order
	Balances map[string]*balance.Balance
	Markets  map[string]*MarketSnapshot
}

type MarketSnapshot struct {
	Name       string
	Orders     map[uint64]*order.Order
	UserOrders map[uint32]map[uint64]*order.Order
}

func (sm *SliceManager) DumpOrders(orders map[string][]*order.Order) error {
	slicePath := filepath.Join(sm.sliceDir, fmt.Sprintf("slice_%d", time.Now().Unix()))
	if err := os.MkdirAll(slicePath, 0755); err != nil {
		return fmt.Errorf("failed to create slice directory: %w", err)
	}

	ordersPath := filepath.Join(slicePath, "orders")
	file, err := os.Create(ordersPath)
	if err != nil {
		return fmt.Errorf("failed to create orders file: %w", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	for market, marketOrders := range orders {
		for _, ord := range marketOrders {
			data, err := sm.SerializeOrder(ord)
			if err != nil {
				continue
			}
			record := map[string]interface{}{
				"market": market,
				"order":  string(data),
			}
			if err := enc.Encode(record); err != nil {
				continue
			}
		}
	}

	tx, err := sm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO slice_history (timestamp, path) VALUES (?, ?)",
		time.Now(), slicePath,
	)
	if err != nil {
		return fmt.Errorf("failed to insert slice history: %w", err)
	}

	sliceID, _ := result.LastInsertId()

	stmt, err := tx.Prepare("INSERT INTO slice_order (slice_id, order_data) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for market, marketOrders := range orders {
		for _, ord := range marketOrders {
			data, err := sm.SerializeOrder(ord)
			if err != nil {
				continue
			}
			record := map[string]interface{}{
				"market": market,
				"order":  string(data),
			}
			recordBytes, _ := json.Marshal(record)
			stmt.Exec(sliceID, string(recordBytes))
		}
	}

	return tx.Commit()
}

func (sm *SliceManager) DumpBalances(balances map[string]*balance.Balance) error {
	slicePath := filepath.Join(sm.sliceDir, fmt.Sprintf("slice_%d", time.Now().Unix()))
	if err := os.MkdirAll(slicePath, 0755); err != nil {
		return fmt.Errorf("failed to create slice directory: %w", err)
	}

	balancesPath := filepath.Join(slicePath, "balances")
	file, err := os.Create(balancesPath)
	if err != nil {
		return fmt.Errorf("failed to create balances file: %w", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	for key, bal := range balances {
		data, err := sm.SerializeBalance(bal)
		if err != nil {
			continue
		}
		record := map[string]interface{}{
			"key":     key,
			"balance": string(data),
		}
		if err := enc.Encode(record); err != nil {
			continue
		}
	}

	tx, err := sm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO slice_history (timestamp, path) VALUES (?, ?)",
		time.Now(), slicePath,
	)
	if err != nil {
		return fmt.Errorf("failed to insert slice history: %w", err)
	}

	sliceID, _ := result.LastInsertId()

	stmt, err := tx.Prepare("INSERT INTO slice_balance (slice_id, balance_data) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for key, bal := range balances {
		data, err := sm.SerializeBalance(bal)
		if err != nil {
			continue
		}
		record := map[string]interface{}{
			"key":     key,
			"balance": string(data),
		}
		recordBytes, _ := json.Marshal(record)
		stmt.Exec(sliceID, string(recordBytes))
	}

	return tx.Commit()
}

func (sm *SliceManager) LoadOrders() (map[string][]*order.Order, error) {
	result := make(map[string][]*order.Order)

	rows, err := sm.db.Query(`
		SELECT so.order_data FROM slice_order so
		INNER JOIN slice_history sh ON so.slice_id = sh.id
		ORDER BY sh.timestamp DESC LIMIT 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var orderData string
		if err := rows.Scan(&orderData); err != nil {
			continue
		}

		var record struct {
			Market string `json:"market"`
			Order  string `json:"order"`
		}
		if err := json.Unmarshal([]byte(orderData), &record); err != nil {
			continue
		}

		ord, err := sm.DeserializeOrder([]byte(record.Order))
		if err != nil {
			continue
		}
		result[record.Market] = append(result[record.Market], ord)
	}

	return result, nil
}

func (sm *SliceManager) LoadBalances() (map[string]*balance.Balance, error) {
	result := make(map[string]*balance.Balance)

	rows, err := sm.db.Query(`
		SELECT sb.balance_data FROM slice_balance sb
		INNER JOIN slice_history sh ON sb.slice_id = sh.id
		ORDER BY sh.timestamp DESC LIMIT 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var balanceData string
		if err := rows.Scan(&balanceData); err != nil {
			continue
		}

		var record struct {
			Key     string `json:"key"`
			Balance string `json:"balance"`
		}
		if err := json.Unmarshal([]byte(balanceData), &record); err != nil {
			continue
		}

		bal, err := sm.DeserializeBalance([]byte(record.Balance))
		if err != nil {
			continue
		}
		result[record.Key] = bal
	}

	return result, nil
}

func (sm *SliceManager) StartPeriodicSlices(interval time.Duration, persister Persister) {
	go func() {
		ticker := time.NewTicker(interval)
		for range ticker.C {
			orders := persister.GetAllOrders()
			balances := persister.GetAllBalances()

			sm.DumpOrders(orders)
			sm.DumpBalances(balances)
		}
	}()
}
