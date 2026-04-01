package handler

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/matchengine/internal/order"
	"github.com/viabtc/go-project/services/matchengine/internal/server"
)

const (
	CMD_MARKET_STATUS       = 301
	CMD_MARKET_KLINE        = 302
	CMD_MARKET_DEALS        = 303
	CMD_MARKET_LAST         = 304
	CMD_MARKET_STATUS_TODAY = 305
	CMD_MARKET_USER_DEALS   = 306
	CMD_MARKET_LIST         = 307
	CMD_MARKET_SUMMARY      = 308
)

type MarketInfo struct {
	Name           string `json:"name"`
	Stock          string `json:"stock"`
	Money          string `json:"money"`
	FeePrec        int    `json:"fee_prec"`
	StockPrec      int    `json:"stock_prec"`
	MoneyPrec      int    `json:"money_prec"`
	MinAmount      string `json:"min_amount"`
	MaxAmount      string `json:"max_amount"`
	BuyMinAmount   string `json:"buy_min_amount"`
	BuyMaxAmount   string `json:"buy_max_amount"`
	SellMinAmount  string `json:"sell_min_amount"`
	SellMaxAmount  string `json:"sell_max_amount"`
	TradeBrief     bool   `json:"trade_brief,omitempty"`
	LastPrice      string `json:"last_price,omitempty"`
	LastAmount     string `json:"last_amount,omitempty"`
	BidPrice       string `json:"bid_price,omitempty"`
	AskPrice       string `json:"ask_price,omitempty"`
	BidAmount      string `json:"bid_amount,omitempty"`
	AskAmount      string `json:"ask_amount,omitempty"`
	OpenPrice      string `json:"open_price,omitempty"`
	ClosePrice     string `json:"close_price,omitempty"`
	HighPrice      string `json:"high_price,omitempty"`
	LowPrice       string `json:"low_price,omitempty"`
	Volume         string `json:"volume,omitempty"`
	Deal           string `json:"deal,omitempty"`
	TradeCount     int    `json:"trade_count,omitempty"`
	OrderCount     int    `json:"order_count,omitempty"`
	BuyOrderCount  int    `json:"buy_order_count,omitempty"`
	SellOrderCount int    `json:"sell_order_count,omitempty"`
}

