package kline

import (
	"sync"
	"time"

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

func (km *KlineManager) GetWeekKline(market string, ts int64, loc *time.Location) *Kline {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.current[market] == nil {
		return nil
	}

	if loc == nil {
		loc = time.UTC
	}

	t := time.Unix(ts, 0).In(loc)
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysFromMonday := int(weekday) - 1
	weekStart := time.Date(t.Year(), t.Month(), t.Day()-daysFromMonday, 0, 0, 0, 0, loc)
	weekStartTs := weekStart.Unix()
	weekEndTs := weekStartTs + 604800 - 1

	return km.getAggregatedKline(market, IntervalWeek, weekStartTs, weekEndTs)
}

func (km *KlineManager) GetMonthKline(market string, ts int64, loc *time.Location) *Kline {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.current[market] == nil {
		return nil
	}

	if loc == nil {
		loc = time.UTC
	}

	t := time.Unix(ts, 0).In(loc)
	monthStart := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	monthStartTs := monthStart.Unix()

	nextMonth := monthStart.AddDate(0, 1, 0)
	monthEnd := time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, loc).Add(-1)
	monthEndTs := monthEnd.Unix()

	return km.getAggregatedKline(market, IntervalMonth, monthStartTs, monthEndTs)
}

func (km *KlineManager) getAggregatedKline(market string, interval Interval, startTs, endTs int64) *Kline {
	var result Kline
	first := true

	for _, k := range km.current[market] {
		if k.Interval != Interval1d && k.Interval != Interval1m && k.Interval != Interval5m &&
			k.Interval != Interval15m && k.Interval != Interval30m && k.Interval != Interval1h &&
			k.Interval != Interval2h && k.Interval != Interval4h && k.Interval != Interval6h &&
			k.Interval != Interval12h && k.Interval != Interval1s {
			continue
		}

		if k.Timestamp < startTs || k.Timestamp > endTs {
			continue
		}

		if first {
			result = *k
			result.Interval = interval
			result.Timestamp = startTs
			first = false
		} else {
			if k.High.GreaterThan(result.High) {
				result.High = k.High
			}
			if k.Low.LessThan(result.Low) {
				result.Low = k.Low
			}
			result.Close = k.Close
			result.Volume = result.Volume.Add(k.Volume)
			result.DealAmount = result.DealAmount.Add(k.DealAmount)
		}
	}

	if first {
		return nil
	}
	return &result
}
