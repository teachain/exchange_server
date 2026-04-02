package order

import (
	"container/heap"
	"sync"

	"github.com/shopspring/decimal"
)

type OrderBook struct {
	mu     sync.RWMutex
	Bids   *PriceQueue
	Asks   *PriceQueue
	orders map[uint64]*Order
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids:   NewPriceQueue(),
		Asks:   NewPriceQueue(),
		orders: make(map[uint64]*Order),
	}
}

func (ob *OrderBook) Add(order *Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.orders[order.ID] = order

	pq := ob.Bids
	if order.Side == SideAsk {
		pq = ob.Asks
	}

	heap.Push(pq, order)
}

func (ob *OrderBook) Remove(orderID uint64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	delete(ob.orders, orderID)
}

func (ob *OrderBook) GetOrder(orderID uint64) (*Order, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	order, ok := ob.orders[orderID]
	return order, ok
}

func (ob *OrderBook) GetBestBid() *Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if ob.Bids.Len() == 0 {
		return nil
	}
	return ob.Bids.Orders[0]
}

func (ob *OrderBook) GetBestAsk() *Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if ob.Asks.Len() == 0 {
		return nil
	}
	return ob.Asks.Orders[0]
}

func (ob *OrderBook) GetOrders() []*Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	orders := make([]*Order, 0, len(ob.orders))
	for _, order := range ob.orders {
		orders = append(orders, order)
	}
	return orders
}

type DepthLevel struct {
	Price  decimal.Decimal
	Amount decimal.Decimal
}

func (ob *OrderBook) GetDepth(limit int, side Side) []DepthLevel {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	pq := ob.Bids
	if side == SideAsk {
		pq = ob.Asks
	}

	levels := make(map[string]decimal.Decimal)
	for _, o := range pq.Orders {
		priceStr := o.Price.String()
		levels[priceStr] = levels[priceStr].Add(o.Left)
	}

	result := make([]DepthLevel, 0, len(levels))
	for priceStr, amount := range levels {
		price, _ := decimal.NewFromString(priceStr)
		result = append(result, DepthLevel{Price: price, Amount: amount})
	}

	return result
}
