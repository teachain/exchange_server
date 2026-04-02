package engine

import (
	"container/heap"
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/matchengine/internal/order"
	"github.com/viabtc/go-project/services/matchengine/internal/persist"
)

var TakerFeeRate = decimal.NewFromFloat(0.001)
var MakerFeeRate = decimal.NewFromFloat(0.001)

const (
	OrderEventPut    = 1
	OrderEventUpdate = 2
	OrderEventFinish = 3
)

func (e *Engine) ProcessOrder(incoming *order.Order) ([]*Trade, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	market, ok := e.getMarketLocked(incoming.Market)
	if !ok {
		return nil, errors.New("market not found")
	}

	ob := e.GetOrCreateOrderBookWithLock(incoming.Market)

	if incoming.Side == order.SideBid {
		cost := incoming.Price.Mul(incoming.Amount)
		if err := e.balances.LockBalance(incoming.UserID, market.Money, cost); err != nil {
			return nil, err
		}
	} else {
		if err := e.balances.LockBalance(incoming.UserID, market.Stock, incoming.Amount); err != nil {
			return nil, err
		}
	}

	if e.producer != nil {
		e.producer.SendOrderEventAsync(OrderEventPut, incoming)
	}

	if e.operLogWriter != nil {
		logData := map[string]interface{}{
			"order_id":  incoming.ID,
			"user_id":   incoming.UserID,
			"market":    incoming.Market,
			"side":      incoming.Side,
			"type":      incoming.Type,
			"price":     incoming.Price.String(),
			"amount":    incoming.Amount.String(),
			"left":      incoming.Left.String(),
			"taker_fee": incoming.TakerFee.String(),
		}
		e.WriteOperLog(persist.OperLogTypeOrderCreate, logData)
	}

	trades, err := e.match(ob, incoming)

	if incoming.Status == order.OrderStatusFinished || incoming.Status == order.OrderStatusCanceled {
		e.AppendOrderHistory(incoming)
	}

	return trades, err
}

func (e *Engine) match(ob *order.OrderBook, incoming *order.Order) ([]*Trade, error) {
	var trades []*Trade

	if incoming.Side == order.SideBid {
		trades = e.matchAsTaker(incoming, ob.Asks, ob)
	} else {
		trades = e.matchAsTaker(incoming, ob.Bids, ob)
	}

	if incoming.Left.LessThan(incoming.Amount) && incoming.Left.IsPositive() {
		incoming.Status = order.OrderStatusPartial
		if incoming.Left.Equal(incoming.Amount) {
			incoming.Status = order.OrderStatusPending
		}
		ob.Add(incoming)
	} else {
		incoming.Status = order.OrderStatusFinished
		incoming.UpdateTime = time.Now()
	}

	return trades, nil
}

func (e *Engine) matchAsTaker(incoming *order.Order, makerQueue *order.PriceQueue, ob *order.OrderBook) []*Trade {
	var trades []*Trade

	isMarketOrder := incoming.Type == order.OrderTypeMarket

	for makerQueue.Len() > 0 && incoming.Left.IsPositive() {
		maker := makerQueue.Orders[0]

		if !isMarketOrder {
			if incoming.Side == order.SideBid && maker.Price.GreaterThan(incoming.Price) {
				break
			}
			if incoming.Side == order.SideAsk && maker.Price.LessThan(incoming.Price) {
				break
			}
		}

		remainingAmount := incoming.Left
		makerRemaining := maker.Left
		tradeAmount := decimal.Min(remainingAmount, makerRemaining)
		tradePrice := maker.Price

		trade := &Trade{
			ID:           e.idGenerator.NextID(),
			TakerOrderID: incoming.ID,
			MakerOrderID: maker.ID,
			TakerUserID:  incoming.UserID,
			MakerUserID:  maker.UserID,
			Market:       incoming.Market,
			Side:         incoming.Side,
			Price:        tradePrice,
			Amount:       tradeAmount,
			TakerFee:     tradeAmount.Mul(TakerFeeRate),
			MakerFee:     tradeAmount.Mul(MakerFeeRate),
			CreatedAt:    time.Now(),
		}
		trades = append(trades, trade)

		incoming.Left = incoming.Left.Sub(tradeAmount)
		maker.Left = maker.Left.Sub(tradeAmount)

		e.settleTrade(trade)

		if maker.Left.IsZero() || maker.Left.IsNegative() {
			heap.Pop(makerQueue)
			ob.Remove(maker.ID)
		}
	}

	return trades
}

