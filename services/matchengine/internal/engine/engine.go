package engine

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
	"github.com/viabtc/go-project/services/matchengine/internal/balance"
	"github.com/viabtc/go-project/services/matchengine/internal/model"
	"github.com/viabtc/go-project/services/matchengine/internal/order"
)

type Trade struct {
	ID           uint64          `json:"id"`
	TakerOrderID uint64          `json:"taker_order_id"`
	MakerOrderID uint64          `json:"maker_order_id"`
	TakerUserID  uint32          `json:"taker_user_id"`
	MakerUserID  uint32          `json:"maker_user_id"`
	Market       string          `json:"market"`
	Side         order.Side      `json:"side"`
	Price        decimal.Decimal `json:"price"`
	Amount       decimal.Decimal `json:"amount"`
	TakerFee     decimal.Decimal `json:"taker_fee"`
	MakerFee     decimal.Decimal `json:"maker_fee"`
	CreatedAt    time.Time       `json:"created_at"`
}

type Producer interface {
	SendOrderEvent(event int, order *order.Order) error
	SendDealEvent(trade *Trade) error
	SendBalanceUpdate(userID uint32, asset string, change decimal.Decimal) error
	SendOrderEventAsync(event int, order *order.Order)
	SendDealEventAsync(trade *Trade)
	SendBalanceUpdateAsync(userID uint32, asset string, change decimal.Decimal)
}

type Engine struct {
	mu          sync.RWMutex
	orderBooks  map[string]*order.OrderBook
	balances    *balance.BalanceManager
	tradeCh     chan *Trade
	orderCh     chan *order.Order
	idGenerator *IDGenerator
	markets     map[string]*model.MarketConfig
	assets      map[string]*model.AssetConfig
	producer    Producer
	orderTrades map[uint64][]*Trade
}

func NewEngine() *Engine {
	e := &Engine{
		orderBooks:  make(map[string]*order.OrderBook),
		balances:    balance.NewBalanceManager(),
		tradeCh:     make(chan *Trade, 10000),
		orderCh:     make(chan *order.Order, 10000),
		idGenerator: NewIDGenerator(),
		markets:     make(map[string]*model.MarketConfig),
		assets:      make(map[string]*model.AssetConfig),
		orderTrades: make(map[uint64][]*Trade),
	}
	e.LoadConfig()
	return e
}

func (e *Engine) LoadConfig() {
	var cfg model.Config

	if err := viper.UnmarshalKey("markets", &cfg.Markets); err != nil {
		println("markets unmarshal error:", err.Error())
	}
	if err := viper.UnmarshalKey("assets", &cfg.Assets); err != nil {
		println("assets unmarshal error:", err.Error())
	}

	for _, m := range cfg.Markets {
		e.markets[m.Name] = m
	}
	for _, a := range cfg.Assets {
		e.assets[a.Name] = a
	}
}

func (e *Engine) GetMarket(name string) (*model.MarketConfig, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.getMarketLocked(name)
}

func (e *Engine) getMarketLocked(name string) (*model.MarketConfig, bool) {
	m, ok := e.markets[name]
	return m, ok
}

func (e *Engine) GetAsset(name string) (*model.AssetConfig, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	a, ok := e.assets[name]
	return a, ok
}

func (e *Engine) GetOrCreateOrderBook(market string) *order.OrderBook {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.getOrCreateOrderBookLocked(market)
}

func (e *Engine) GetOrCreateOrderBookWithLock(market string) *order.OrderBook {
	ob, ok := e.orderBooks[market]
	if !ok {
		ob = order.NewOrderBook()
		e.orderBooks[market] = ob
	}
	return ob
}

func (e *Engine) getOrCreateOrderBookLocked(market string) *order.OrderBook {
	ob, ok := e.orderBooks[market]
	if !ok {
		ob = order.NewOrderBook()
		e.orderBooks[market] = ob
	}
	return ob
}

func (e *Engine) GetOrderBook(market string) (*order.OrderBook, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ob, ok := e.orderBooks[market]
	return ob, ok
}

func (e *Engine) GetOrder(orderID uint64) (*order.Order, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, ob := range e.orderBooks {
		if ord, ok := ob.GetOrder(orderID); ok {
			return ord, true
		}
	}
	return nil, false
}

func (e *Engine) NextID() uint64 {
	return e.idGenerator.NextID()
}

func (e *Engine) GetBalance(userID uint32, asset string) (decimal.Decimal, decimal.Decimal) {
	return e.balances.GetBalance(userID, asset)
}

func (e *Engine) SetBalance(userID uint32, asset string, balance, frozen decimal.Decimal) {
	e.balances.SetBalance(userID, asset, balance, frozen)
}

func (e *Engine) GetAllBalancesForUser(userID uint32) map[string]*balance.Balance {
	return e.balances.GetAllBalancesForUser(userID)
}

func (e *Engine) GetAllBalancesForAsset(asset string) []*balance.Balance {
	return e.balances.GetAllBalancesForAsset(asset)
}

func (e *Engine) ListAssets() []string {
	return e.balances.ListAssets()
}

func (e *Engine) ListMarkets() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	markets := make([]string, 0, len(e.orderBooks))
	for market := range e.orderBooks {
		markets = append(markets, market)
	}
	return markets
}

func (e *Engine) GetAllAssets() []*model.AssetConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	assets := make([]*model.AssetConfig, 0, len(e.assets))
	for _, a := range e.assets {
		assets = append(assets, a)
	}
	return assets
}

func (e *Engine) GetBalances() *balance.BalanceManager {
	return e.balances
}

func (e *Engine) TradeChan() <-chan *Trade {
	return e.tradeCh
}

func (e *Engine) OrderChan() <-chan *order.Order {
	return e.orderCh
}

func (e *Engine) SetProducer(p Producer) {
	e.producer = p
}

func (e *Engine) AddTradeToOrder(trade *Trade) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.orderTrades[trade.TakerOrderID] = append(e.orderTrades[trade.TakerOrderID], trade)
	e.orderTrades[trade.MakerOrderID] = append(e.orderTrades[trade.MakerOrderID], trade)
}

func (e *Engine) GetOrderTrades(orderID uint64, offset, limit int) ([]*Trade, int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	trades := e.orderTrades[orderID]
	total := len(trades)

	if offset >= total {
		return []*Trade{}, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return trades[offset:end], total
}
