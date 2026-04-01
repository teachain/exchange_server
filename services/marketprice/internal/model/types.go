package model

import (
	"github.com/shopspring/decimal"
)

type MarketInfo struct {
	Name       string
	LastPrice  decimal.Decimal
	SecKlines  map[int64]*KlineInfo
	MinKlines  map[int64]*KlineInfo
	HourKlines map[int64]*KlineInfo
	DayKlines  map[int64]*KlineInfo
	Deals      []*Deal
	UpdateMap  map[UpdateKey]bool
	UpdateTime float64
}

type KlineInfo struct {
	Open   decimal.Decimal
	Close  decimal.Decimal
	High   decimal.Decimal
	Low    decimal.Decimal
	Volume decimal.Decimal
	Deal   decimal.Decimal
}

type UpdateKey struct {
	KlineType int // 0=sec, 1=min, 2=hour, 3=day
	Timestamp int64
}

type Deal struct {
	ID     int64           `json:"id"`
	Time   float64         `json:"time"`
	Price  decimal.Decimal `json:"price"`
	Amount decimal.Decimal `json:"amount"`
	Side   int             `json:"side"` // 1=ask, 2=bid
	Type   string          `json:"type"` // "buy" or "sell"
}

type MarketStatus struct {
	Period int64           `json:"period"`
	Last   decimal.Decimal `json:"last"`
	Open   decimal.Decimal `json:"open"`
	Close  decimal.Decimal `json:"close"`
	High   decimal.Decimal `json:"high"`
	Low    decimal.Decimal `json:"low"`
	Volume decimal.Decimal `json:"volume"`
	Deal   decimal.Decimal `json:"deal"`
}
