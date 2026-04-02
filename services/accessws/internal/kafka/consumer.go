package kafka

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viabtc/go-project/services/accessws/internal/handler"
	"github.com/viabtc/go-project/services/accessws/internal/model"
	"github.com/viabtc/go-project/services/accessws/internal/rpc"
	"github.com/viabtc/go-project/services/accessws/internal/subscription"
)

type Consumer struct {
	brokers       []string
	group         string
	subMgr        *subscription.Manager
	rpcClient     *rpc.RPCCLient
	ordersTopic   string
	balancesTopic string
	notifyCh      chan *Notification
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

type Notification struct {
	Method string
	Params interface{}
	UserID uint32
	Asset  string
	Market string
	Sess   *model.ClientSession
}

func NewConsumer(brokers []string, group, ordersTopic, balancesTopic string, subMgr *subscription.Manager, rpcClient *rpc.RPCCLient) *Consumer {
	return &Consumer{
		brokers:       brokers,
		group:         group,
		subMgr:        subMgr,
		rpcClient:     rpcClient,
		ordersTopic:   ordersTopic,
		balancesTopic: balancesTopic,
		notifyCh:      make(chan *Notification, 10000),
		stopCh:        make(chan struct{}),
	}
}

func (c *Consumer) Start() error {
	c.wg.Add(2)

	go func() {
		defer c.wg.Done()
		c.consumeOrders()
	}()

	go func() {
		defer c.wg.Done()
		c.consumeBalances()
	}()

	go c.processNotifications()

	return nil
}

func (c *Consumer) consumeOrders() {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        c.brokers,
		Topic:          c.ordersTopic,
		GroupID:        c.group + "_orders",
		MinBytes:       10e3,
		MaxBytes:       10e6,
		MaxWait:        1 * time.Second,
		StartOffset:    kafka.LastOffset,
		CommitInterval: time.Second,
	})
	defer reader.Close()

	for {
		select {
		case <-c.stopCh:
			return
		default:
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return
				}
				log.Printf("failed to read orders message: %v", err)
				continue
			}
			c.processOrdersMessage(msg.Value)
		}
	}
}

func (c *Consumer) consumeBalances() {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        c.brokers,
		Topic:          c.balancesTopic,
		GroupID:        c.group + "_balances",
		MinBytes:       10e3,
		MaxBytes:       10e6,
		MaxWait:        1 * time.Second,
		StartOffset:    kafka.LastOffset,
		CommitInterval: time.Second,
	})
	defer reader.Close()

	for {
		select {
		case <-c.stopCh:
			return
		default:
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return
				}
				log.Printf("failed to read balances message: %v", err)
				continue
			}
			c.processBalancesMessage(msg.Value)
		}
	}
}

func (c *Consumer) processNotifications() {
	for {
		select {
		case notif := <-c.notifyCh:
			c.sendNotification(notif)
		case <-c.stopCh:
			return
		}
	}
}

func (c *Consumer) sendNotification(notif *Notification) {
	switch notif.Method {
	case "order.update":
		subs := c.subMgr.GetOrderSubscribers(notif.UserID)
		handler.BroadcastToSessions(subs, "order.update", notif.Params)
	case "asset.update":
		subs := c.subMgr.GetAssetSubscribers(notif.UserID, notif.Asset)
		handler.BroadcastToSessions(subs, "asset.update", notif.Params)
	}
}

func (c *Consumer) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

func (c *Consumer) processOrdersMessage(data []byte) {
	var event model.OrderEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("failed to parse order event: %v", err)
		return
	}

	c.notifyCh <- &Notification{
		Method: "order.update",
		Params: event,
		UserID: event.Order.UserID,
	}

	c.notifyCh <- &Notification{
		Method: "asset.update",
		Params: map[string]interface{}{
			"asset":  event.Stock,
			"stock":  event.Stock,
			"money":  event.Money,
			"action": "order",
		},
		UserID: event.Order.UserID,
		Asset:  event.Stock,
	}
}

func (c *Consumer) processBalancesMessage(data []byte) {
	var fields []interface{}
	if err := json.Unmarshal(data, &fields); err != nil {
		log.Printf("failed to parse balance event: %v", err)
		return
	}

	if len(fields) < 4 {
		return
	}

	userID, ok1 := fields[1].(float64)
	asset, ok2 := fields[2].(string)
	if !ok1 || !ok2 {
		return
	}

	body := rpc.BuildBalanceQueryBody(uint32(userID), asset)
	resp, err := c.rpcClient.QueryMatchEngine(rpc.CMD_BALANCE_QUERY, body)
	if err != nil {
		log.Printf("failed to query balance: %v", err)
		return
	}

	c.notifyCh <- &Notification{
		Method: "asset.update",
		Params: json.RawMessage(resp.Body),
		UserID: uint32(userID),
		Asset:  asset,
	}
}
