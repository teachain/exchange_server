package handler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/matchengine/internal/engine"
	"github.com/viabtc/go-project/services/matchengine/internal/order"
	"github.com/viabtc/go-project/services/matchengine/internal/server"
)

const (
	CMD_ORDER_PUT_LIMIT       = 201
	CMD_ORDER_PUT_MARKET      = 202
	CMD_ORDER_QUERY           = 203
	CMD_ORDER_CANCEL          = 204
	CMD_ORDER_BOOK            = 205
	CMD_ORDER_BOOK_DEPTH      = 206
	CMD_ORDER_DETAIL          = 207
	CMD_ORDER_HISTORY         = 208
	CMD_ORDER_DEALS           = 209
	CMD_ORDER_DETAIL_FINISHED = 210
)

type OrderInfo struct {
	ID         int64             `json:"id"`
	Market     string            `json:"market"`
	Side       order.Side        `json:"side"`
	Type       int               `json:"type"`
	UserID     int64             `json:"user"`
	Price      decimal.Decimal   `json:"price"`
	Amount     decimal.Decimal   `json:"amount"`
	TakerFee   decimal.Decimal   `json:"taker_fee"`
	MakerFee   decimal.Decimal   `json:"maker_fee"`
	Left       decimal.Decimal   `json:"left"`
	DealStock  decimal.Decimal   `json:"deal_stock"`
	DealMoney  decimal.Decimal   `json:"deal_money"`
	DealFee    decimal.Decimal   `json:"deal_fee"`
	Status     order.OrderStatus `json:"status"`
	CreateTime float64           `json:"ctime"`
	UpdateTime float64           `json:"mtime"`
	Source     string            `json:"source,omitempty"`
}

type TradeInfo struct {
	ID           int64           `json:"id"`
	TakerOrderID int64           `json:"taker_order_id"`
	MakerOrderID int64           `json:"maker_order_id"`
	TakerUserID  int64           `json:"taker_user_id"`
	MakerUserID  int64           `json:"maker_user_id"`
	Market       string          `json:"market"`
	Side         order.Side      `json:"side"`
	Price        decimal.Decimal `json:"price"`
	Amount       decimal.Decimal `json:"amount"`
	TakerFee     decimal.Decimal `json:"taker_fee"`
	MakerFee     decimal.Decimal `json:"maker_fee"`
	CreateTime   float64         `json:"ctime"`
}

type DepthLevel struct {
	Price  string `json:"price"`
	Amount string `json:"amount"`
}

func orderToInfo(o *order.Order) OrderInfo {
	left := o.Amount.Sub(o.Deal)
	return OrderInfo{
		ID:         o.ID,
		Market:     o.Market,
		Side:       o.Side,
		Type:       1,
		UserID:     o.UserID,
		Price:      o.Price,
		Amount:     o.Amount,
		TakerFee:   o.Fee,
		MakerFee:   decimal.Zero,
		Left:       left,
		DealStock:  o.Deal,
		DealMoney:  o.Price.Mul(o.Deal),
		DealFee:    o.Fee,
		Status:     o.Status,
		CreateTime: float64(o.CreatedAt.Unix()),
		UpdateTime: float64(time.Now().Unix()),
	}
}

func tradeToInfo(t *engine.Trade) TradeInfo {
	return TradeInfo{
		ID:           t.ID,
		TakerOrderID: t.TakerOrderID,
		MakerOrderID: t.MakerOrderID,
		TakerUserID:  t.TakerUserID,
		MakerUserID:  t.MakerUserID,
		Market:       t.Market,
		Side:         t.Side,
		Price:        t.Price,
		Amount:       t.Amount,
		TakerFee:     t.TakerFee,
		MakerFee:     t.MakerFee,
		CreateTime:   float64(t.CreatedAt.Unix()),
	}
}

