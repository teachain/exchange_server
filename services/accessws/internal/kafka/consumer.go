package kafka

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/IBM/sarama"
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
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	ordersConsumer, err := sarama.NewConsumerGroup(c.brokers, c.group+"_orders", config)
	if err != nil {
		return err
	}

	balancesConsumer, err := sarama.NewConsumerGroup(c.brokers, c.group+"_balances", config)
	if err != nil {
		return err
	}

	ordersHandler := &consumerGroupHandler{
		consumerType: "orders",
		subMgr:       c.subMgr,
		rpcClient:    c.rpcClient,
		notifyCh:     c.notifyCh,
	}

	balancesHandler := &consumerGroupHandler{
		consumerType: "balances",
		subMgr:       c.subMgr,
		rpcClient:    c.rpcClient,
		notifyCh:     c.notifyCh,
	}

	c.wg.Add(2)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.stopCh:
				return
			default:
				ordersConsumer.Consume(nil, []string{c.ordersTopic}, ordersHandler)
			}
		}
	}()

	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.stopCh:
				return
			default:
				balancesConsumer.Consume(nil, []string{c.balancesTopic}, balancesHandler)
			}
		}
	}()

	go c.processNotifications()

	return nil
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

type consumerGroupHandler struct {
	consumerType string
	subMgr       *subscription.Manager
	rpcClient    *rpc.RPCCLient
	notifyCh     chan *Notification
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			if h.consumerType == "orders" {
				h.processOrdersMessage(msg.Value)
			} else {
				h.processBalancesMessage(msg.Value)
			}
			session.MarkMessage(msg, "")
		}
	}
}

func (h *consumerGroupHandler) processOrdersMessage(data []byte) {
	var event model.OrderEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("failed to parse order event: %v", err)
		return
	}

	h.notifyCh <- &Notification{
		Method: "order.update",
		Params: event,
		UserID: event.Order.UserID,
	}

	h.notifyCh <- &Notification{
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

func (h *consumerGroupHandler) processBalancesMessage(data []byte) {
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
	resp, err := h.rpcClient.QueryMatchEngine(rpc.CMD_BALANCE_QUERY, body)
	if err != nil {
		log.Printf("failed to query balance: %v", err)
		return
	}

	h.notifyCh <- &Notification{
		Method: "asset.update",
		Params: json.RawMessage(resp.Body),
		UserID: uint32(userID),
		Asset:  asset,
	}
}