func (e *Engine) settleTrade(trade *Trade) {
	market, ok := e.GetMarket(trade.Market)
	if !ok {
		return
	}

	if e.producer != nil {
		e.producer.SendDealEventAsync(trade)
	}

	e.AddTradeToOrder(trade)

	if trade.Side == order.SideBid {
		cost := trade.Price.Mul(trade.Amount)
		e.balances.DeductBalance(trade.TakerUserID, market.Money, cost)
		e.balances.AddBalance(trade.MakerUserID, market.Money, cost.Sub(trade.MakerFee))
		e.balances.UnlockBalance(trade.TakerUserID, market.Money, cost)

		if e.producer != nil {
			e.producer.SendBalanceUpdateAsync(trade.TakerUserID, market.Money, cost.Neg())
			e.producer.SendBalanceUpdateAsync(trade.MakerUserID, market.Money, cost.Sub(trade.MakerFee))
		}
	} else {
		e.balances.DeductBalance(trade.TakerUserID, market.Stock, trade.Amount)
		e.balances.AddBalance(trade.MakerUserID, market.Stock, trade.Amount.Sub(trade.MakerFee))
		e.balances.UnlockBalance(trade.TakerUserID, market.Stock, trade.Amount)

		if e.producer != nil {
			e.producer.SendBalanceUpdateAsync(trade.TakerUserID, market.Stock, trade.Amount.Neg())
			e.producer.SendBalanceUpdateAsync(trade.MakerUserID, market.Stock, trade.Amount.Sub(trade.MakerFee))
		}
	}
}

func (e *Engine) PutOrderNoLock(incoming *order.Order) ([]*Trade, error) {
	ob := e.getOrCreateOrderBookLocked(incoming.Market)

	trades, err := e.match(ob, incoming)
	if err != nil {
		return nil, err
	}

	if incoming.Left.IsPositive() && !incoming.IsFinished() {
		incoming.Status = order.OrderStatusPartial
		ob.Add(incoming)
	}

	return trades, nil
}

func (e *Engine) CancelOrder(orderID uint64, market string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ob, ok := e.orderBooks[market]
	if !ok {
		return nil
	}

	ord, ok := ob.GetOrder(orderID)
	if !ok {
		return nil
	}

	if ord.Status != order.OrderStatusPending && ord.Status != order.OrderStatusPartial {
		return nil
	}

	ob.Remove(orderID)

	marketConfig, ok := e.GetMarket(ord.Market)
	if !ok {
		return nil
	}

	remaining := ord.Left
	if ord.Side == order.SideBid {
		e.balances.UnlockBalance(ord.UserID, marketConfig.Money, ord.Price.Mul(remaining))
	} else {
		e.balances.UnlockBalance(ord.UserID, marketConfig.Stock, remaining)
	}

	ord.Status = order.OrderStatusCanceled
	ord.UpdateTime = time.Now()

	if e.operLogWriter != nil {
		logData := map[string]interface{}{
			"order_id": ord.ID,
			"user_id":  ord.UserID,
			"market":   ord.Market,
			"side":     ord.Side,
			"price":    ord.Price.String(),
			"amount":   ord.Amount.String(),
			"left":     ord.Left.String(),
		}
		e.WriteOperLog(persist.OperLogTypeOrderCancel, logData)
	}

	e.AppendOrderHistory(ord)

	return nil
}