type KlineInfo struct {
	Time   int64  `json:"time"`
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

type DealInfo struct {
	ID          int64           `json:"id"`
	Time        float64         `json:"time"`
	Amount      decimal.Decimal `json:"amount"`
	Price       decimal.Decimal `json:"price"`
	Side        order.Side      `json:"side"`
	BuyOrderID  int64           `json:"buy_order_id"`
	SellOrderID int64           `json:"sell_order_id"`
}

type MarketStatus struct {
	Market            string          `json:"market"`
	Mode              string          `json:"mode"`
	Seq               int64           `json:"seq"`
	OrderCount        int             `json:"order_count"`
	PendingOrderCount int             `json:"pending_order_count"`
	BidPrice          decimal.Decimal `json:"bid_price"`
	BidAmount         decimal.Decimal `json:"bid_amount"`
	BidCount          int             `json:"bid_count"`
	AskPrice          decimal.Decimal `json:"ask_price"`
	AskAmount         decimal.Decimal `json:"ask_amount"`
	AskCount          int             `json:"ask_count"`
}

func parseMarketName(market string) (stock, money string) {
	parts := strings.Split(market, "_")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return market, ""
}

func HandleMarketList(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	eng := s.GetEngine()
	markets := eng.ListMarkets()

	result := make([]MarketInfo, 0, len(markets))
	for _, name := range markets {
		stock, money := parseMarketName(name)
		result = append(result, MarketInfo{
			Name:      name,
			Stock:     stock,
			Money:     money,
			FeePrec:   8,
			StockPrec: 8,
			MoneyPrec: 8,
			MinAmount: "0.0001",
			MaxAmount: "1000000",
		})
	}

	return json.Marshal(result)
}

func HandleMarketStatus(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 1 {
		return nil, fmt.Errorf("invalid arguments: expected 1 param")
	}

	market, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	eng := s.GetEngine()
	ob, ok := eng.GetOrderBook(market)
	if !ok {
		return json.Marshal(MarketStatus{Market: market})
	}

	bestBid := ob.GetBestBid()
	bestAsk := ob.GetBestAsk()

	var bidPrice, bidAmount, askPrice, askAmount decimal.Decimal
	var bidCount, askCount int

	if bestBid != nil {
		bidPrice = bestBid.Price
	}
	if bestAsk != nil {
		askPrice = bestAsk.Price
	}

	orders := ob.GetOrders()
	pendingCount := 0
	for _, o := range orders {
		if o.Status == order.OrderStatusPending || o.Status == order.OrderStatusPartial {
			pendingCount++
			if o.Side == order.SideBuy {
				bidCount++
				bidAmount = bidAmount.Add(o.Amount.Sub(o.Deal))
			} else {
				askCount++
				askAmount = askAmount.Add(o.Amount.Sub(o.Deal))
			}
		}
	}

	status := MarketStatus{
		Market:            market,
		Mode:              "online",
		Seq:               int64(len(orders)),
		OrderCount:        len(orders),
		PendingOrderCount: pendingCount,
		BidPrice:          bidPrice,
		BidAmount:         bidAmount,
		BidCount:          bidCount,
		AskPrice:          askPrice,
		AskAmount:         askAmount,
		AskCount:          askCount,
	}

	return json.Marshal(status)
}

func HandleMarketStatusToday(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 1 {
		return nil, fmt.Errorf("invalid arguments: expected 1 param")
	}

	market, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	eng := s.GetEngine()
	ob, ok := eng.GetOrderBook(market)
	if !ok {
		return json.Marshal(MarketInfo{Name: market})
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var highPrice, lowPrice, openPrice, closePrice, volume, deal decimal.Decimal
	tradeCount := 0

	orders := ob.GetOrders()
	for _, o := range orders {
		if o.CreatedAt.Before(startOfDay) {
			continue
		}
		if o.Status == order.OrderStatusFilled {
			tradeCount++
			deal = deal.Add(o.Price.Mul(o.Deal))
			if highPrice.IsZero() || o.Price.GreaterThan(highPrice) {
				highPrice = o.Price
			}
			if lowPrice.IsZero() || o.Price.LessThan(lowPrice) {
				lowPrice = o.Price
			}
			volume = volume.Add(o.Deal)
			closePrice = o.Price
		}
	}

	bestBid := ob.GetBestBid()
	bestAsk := ob.GetBestAsk()

	info := MarketInfo{
		Name:       market,
		OpenPrice:  openPrice.String(),
		ClosePrice: closePrice.String(),
		HighPrice:  highPrice.String(),
		LowPrice:   lowPrice.String(),
		Volume:     volume.String(),
		Deal:       deal.String(),
		TradeCount: tradeCount,
	}

	if bestBid != nil {
		info.BidPrice = bestBid.Price.String()
	}
	if bestAsk != nil {
		info.AskPrice = bestAsk.Price.String()
	}

	return json.Marshal(info)
}

func HandleMarketKline(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 4 {
		return nil, fmt.Errorf("invalid arguments: expected 4 params")
	}

	market, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	startTimeVal, ok := params[1].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid start_time")
	}

	endTimeVal, ok := params[2].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid end_time")
	}

	interval, ok := params[3].(string)
	if !ok {
		return nil, fmt.Errorf("invalid interval")
	}

	_ = market
	_ = startTimeVal
	_ = endTimeVal
	_ = interval

	klines := []KlineInfo{}
	return json.Marshal(klines)
}

func HandleMarketDeals(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 3 {
		return nil, fmt.Errorf("invalid arguments: expected 3 params")
	}

	market, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	offset, ok := params[1].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid offset")
	}

	limit, ok := params[2].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid limit")
	}

	if limit > 100 {
		limit = 100
	}

	_ = market
	_ = int(offset)
	_ = int(limit)

	deals := []DealInfo{}
	result := map[string]interface{}{
		"offset":  int(offset),
		"limit":   int(limit),
		"total":   0,
		"records": deals,
	}

	return json.Marshal(result)
}

func HandleMarketLast(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 1 {
		return nil, fmt.Errorf("invalid arguments: expected 1 param")
	}

	market, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	eng := s.GetEngine()
	ob, ok := eng.GetOrderBook(market)
	if !ok {
		return json.Marshal(nil)
	}

	var lastPrice, lastAmount decimal.Decimal
	var lastTime float64

	orders := ob.GetOrders()
	for _, o := range orders {
		if o.Status == order.OrderStatusFilled && o.FinishedAt != nil {
			if lastTime == 0 || o.FinishedAt.Unix() > int64(lastTime) {
				lastTime = float64(o.FinishedAt.Unix())
				lastPrice = o.Price
				lastAmount = o.Deal
			}
		}
	}

	if lastTime == 0 {
		return json.Marshal(nil)
	}

	result := map[string]interface{}{
		"price":  lastPrice.String(),
		"amount": lastAmount.String(),
		"time":   lastTime,
	}

	return json.Marshal(result)
}

