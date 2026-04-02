package engine

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/matchengine/internal/order"
)

type StopOrder struct {
	Order        order.Order
	TriggerPrice decimal.Decimal
	Triggered    bool
}

type StopManager struct {
	mu    sync.RWMutex
	stops map[string][]*StopOrder
}

func NewStopManager() *StopManager {
	return &StopManager{
		stops: make(map[string][]*StopOrder),
	}
}

func (sm *StopManager) AddStopOrder(stop *StopOrder) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	stop.Triggered = false
	stop.Order.Status = order.OrderStatusPending
	stop.Order.CreateTime = time.Now()
	stop.Order.UpdateTime = time.Now()

	sm.stops[stop.Order.Market] = append(sm.stops[stop.Order.Market], stop)
}

func (sm *StopManager) RemoveStopOrder(market string, orderID uint64) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	orders := sm.stops[market]
	for i, o := range orders {
		if o.Order.ID == orderID {
			sm.stops[market] = append(orders[:i], orders[i+1:]...)
			return true
		}
	}
	return false
}

func (sm *StopManager) CheckStopOrders(market string, lastPrice decimal.Decimal) []order.Order {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var triggered []order.Order

	orders := sm.stops[market]
	var remaining []*StopOrder

	for _, stop := range orders {
		if stop.Triggered {
			continue
		}

		shouldTrigger := false

		if stop.Order.Side == order.SideBid && lastPrice.LessThanOrEqual(stop.TriggerPrice) {
			shouldTrigger = true
		} else if stop.Order.Side == order.SideAsk && lastPrice.GreaterThanOrEqual(stop.TriggerPrice) {
			shouldTrigger = true
		}

		if shouldTrigger {
			stop.Triggered = true
			triggered = append(triggered, stop.Order)
		} else {
			remaining = append(remaining, stop)
		}
	}

	sm.stops[market] = remaining
	return triggered
}

func (sm *StopManager) GetStopOrders(market string) []*StopOrder {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.stops[market]
}

func (sm *StopManager) ClearMarket(market string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.stops, market)
}
