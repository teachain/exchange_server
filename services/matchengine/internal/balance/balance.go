package balance

import (
	"errors"
	"fmt"
	"sync"

	"github.com/shopspring/decimal"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrBalanceNotFound     = errors.New("balance not found")
	ErrInvalidAmount       = errors.New("invalid amount")
)

type Balance struct {
	UserID  int64
	Asset   string
	Balance decimal.Decimal
	Frozen  decimal.Decimal
}

type BalanceManager struct {
	mu       sync.RWMutex
	balances map[string]*Balance
}

func NewBalanceManager() *BalanceManager {
	return &BalanceManager{
		balances: make(map[string]*Balance),
	}
}

func (bm *BalanceManager) key(userID int64, asset string) string {
	return fmt.Sprintf("%d:%s", userID, asset)
}

func (bm *BalanceManager) GetBalance(userID int64, asset string) (decimal.Decimal, decimal.Decimal) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	b, ok := bm.balances[bm.key(userID, asset)]
	if !ok {
		return decimal.Zero, decimal.Zero
	}
	return b.Balance, b.Frozen
}

func (bm *BalanceManager) LockBalance(userID int64, asset string, amount decimal.Decimal) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	k := bm.key(userID, asset)
	b, ok := bm.balances[k]
	if !ok {
		return ErrInsufficientBalance
	}

	available := b.Balance.Sub(b.Frozen)
	if available.LessThan(amount) {
		return ErrInsufficientBalance
	}

	b.Frozen = b.Frozen.Add(amount)
	return nil
}

func (bm *BalanceManager) UnlockBalance(userID int64, asset string, amount decimal.Decimal) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	k := bm.key(userID, asset)
	b, ok := bm.balances[k]
	if !ok {
		return ErrBalanceNotFound
	}

	if b.Frozen.LessThan(amount) {
		return ErrInvalidAmount
	}

	b.Frozen = b.Frozen.Sub(amount)
	return nil
}

func (bm *BalanceManager) DeductBalance(userID int64, asset string, amount decimal.Decimal) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	k := bm.key(userID, asset)
	b, ok := bm.balances[k]
	if !ok {
		return ErrBalanceNotFound
	}

	if b.Balance.LessThan(amount) {
		return ErrInsufficientBalance
	}

	b.Balance = b.Balance.Sub(amount)
	return nil
}

func (bm *BalanceManager) AddBalance(userID int64, asset string, amount decimal.Decimal) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	k := bm.key(userID, asset)
	b, ok := bm.balances[k]
	if !ok {
		bm.balances[k] = &Balance{
			UserID:  userID,
			Asset:   asset,
			Balance: amount,
			Frozen:  decimal.Zero,
		}
		return
	}

	b.Balance = b.Balance.Add(amount)
}

func (bm *BalanceManager) SetBalance(userID int64, asset string, balance, frozen decimal.Decimal) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	k := bm.key(userID, asset)
	bm.balances[k] = &Balance{
		UserID:  userID,
		Asset:   asset,
		Balance: balance,
		Frozen:  frozen,
	}
}

func (bm *BalanceManager) GetAllBalancesForUser(userID int64) map[string]*Balance {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make(map[string]*Balance)
	prefix := fmt.Sprintf("%d:", userID)
	for k, b := range bm.balances {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			result[b.Asset] = b
		}
	}
	return result
}

func (bm *BalanceManager) GetAllBalancesForAsset(asset string) []*Balance {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var result []*Balance
	for _, b := range bm.balances {
		if b.Asset == asset {
			result = append(result, b)
		}
	}
	return result
}

func (bm *BalanceManager) ListAssets() []string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	assetSet := make(map[string]struct{})
	for _, b := range bm.balances {
		assetSet[b.Asset] = struct{}{}
	}

	assets := make([]string, 0, len(assetSet))
	for asset := range assetSet {
		assets = append(assets, asset)
	}
	return assets
}