func HandleOrderPutLimit(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 8 {
		return nil, fmt.Errorf("invalid arguments: expected 8 params")
	}

	userID, ok := params[0].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user_id")
	}

	market, ok := params[1].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	sideVal, ok := params[2].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid side")
	}
	side := order.Side(int(sideVal))
	if side != order.SideBuy && side != order.SideSell {
		return nil, fmt.Errorf("invalid side value")
	}

	amountStr, ok := params[3].(string)
	if !ok {
		return nil, fmt.Errorf("invalid amount")
	}
	amount, err := decimal.NewFromString(amountStr)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("invalid amount")
	}

	priceStr, ok := params[4].(string)
	if !ok {
		return nil, fmt.Errorf("invalid price")
	}
	price, err := decimal.NewFromString(priceStr)
	if err != nil || price.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("invalid price")
	}

	takerFeeStr, ok := params[5].(string)
	if !ok {
		return nil, fmt.Errorf("invalid taker_fee")
	}
	takerFee, err := decimal.NewFromString(takerFeeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid taker_fee")
	}

	_, ok = params[6].(string)
	if !ok {
		return nil, fmt.Errorf("invalid maker_fee")
	}

	source, ok := params[7].(string)
	if !ok {
		source = ""
	}
	_ = source

	eng := s.GetEngine()

	if side == order.SideBuy {
		frozen := price.Mul(amount)
		err = eng.GetBalances().LockBalance(int64(userID), market, frozen)
		if err != nil {
			return nil, fmt.Errorf("balance not enough")
		}
	} else {
		err = eng.GetBalances().LockBalance(int64(userID), market, amount)
		if err != nil {
			return nil, fmt.Errorf("balance not enough")
		}
	}

	incoming := &order.Order{
		ID:        eng.NextID(),
		UserID:    int64(userID),
		Market:    market,
		Side:      side,
		Price:     price,
		Amount:    amount,
		Deal:      decimal.Zero,
		Fee:       takerFee,
		Status:    order.OrderStatusPending,
		CreatedAt: time.Now(),
	}

	trades, err := eng.ProcessOrder(incoming)
	if err != nil {
		if side == order.SideBuy {
			frozen := price.Mul(amount)
			eng.GetBalances().UnlockBalance(int64(userID), market, frozen)
		} else {
			eng.GetBalances().UnlockBalance(int64(userID), market, amount)
		}
		return nil, fmt.Errorf("process order failed")
	}

	if incoming.Deal.IsZero() {
	} else {
		if side == order.SideBuy {
			frozen := price.Mul(amount)
			spent := price.Mul(incoming.Deal)
			eng.GetBalances().UnlockBalance(int64(userID), market, frozen.Sub(spent))
		} else {
			spent := incoming.Deal
			eng.GetBalances().UnlockBalance(int64(userID), market, amount.Sub(spent))
		}
	}

	result := map[string]interface{}{
		"order":  orderToInfo(incoming),
		"trades": tradesToInfos(trades),
	}

	return json.Marshal(result)
}

func HandleOrderPutMarket(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 6 {
		return nil, fmt.Errorf("invalid arguments: expected 6 params")
	}

	userID, ok := params[0].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user_id")
	}

	market, ok := params[1].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	sideVal, ok := params[2].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid side")
	}
	side := order.Side(int(sideVal))
	if side != order.SideBuy && side != order.SideSell {
		return nil, fmt.Errorf("invalid side value")
	}

	amountStr, ok := params[3].(string)
	if !ok {
		return nil, fmt.Errorf("invalid amount")
	}
	amount, err := decimal.NewFromString(amountStr)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("invalid amount")
	}

	takerFeeStr, ok := params[4].(string)
	if !ok {
		return nil, fmt.Errorf("invalid taker_fee")
	}
	takerFee, err := decimal.NewFromString(takerFeeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid taker_fee")
	}

	_, ok = params[5].(string)

	eng := s.GetEngine()

	ob, ok := eng.GetOrderBook(market)
	if !ok {
		return nil, fmt.Errorf("market not found")
	}

	var marketPrice decimal.Decimal
	if side == order.SideBuy && ob.Asks.Len() > 0 {
		marketPrice = ob.Asks.Orders[0].Price
	} else if side == order.SideSell && ob.Bids.Len() > 0 {
		marketPrice = ob.Bids.Orders[0].Price
	} else {
		return nil, fmt.Errorf("no enough trader")
	}

	if side == order.SideBuy {
		frozen := marketPrice.Mul(amount)
		err = eng.GetBalances().LockBalance(int64(userID), market, frozen)
		if err != nil {
			return nil, fmt.Errorf("balance not enough")
		}
	} else {
		err = eng.GetBalances().LockBalance(int64(userID), market, amount)
		if err != nil {
			return nil, fmt.Errorf("balance not enough")
		}
	}

	incoming := &order.Order{
		ID:        eng.NextID(),
		UserID:    int64(userID),
		Market:    market,
		Side:      side,
		Price:     marketPrice,
		Amount:    amount,
		Deal:      decimal.Zero,
		Fee:       takerFee,
		Status:    order.OrderStatusPending,
		CreatedAt: time.Now(),
	}

	trades, err := eng.ProcessOrder(incoming)
	if err != nil {
		if side == order.SideBuy {
			frozen := marketPrice.Mul(amount)
			eng.GetBalances().UnlockBalance(int64(userID), market, frozen)
		} else {
			eng.GetBalances().UnlockBalance(int64(userID), market, amount)
		}
		return nil, fmt.Errorf("process order failed")
	}

	if side == order.SideBuy {
		frozen := marketPrice.Mul(amount)
		spent := marketPrice.Mul(incoming.Deal)
		eng.GetBalances().UnlockBalance(int64(userID), market, frozen.Sub(spent))
	} else {
		spent := incoming.Deal
		eng.GetBalances().UnlockBalance(int64(userID), market, amount.Sub(spent))
	}

	result := map[string]interface{}{
		"order":  orderToInfo(incoming),
		"trades": tradesToInfos(trades),
	}

	return json.Marshal(result)
}

