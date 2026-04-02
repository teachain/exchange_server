package persist

import (
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
	return nil
}

func (sm *SliceManager) DumpBalances(balances map[string]*balance.Balance) error {
	return nil
}

func (sm *SliceManager) LoadOrders() (map[string][]*order.Order, error) {
	return nil, nil
}

func (sm *SliceManager) LoadBalances() (map[string]*balance.Balance, error) {
	return nil, nil
}

func (sm *SliceManager) StartPeriodicSlices(interval time.Duration, persister Persister) {
	go func() {
		ticker := time.NewTicker(interval)
		for range ticker.C {
			orders := persister.GetAllOrders()
			balances := persister.GetAllBalances()

			sm.DumpOrders(orders)
			sm.DumpBalances(balances)
			sm.MakeSlice()
		}
	}()
}
