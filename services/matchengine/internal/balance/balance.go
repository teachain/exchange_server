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
	UserID    uint32
	Asset     string
	Available decimal.Decimal
	Frozen    decimal.Decimal
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

func (bm *BalanceManager) key(userID uint32, asset string) string {
	return fmt.Sprintf("%d:%s", userID, asset)
}

func (bm *BalanceManager) GetBalance(userID uint32, asset string) (decimal.Decimal, decimal.Decimal) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	b, ok := bm.balances[bm.key(userID, asset)]
	if !ok {
		return decimal.Zero, decimal.Zero
	}
	return b.Available, b.Frozen
}

func (bm *BalanceManager) Freeze(userID uint32, asset string, amount decimal.Decimal) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	k := bm.key(userID, asset)
	b, ok := bm.balances[k]
	if !ok {
		return ErrInsufficientBalance
	}

	available := b.Available
	if available.LessThan(amount) {
		return ErrInsufficientBalance
	}

	b.Available = b.Available.Sub(amount)
	b.Frozen = b.Frozen.Add(amount)
	return nil
}

func (bm *BalanceManager) Unfreeze(userID uint32, asset string, amount decimal.Decimal) error {
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
	b.Available = b.Available.Add(amount)
	return nil
}

func (bm *BalanceManager) LockBalance(userID uint32, asset string, amount decimal.Decimal) error {
	return bm.Freeze(userID, asset, amount)
}

func (bm *BalanceManager) UnlockBalance(userID uint32, asset string, amount decimal.Decimal) error {
	return bm.Unfreeze(userID, asset, amount)
}

func (bm *BalanceManager) Sub(userID uint32, asset string, amount decimal.Decimal) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	k := bm.key(userID, asset)
	b, ok := bm.balances[k]
	if !ok {
		return ErrBalanceNotFound
	}

	if b.Available.LessThan(amount) {
		return ErrInsufficientBalance
	}

	b.Available = b.Available.Sub(amount)
	return nil
}

func (bm *BalanceManager) Add(userID uint32, asset string, amount decimal.Decimal) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	k := bm.key(userID, asset)
	b, ok := bm.balances[k]
	if !ok {
		bm.balances[k] = &Balance{
			UserID:    userID,
			Asset:     asset,
			Available: amount,
			Frozen:    decimal.Zero,
		}
		return nil
	}

	b.Available = b.Available.Add(amount)
	return nil
}

func (bm *BalanceManager) Change(userID uint32, asset string, change decimal.Decimal) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	k := bm.key(userID, asset)
	b, ok := bm.balances[k]
	if !ok {
		if change.IsNegative() {
			return ErrBalanceNotFound
		}
		bm.balances[k] = &Balance{
			UserID:    userID,
			Asset:     asset,
			Available: change,
			Frozen:    decimal.Zero,
		}
		return nil
	}

	if change.IsNegative() {
		if b.Available.LessThan(change.Abs()) {
			return ErrInsufficientBalance
		}
		b.Available = b.Available.Add(change)
	} else {
		b.Available = b.Available.Add(change)
	}
	return nil
}

func (bm *BalanceManager) DeductBalance(userID uint32, asset string, amount decimal.Decimal) error {
	return bm.Sub(userID, asset, amount)
}

func (bm *BalanceManager) AddBalance(userID uint32, asset string, amount decimal.Decimal) {
	bm.Add(userID, asset, amount)
}

func (bm *BalanceManager) SetBalance(userID uint32, asset string, available, frozen decimal.Decimal) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	k := bm.key(userID, asset)
	bm.balances[k] = &Balance{
		UserID:    userID,
		Asset:     asset,
		Available: available,
		Frozen:    frozen,
	}
}

func (bm *BalanceManager) GetAllBalancesForUser(userID uint32) map[string]*Balance {
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

func (bm *BalanceManager) GetAllBalances() map[string]*Balance {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make(map[string]*Balance)
	for k, b := range bm.balances {
		result[k] = b
	}
	return result
}