func HandleOrderQuery(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
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
			"records": []OrderInfo{},
		}
		return json.Marshal(result)
	}

	orders := ob.GetOrders()
	var userOrders []*order.Order
	for _, o := range orders {
		if o.UserID == int64(userID) && o.Status != order.OrderStatusCancelled {
			userOrders = append(userOrders, o)
		}
	}

	total := len(userOrders)
	start := int(offset)
	if start > total {
		start = total
	}
	end := start + int(limit)
	if end > total {
		end = total
	}

	records := make([]OrderInfo, 0)
	for i := start; i < end; i++ {
		records = append(records, orderToInfo(userOrders[i]))
	}

	result := map[string]interface{}{
		"limit":   int(limit),
		"offset":  int(offset),
		"total":   total,
		"records": records,
	}

	return json.Marshal(result)
}

func HandleOrderCancel(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 3 {
		return nil, fmt.Errorf("invalid arguments: expected 3 params")
	}

	userID, ok := params[0].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user_id")
	}

	market, ok := params[1].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	orderID, ok := params[2].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid order_id")
	}

	eng := s.GetEngine()
	ord, found := eng.GetOrder(int64(orderID))
	if !found {
		return nil, fmt.Errorf("order not found")
	}

	if ord.UserID != int64(userID) {
		return nil, fmt.Errorf("user not match")
	}

	err := eng.CancelOrder(int64(orderID), market)
	if err != nil {
		return nil, fmt.Errorf("cancel order failed")
	}

	updated, found := eng.GetOrder(int64(orderID))
	if !found {
		return nil, fmt.Errorf("order not found after cancel")
	}

	result := map[string]interface{}{
		"order": orderToInfo(updated),
	}

	return json.Marshal(result)
}

func HandleOrderBook(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
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

	sideVal, ok := params[1].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid side")
	}
	side := order.Side(int(sideVal))
	if side != order.SideBuy && side != order.SideSell {
		return nil, fmt.Errorf("invalid side value")
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
			"offset": int(offset),
			"limit":  int(limit),
			"total":  0,
			"orders": []OrderInfo{},
		}
		return json.Marshal(result)
	}

	var orders []*order.Order
	var total int
	if side == order.SideBuy {
		orders = ob.Bids.Orders
		total = len(orders)
	} else {
		orders = ob.Asks.Orders
		total = len(orders)
	}

	start := int(offset)
	if start > total {
		start = total
	}
	end := start + int(limit)
	if end > total {
		end = total
	}

	records := make([]OrderInfo, 0)
	for i := start; i < end; i++ {
		records = append(records, orderToInfo(orders[i]))
	}

	result := map[string]interface{}{
		"offset": int(offset),
		"limit":  int(limit),
		"total":  total,
		"orders": records,
	}

	return json.Marshal(result)
}

func HandleOrderBookDepth(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
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

	limit, ok := params[1].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid limit")
	}
	if limit > 100 {
		limit = 100
	}

	intervalStr, ok := params[2].(string)
	if !ok {
		return nil, fmt.Errorf("invalid interval")
	}
	interval, err := decimal.NewFromString(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid interval")
	}

	eng := s.GetEngine()
	ob, ok := eng.GetOrderBook(market)
	if !ok {
		result := map[string]interface{}{
			"asks": []DepthLevel{},
			"bids": []DepthLevel{},
		}
		return json.Marshal(result)
	}

	asks := getDepthWithInterval(ob.Asks.Orders, int(limit), interval)
	bids := getDepthWithInterval(ob.Bids.Orders, int(limit), interval)

	result := map[string]interface{}{
		"asks": asks,
		"bids": bids,
	}

	return json.Marshal(result)
}

