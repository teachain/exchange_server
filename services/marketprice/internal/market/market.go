package market

import (
	"sync"

	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/marketprice/internal/model"
)

const (
	KlineTypeSec  = 0
	KlineTypeMin  = 1
	KlineTypeHour = 2
	KlineTypeDay  = 3
)

type Manager struct {
	markets map[string]*model.MarketInfo
	mu      sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		markets: make(map[string]*model.MarketInfo),
	}
}

func (m *Manager) GetOrCreate(marketName string) *model.MarketInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.markets[marketName]
	if !ok {
		info = &model.MarketInfo{
			Name:       marketName,
			LastPrice:  decimal.Zero,
			SecKlines:  make(map[int64]*model.KlineInfo),
			MinKlines:  make(map[int64]*model.KlineInfo),
			HourKlines: make(map[int64]*model.KlineInfo),
			DayKlines:  make(map[int64]*model.KlineInfo),
			Deals:      make([]*model.Deal, 0),
			UpdateMap:  make(map[model.UpdateKey]bool),
		}
		m.markets[marketName] = info
	}
	return info
}

func (m *Manager) Get(marketName string) (*model.MarketInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.markets[marketName]
	return info, ok
}

func (m *Manager) UpdateKline(info *model.MarketInfo, klineType int, timestamp int64, price, amount decimal.Decimal) {
	var klines map[int64]*model.KlineInfo
	var interval int64

	switch klineType {
	case KlineTypeSec:
		klines = info.SecKlines
		interval = 1
	case KlineTypeMin:
		klines = info.MinKlines
		interval = 60
	case KlineTypeHour:
		klines = info.HourKlines
		interval = 3600
	case KlineTypeDay:
		klines = info.DayKlines
		interval = 86400
	}

	tsKey := timestamp / interval * interval

	kline, ok := klines[tsKey]
	if !ok {
		kline = &model.KlineInfo{
			Open:   price,
			Close:  price,
			High:   price,
			Low:    price,
			Volume: decimal.Zero,
			Deal:   decimal.Zero,
		}
		klines[tsKey] = kline
	}

	kline.Close = price
	if price.GreaterThan(kline.High) {
		kline.High = price
	}
	if price.LessThan(kline.Low) {
		kline.Low = price
	}
	kline.Volume = kline.Volume.Add(amount)

	info.UpdateMap[model.UpdateKey{KlineType: klineType, Timestamp: tsKey}] = true
}

func (m *Manager) AddDeal(info *model.MarketInfo, deal *model.Deal) {
	info.Deals = append(info.Deals, deal)
	const MaxDeals = 100
	if len(info.Deals) > MaxDeals {
		info.Deals = info.Deals[len(info.Deals)-MaxDeals:]
	}
	info.LastPrice = deal.Price
	info.UpdateTime = deal.Time
}

func (m *Manager) ListMarkets() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	markets := make([]string, 0, len(m.markets))
	for name := range m.markets {
		markets = append(markets, name)
	}
	return markets
}