func HandleMarketUserDeals(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 4 {
		return nil, fmt.Errorf("invalid arguments: expected 4 params")
	}

	userID, ok := params[0].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user_id")
	}

	market, ok := params[1].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	offset, ok := params[2].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid offset")
	}

	limit, ok := params[3].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid limit")
	}

	if limit > 100 {
		limit = 100
	}

	eng := s.GetEngine()
	ob, ok := eng.GetOrderBook(market)
	if !ok {
		result := map[string]interface{}{
			"limit":   int(limit),
			"offset":  int(offset),
			"total":   0,
			"records": []DealInfo{},
		}
		return json.Marshal(result)
	}

	var userDeals []DealInfo
	orders := ob.GetOrders()
	for _, o := range orders {
		if o.UserID == int64(userID) && o.Status == order.OrderStatusFilled && o.FinishedAt != nil {
			userDeals = append(userDeals, DealInfo{
				ID:          o.ID,
				Time:        float64(o.FinishedAt.Unix()),
				Amount:      o.Deal,
				Price:       o.Price,
				Side:        o.Side,
				BuyOrderID:  0,
				SellOrderID: 0,
			})
		}
	}

	total := len(userDeals)
	start := int(offset)
	if start > total {
		start = total
	}
	end := start + int(limit)
	if end > total {
		end = total
	}

	result := map[string]interface{}{
		"limit":   int(limit),
		"offset":  int(offset),
		"total":   total,
		"records": userDeals[start:end],
	}

	return json.Marshal(result)
}

func HandleMarketSummary(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 1 {
		return nil, fmt.Errorf("invalid arguments: expected 1 param")
	}

	market, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	eng := s.GetEngine()
	ob, ok := eng.GetOrderBook(market)
	if !ok {
		return json.Marshal(MarketInfo{Name: market})
	}

	stock, money := parseMarketName(market)

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var highPrice, lowPrice, openPrice, closePrice, volume, deal decimal.Decimal
	var buyOrderCount, sellOrderCount int

	orders := ob.GetOrders()
	for _, o := range orders {
		if o.Status == order.OrderStatusPending || o.Status == order.OrderStatusPartial {
			if o.Side == order.SideBuy {
				buyOrderCount++
			} else {
				sellOrderCount++
			}
		}
		if o.CreatedAt.Before(startOfDay) {
			continue
		}
		if o.Status == order.OrderStatusFilled {
			deal = deal.Add(o.Price.Mul(o.Deal))
			if highPrice.IsZero() || o.Price.GreaterThan(highPrice) {
				highPrice = o.Price
			}
			if lowPrice.IsZero() || o.Price.LessThan(lowPrice) {
				lowPrice = o.Price
			}
			volume = volume.Add(o.Deal)
			closePrice = o.Price
		}
	}

	bestBid := ob.GetBestBid()
	bestAsk := ob.GetBestAsk()

	info := MarketInfo{
		Name:           market,
		Stock:          stock,
		Money:          money,
		FeePrec:        8,
		StockPrec:      8,
		MoneyPrec:      8,
		OpenPrice:      openPrice.String(),
		ClosePrice:     closePrice.String(),
		HighPrice:      highPrice.String(),
		LowPrice:       lowPrice.String(),
		Volume:         volume.String(),
		Deal:           deal.String(),
		BuyOrderCount:  buyOrderCount,
		SellOrderCount: sellOrderCount,
		OrderCount:     buyOrderCount + sellOrderCount,
	}

	if bestBid != nil {
		info.BidPrice = bestBid.Price.String()
		bidAmt := bestBid.Amount.Sub(bestBid.Deal)
		info.BidAmount = bidAmt.String()
	}

	if bestAsk != nil {
		info.AskPrice = bestAsk.Price.String()
		askAmt := bestAsk.Amount.Sub(bestAsk.Deal)
		info.AskAmount = askAmt.String()
	}

	return json.Marshal(info)
}

func RegisterMarketHandlers(s *server.RPCServer) {
	s.Handle(CMD_MARKET_STATUS, HandleMarketStatus)
	s.Handle(CMD_MARKET_KLINE, HandleMarketKline)
	s.Handle(CMD_MARKET_DEALS, HandleMarketDeals)
	s.Handle(CMD_MARKET_LAST, HandleMarketLast)
	s.Handle(CMD_MARKET_STATUS_TODAY, HandleMarketStatusToday)
	s.Handle(CMD_MARKET_USER_DEALS, HandleMarketUserDeals)
	s.Handle(CMD_MARKET_LIST, HandleMarketList)
	s.Handle(CMD_MARKET_SUMMARY, HandleMarketSummary)
}