func getDepthWithInterval(orders []*order.Order, limit int, interval decimal.Decimal) []DepthLevel {
	if len(orders) == 0 {
		return []DepthLevel{}
	}

	levels := make(map[string]decimal.Decimal)
	for _, o := range orders {
		var price decimal.Decimal
		if interval.IsZero() {
			price = o.Price
		} else {
			q := o.Price.Div(interval).Floor()
			r := o.Price.Mod(interval)
			if !r.IsZero() {
				price = q.Add(decimal.NewFromInt(1)).Mul(interval)
			} else {
				price = q.Mul(interval)
			}
		}
		priceStr := price.String()
		left := o.Amount.Sub(o.Deal)
		levels[priceStr] = levels[priceStr].Add(left)
	}

	result := make([]DepthLevel, 0, len(levels))
	for priceStr, amount := range levels {
		price, _ := decimal.NewFromString(priceStr)
		result = append(result, DepthLevel{
			Price:  price.String(),
			Amount: amount.String(),
		})
	}

	return result
}

func HandleOrderDetail(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 2 {
		return nil, fmt.Errorf("invalid arguments: expected 2 params")
	}

	market, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	orderID, ok := params[1].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid order_id")
	}

	eng := s.GetEngine()
	ord, found := eng.GetOrder(int64(orderID))
	if !found || ord.Market != market {
		return json.Marshal(nil)
	}

	return json.Marshal(orderToInfo(ord))
}

func HandleOrderHistory(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
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
			"records": []OrderInfo{},
		}
		return json.Marshal(result)
	}

	orders := ob.GetOrders()
	var finishedOrders []*order.Order
	for _, o := range orders {
		if o.UserID == int64(userID) && (o.Status == order.OrderStatusFilled || o.Status == order.OrderStatusCancelled) {
			finishedOrders = append(finishedOrders, o)
		}
	}

	total := len(finishedOrders)
	start := int(offset)
	if start > total {
		start = total
	}
	end := start + int(limit)
	if end > total {
		end = total
	}

	records := make([]OrderInfo, 0)
	for i := start; i < end; i++ {
		records = append(records, orderToInfo(finishedOrders[i]))
	}

	result := map[string]interface{}{
		"limit":   int(limit),
		"offset":  int(offset),
		"total":   total,
		"records": records,
	}

	return json.Marshal(result)
}

func HandleOrderDeals(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
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
	_, ok = eng.GetOrderBook(market)
	if !ok {
		result := map[string]interface{}{
			"limit":   int(limit),
			"offset":  int(offset),
			"total":   0,
			"records": []TradeInfo{},
		}
		return json.Marshal(result)
	}

	_ = userID

	result := map[string]interface{}{
		"limit":   int(limit),
		"offset":  int(offset),
		"total":   0,
		"records": []TradeInfo{},
	}

	return json.Marshal(result)
}

func HandleOrderDetailFinished(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 2 {
		return nil, fmt.Errorf("invalid arguments: expected 2 params")
	}

	market, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid market")
	}

	orderID, ok := params[1].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid order_id")
	}

	eng := s.GetEngine()
	ord, found := eng.GetOrder(int64(orderID))
	if !found || ord.Market != market {
		return json.Marshal(nil)
	}

	if ord.Status != order.OrderStatusFilled && ord.Status != order.OrderStatusCancelled {
		return json.Marshal(nil)
	}

	return json.Marshal(orderToInfo(ord))
}

func tradesToInfos(trades []*engine.Trade) []TradeInfo {
	result := make([]TradeInfo, 0, len(trades))
	for _, t := range trades {
		result = append(result, tradeToInfo(t))
	}
	return result
}

func RegisterOrderHandlers(s *server.RPCServer) {
	s.Handle(CMD_ORDER_PUT_LIMIT, HandleOrderPutLimit)
	s.Handle(CMD_ORDER_PUT_MARKET, HandleOrderPutMarket)
	s.Handle(CMD_ORDER_QUERY, HandleOrderQuery)
	s.Handle(CMD_ORDER_CANCEL, HandleOrderCancel)
	s.Handle(CMD_ORDER_BOOK, HandleOrderBook)
	s.Handle(CMD_ORDER_BOOK_DEPTH, HandleOrderBookDepth)
	s.Handle(CMD_ORDER_DETAIL, HandleOrderDetail)
	s.Handle(CMD_ORDER_HISTORY, HandleOrderHistory)
	s.Handle(CMD_ORDER_DEALS, HandleOrderDeals)
	s.Handle(CMD_ORDER_DETAIL_FINISHED, HandleOrderDetailFinished)
}
