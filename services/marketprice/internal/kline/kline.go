package kline

import (
	"sync"

	"github.com/shopspring/decimal"
)

type Kline struct {
	Market     string          `json:"market"`
	Interval   Interval        `json:"interval"`
	Open       decimal.Decimal `json:"open"`
	High       decimal.Decimal `json:"high"`
	Low        decimal.Decimal `json:"low"`
	Close      decimal.Decimal `json:"close"`
	Volume     decimal.Decimal `json:"volume"`
	DealAmount decimal.Decimal `json:"deal_amount"`
	Timestamp  int64           `json:"timestamp"`
}

type KlineManager struct {
	mu      sync.RWMutex
	current map[string]map[Interval]*Kline
}

func NewKlineManager() *KlineManager {
	return &KlineManager{
		current: make(map[string]map[Interval]*Kline),
	}
}

func (km *KlineManager) AddDeal(market string, price, amount decimal.Decimal, ts int64) {
	km.mu.Lock()
	defer km.mu.Unlock()

	allIntervals := []Interval{
		Interval1s, Interval1m, Interval5m, Interval15m, Interval30m,
		Interval1h, Interval2h, Interval4h, Interval6h, Interval12h,
		Interval1d, Interval1w, Interval1M,
	}

	for _, interval := range allIntervals {
		tsBucket := interval.ToTimestamp(ts)

		if km.current[market] == nil {
			km.current[market] = make(map[Interval]*Kline)
		}

		if kline, ok := km.current[market][interval]; ok && kline.Timestamp == tsBucket {
			kline.High = decimal.Max(kline.High, price)
			kline.Low = decimal.Min(kline.Low, price)
			kline.Close = price
			kline.Volume = kline.Volume.Add(amount)
			kline.DealAmount = kline.DealAmount.Add(price.Mul(amount))
		} else {
			km.current[market][interval] = &Kline{
				Market:     market,
				Interval:   interval,
				Open:       price,
				High:       price,
				Low:        price,
				Close:      price,
				Volume:     amount,
				DealAmount: price.Mul(amount),
				Timestamp:  tsBucket,
			}
		}
	}
}

func (km *KlineManager) GetKline(market string, interval Interval, ts int64) *Kline {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.current[market] == nil {
		return nil
	}
	return km.current[market][interval]
}
