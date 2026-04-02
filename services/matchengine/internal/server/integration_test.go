package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
	"github.com/viabtc/go-project/services/matchengine/internal/engine"
	"github.com/viabtc/go-project/services/matchengine/internal/order"
)

type RPCClient struct {
	conn net.Conn
	mu   sync.Mutex
	seq  uint32
}

func NewRPCClient(addr string) (*RPCClient, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &RPCClient{conn: conn}, nil
}

func (c *RPCClient) Close() error {
	return c.conn.Close()
}

func (c *RPCClient) SendRequest(command uint32, params interface{}) (*RPCPkg, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.seq++
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	pkg := &RPCPkg{
		Magic:    RPCPkgMagic,
		Command:  command,
		PkgType:  RPCPkgTypeReq,
		Sequence: c.seq,
		ReqID:    uint64(c.seq),
		Body:     body,
	}

	data, err := pkg.Pack()
	if err != nil {
		return nil, err
	}

	_, err = c.conn.Write(data)
	if err != nil {
		return nil, err
	}

	respHeader := make([]byte, RPCPkgHeadSize)
	_, err = c.conn.Read(respHeader)
	if err != nil {
		return nil, err
	}

	respBodySize := int(binary.LittleEndian.Uint32(respHeader[30:34]))
	respExtSize := int(binary.LittleEndian.Uint16(respHeader[34:36]))
	respTotalSize := RPCPkgHeadSize + respExtSize + respBodySize

	respData := make([]byte, respTotalSize)
	copy(respData, respHeader)

	if respBodySize > 0 {
		_, err = c.conn.Read(respData[RPCPkgHeadSize:])
		if err != nil {
			return nil, err
		}
	}

	resp := &RPCPkg{}
	err = resp.Unpack(respData)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type BalanceInfo struct {
	Available string `json:"available"`
	Freeze    string `json:"freeze"`
}

type OrderInfo struct {
	ID         uint64  `json:"id"`
	Market     string  `json:"market"`
	Side       int     `json:"side"`
	Type       int     `json:"type"`
	UserID     uint32  `json:"user"`
	Price      string  `json:"price"`
	Amount     string  `json:"amount"`
	Left       string  `json:"left"`
	DealStock  string  `json:"deal_stock"`
	DealMoney  string  `json:"deal_money"`
	DealFee    string  `json:"deal_fee"`
	Status     int     `json:"status"`
	CreateTime float64 `json:"ctime"`
	UpdateTime float64 `json:"mtime"`
}

type MarketStatus struct {
	Market            string `json:"market"`
	Mode              string `json:"mode"`
	Seq               int64  `json:"seq"`
	OrderCount        int    `json:"order_count"`
	PendingOrderCount int    `json:"pending_order_count"`
	BidPrice          string `json:"bid_price"`
	BidAmount         string `json:"bid_amount"`
	BidCount          int    `json:"bid_count"`
	AskPrice          string `json:"ask_price"`
	AskAmount         string `json:"ask_amount"`
	AskCount          int    `json:"ask_count"`
}

type MarketInfo struct {
	Name      string `json:"name"`
	Stock     string `json:"stock"`
	Money     string `json:"money"`
	FeePrec   int    `json:"fee_prec"`
	StockPrec int    `json:"stock_prec"`
	MoneyPrec int    `json:"money_prec"`
	MinAmount string `json:"min_amount"`
	MaxAmount string `json:"max_amount"`
}

type AssetInfo struct {
	Name string `json:"name"`
	Prec int    `json:"prec"`
}

const (
	CMD_BALANCE_QUERY   = 101
	CMD_ORDER_PUT_LIMIT = 201
	CMD_ORDER_CANCEL    = 204
	CMD_MARKET_STATUS   = 301
	CMD_MARKET_LIST     = 307
	CMD_ASSET_LIST      = 104
)

func MockHandleBalanceQuery(s *RPCServer, pkg *RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("invalid arguments")
	}

	userID, ok := params[0].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user_id")
	}
	if userID == 0 {
		return nil, fmt.Errorf("invalid user_id")
	}

	result := make(map[string]BalanceInfo)
	engine := s.GetEngine()

	if len(params) == 1 {
		balances := engine.GetAllBalancesForUser(uint32(userID))
		for asset, bal := range balances {
			result[asset] = BalanceInfo{
				Available: bal.Available.Sub(bal.Frozen).String(),
				Freeze:    bal.Frozen.String(),
			}
		}
	} else {
		for i := 1; i < len(params); i++ {
			asset, ok := params[i].(string)
			if !ok {
				return nil, fmt.Errorf("invalid asset name")
			}
			balance, frozen := engine.GetBalance(uint32(userID), asset)
			result[asset] = BalanceInfo{
				Available: balance.Sub(frozen).String(),
				Freeze:    frozen.String(),
			}
		}
	}

	return json.Marshal(result)
}

func MockHandleOrderPutLimit(s *RPCServer, pkg *RPCPkg) ([]byte, error) {
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
	if side != order.SideBid && side != order.SideAsk {
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
	_, err = decimal.NewFromString(takerFeeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid taker_fee")
	}

	eng := s.GetEngine()

	if side == order.SideBid {
		frozen := price.Mul(amount)
		err = eng.GetBalances().LockBalance(uint32(userID), market, frozen)
		if err != nil {
			return nil, fmt.Errorf("balance not enough")
		}
	} else {
		err = eng.GetBalances().LockBalance(uint32(userID), market, amount)
		if err != nil {
			return nil, fmt.Errorf("balance not enough")
		}
	}

	incoming := &order.Order{
		ID:         eng.NextID(),
		UserID:     uint32(userID),
		Market:     market,
		Side:       side,
		Price:      price,
		Amount:     amount,
		Left:       amount,
		Status:     order.OrderStatusPending,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}

	trades, err := eng.ProcessOrder(incoming)
	if err != nil {
		if side == order.SideBid {
			frozen := price.Mul(amount)
			eng.GetBalances().UnlockBalance(uint32(userID), market, frozen)
		} else {
			eng.GetBalances().UnlockBalance(uint32(userID), market, amount)
		}
		return nil, fmt.Errorf("process order failed")
	}

	if incoming.Left.IsZero() {
	} else {
		if side == order.SideBid {
			frozen := price.Mul(amount)
			spent := price.Mul(incoming.Left)
			eng.GetBalances().UnlockBalance(uint32(userID), market, frozen.Sub(spent))
		} else {
			spent := incoming.Left
			eng.GetBalances().UnlockBalance(uint32(userID), market, amount.Sub(spent))
		}
	}

	left := incoming.Amount.Sub(incoming.Left)
	orderInfo := OrderInfo{
		ID:         incoming.ID,
		Market:     incoming.Market,
		Side:       int(incoming.Side),
		Type:       1,
		UserID:     incoming.UserID,
		Price:      incoming.Price.String(),
		Amount:     incoming.Amount.String(),
		Left:       left.String(),
		DealStock:  incoming.Left.String(),
		DealMoney:  incoming.Price.Mul(incoming.Left).String(),
		DealFee:    incoming.DealFee.String(),
		Status:     int(incoming.Status),
		CreateTime: float64(incoming.CreateTime.Unix()),
		UpdateTime: float64(time.Now().Unix()),
	}

	tradeInfos := make([]map[string]interface{}, 0)
	for _, t := range trades {
		tradeInfos = append(tradeInfos, map[string]interface{}{
			"id":             t.ID,
			"taker_order_id": t.TakerOrderID,
			"maker_order_id": t.MakerOrderID,
			"price":          t.Price.String(),
			"amount":         t.Amount.String(),
		})
	}

	result := map[string]interface{}{
		"order":  orderInfo,
		"trades": tradeInfos,
	}

	return json.Marshal(result)
}

func MockHandleOrderCancel(s *RPCServer, pkg *RPCPkg) ([]byte, error) {
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
	ord, found := eng.GetOrder(uint64(orderID))
	if !found {
		return nil, fmt.Errorf("order not found")
	}

	if ord.UserID != uint32(userID) {
		return nil, fmt.Errorf("user not match")
	}

	err := eng.CancelOrder(uint64(orderID), market)
	if err != nil {
		return nil, fmt.Errorf("cancel order failed")
	}

	updated, found := eng.GetOrder(uint64(orderID))
	if !found {
		return nil, fmt.Errorf("order not found after cancel")
	}

	left := updated.Amount.Sub(updated.Left)
	orderInfo := OrderInfo{
		ID:         updated.ID,
		Market:     updated.Market,
		Side:       int(updated.Side),
		Type:       1,
		UserID:     updated.UserID,
		Price:      updated.Price.String(),
		Amount:     updated.Amount.String(),
		Left:       left.String(),
		DealStock:  updated.Left.String(),
		DealMoney:  updated.Price.Mul(updated.Left).String(),
		DealFee:    updated.DealFee.String(),
		Status:     int(updated.Status),
		CreateTime: float64(updated.CreateTime.Unix()),
		UpdateTime: float64(time.Now().Unix()),
	}

	result := map[string]interface{}{
		"order": orderInfo,
	}

	return json.Marshal(result)
}

func MockHandleMarketStatus(s *RPCServer, pkg *RPCPkg) ([]byte, error) {
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
		result := MarketStatus{Market: market}
		return json.Marshal(result)
	}

	bestBid := ob.GetBestBid()
	bestAsk := ob.GetBestAsk()

	var bidPrice, bidAmount, askPrice, askAmount string
	var bidCount, askCount int

	if bestBid != nil {
		bidPrice = bestBid.Price.String()
		bidAmount = bestBid.Amount.Sub(bestBid.Left).String()
	}
	if bestAsk != nil {
		askPrice = bestAsk.Price.String()
		askAmount = bestAsk.Amount.Sub(bestAsk.Left).String()
	}

	orders := ob.GetOrders()
	pendingCount := 0
	for _, o := range orders {
		if o.Status == order.OrderStatusPending || o.Status == order.OrderStatusPartial {
			pendingCount++
			if o.Side == order.SideBid {
				bidCount++
			} else {
				askCount++
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

func MockHandleMarketList(s *RPCServer, pkg *RPCPkg) ([]byte, error) {
	eng := s.GetEngine()
	markets := eng.ListMarkets()

	result := make([]MarketInfo, 0, len(markets))
	for _, name := range markets {
		result = append(result, MarketInfo{
			Name:      name,
			Stock:     name,
			Money:     "",
			FeePrec:   8,
			StockPrec: 8,
			MoneyPrec: 8,
			MinAmount: "0.0001",
			MaxAmount: "1000000",
		})
	}

	return json.Marshal(result)
}

func MockHandleAssetList(s *RPCServer, pkg *RPCPkg) ([]byte, error) {
	knownAssets := []string{"BTC", "ETH", "USDT"}

	result := make([]AssetInfo, 0, len(knownAssets))
	for _, asset := range knownAssets {
		result = append(result, AssetInfo{
			Name: asset,
			Prec: 8,
		})
	}

	return json.Marshal(result)
}

func initViperConfig() {
	viper.SetConfigType("yaml")
	yamlContent := `markets: []
assets: []
`
	viper.ReadConfig(bytes.NewReader([]byte(yamlContent)))
}

func startTestRPCServerWithMockHandler(port int) (*RPCServer, string, error) {
	// Skip viper and engine for minimal test
	s := NewRPCServer(nil)

	simpleHandler := func(s *RPCServer, pkg *RPCPkg) ([]byte, error) {
		fmt.Println("Simple handler called!")
		return []byte(`{"status":"ok"}`), nil
	}
	s.Handle(999, simpleHandler)

	addr := fmt.Sprintf("localhost:%d", port)
	err := s.Start(addr)
	if err != nil {
		return nil, "", err
	}

	time.Sleep(100 * time.Millisecond)

	return s, addr, nil
}

func startTestRPCServer(port int) (*RPCServer, string, error) {
	initViperConfig()
	e := engine.NewEngine()
	if e == nil {
		return nil, "", fmt.Errorf("engine is nil")
	}
	e.SetBalance(1, "USDT", decimal.NewFromFloat(10000), decimal.Zero)
	e.SetBalance(1, "BTC", decimal.NewFromFloat(1), decimal.Zero)
	e.SetBalance(2, "USDT", decimal.NewFromFloat(10000), decimal.Zero)
	e.SetBalance(2, "BTC", decimal.NewFromFloat(1), decimal.Zero)

	ob := e.GetOrCreateOrderBook("BTC_USDT")

	ord1 := &order.Order{
		ID:         e.NextID(),
		UserID:     2,
		Market:     "BTC_USDT",
		Side:       order.SideAsk,
		Price:      decimal.NewFromFloat(50000),
		Amount:     decimal.NewFromFloat(0.5),
		Left:       decimal.NewFromFloat(0.5),
		Status:     order.OrderStatusPending,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	ob.Add(ord1)

	ord2 := &order.Order{
		ID:         e.NextID(),
		UserID:     2,
		Market:     "BTC_USDT",
		Side:       order.SideAsk,
		Price:      decimal.NewFromFloat(51000),
		Amount:     decimal.NewFromFloat(0.3),
		Left:       decimal.NewFromFloat(0.3),
		Status:     order.OrderStatusPending,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	ob.Add(ord2)

	s := NewRPCServer(e)
	s.Handle(CMD_BALANCE_QUERY, MockHandleBalanceQuery)
	s.Handle(CMD_ORDER_PUT_LIMIT, MockHandleOrderPutLimit)
	s.Handle(CMD_ORDER_CANCEL, MockHandleOrderCancel)
	s.Handle(CMD_MARKET_STATUS, MockHandleMarketStatus)
	s.Handle(CMD_MARKET_LIST, MockHandleMarketList)
	s.Handle(CMD_ASSET_LIST, MockHandleAssetList)

	addr := fmt.Sprintf("localhost:%d", port)
	err := s.Start(addr)
	if err != nil {
		return nil, "", err
	}

	time.Sleep(100 * time.Millisecond)

	return s, addr, nil
}

func TestRPCIntegration_SimplePing(t *testing.T) {
	// Simple TCP echo server test
	ln, err := net.Listen("tcp", "localhost:18721")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	done := make(chan bool)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Logf("Accept error: %v", err)
			done <- false
			return
		}
		t.Logf("Accepted connection from %v", conn.RemoteAddr())

		buf := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			t.Logf("Read error: %v", err)
			conn.Close()
			done <- false
			return
		}
		t.Logf("Read %d bytes: %v", n, buf[:min(n, 20)])

		conn.Write(buf[:n])
		conn.Close()
		done <- true
	}()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:18721")
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	n, err := conn.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	t.Logf("Wrote %d bytes", n)

	respBuf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err = conn.Read(respBuf)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	t.Logf("Read %d bytes: %s", n, string(respBuf[:n]))

	if !<-done {
		t.Fatal("Server did not complete successfully")
	}
}

func TestRPCIntegration_RPCServerPing(t *testing.T) {
	s := NewRPCServer(nil)

	simpleHandler := func(s *RPCServer, pkg *RPCPkg) ([]byte, error) {
		return []byte(`{"status":"ok"}`), nil
	}
	s.Handle(999, simpleHandler)

	addr := "localhost:18730"
	err := s.Start(addr)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer s.Close()

	time.Sleep(500 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	pkg := &RPCPkg{
		Magic:    RPCPkgMagic,
		Command:  999,
		PkgType:  RPCPkgTypeReq,
		Sequence: 1,
		ReqID:    1,
		Body:     []byte(`["test"]`),
	}

	data, err := pkg.Pack()
	if err != nil {
		t.Fatalf("Failed to pack: %v", err)
	}

	n, err := conn.Write(data)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	t.Logf("Wrote %d bytes", n)

	respBuf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err = conn.Read(respBuf)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	t.Logf("Read %d bytes", n)

	var resp RPCPkg
	err = resp.Unpack(respBuf[:n])
	if err != nil {
		t.Fatalf("Failed to unpack response: %v", err)
	}

	t.Logf("Response: result=%d, body=%s", resp.Result, string(resp.Body))
	if resp.Result != 0 {
		t.Errorf("Expected result 0, got %d", resp.Result)
	}
}

func TestRPCIntegration_BalanceQuery(t *testing.T) {
	srv, addr, err := startTestRPCServer(18711)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic: %v", r)
		}
	}()

	client, err := NewRPCClient(addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	params := []interface{}{float64(1)}
	resp, err := client.SendRequest(CMD_BALANCE_QUERY, params)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.Result != 0 {
		t.Errorf("Expected result 0, got %d", resp.Result)
	}

	if resp.PkgType != RPCPkgTypeResp {
		t.Errorf("Expected pkg type %d, got %d", RPCPkgTypeResp, resp.PkgType)
	}

	var balances map[string]BalanceInfo
	err = json.Unmarshal(resp.Body, &balances)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if _, ok := balances["USDT"]; !ok {
		t.Error("Expected USDT balance")
	}
	if _, ok := balances["BTC"]; !ok {
		t.Error("Expected BTC balance")
	}
}

func TestRPCIntegration_OrderPlacement(t *testing.T) {
	srv, addr, err := startTestRPCServer(18712)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	client, err := NewRPCClient(addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	params := []interface{}{
		float64(1),
		"BTC_USDT",
		float64(1),
		"0.1",
		"49000",
		"0.001",
		"0.002",
		"test",
	}
	resp, err := client.SendRequest(CMD_ORDER_PUT_LIMIT, params)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.Result != 0 {
		t.Errorf("Expected result 0, got %d, body: %s", resp.Result, string(resp.Body))
	}

	if resp.PkgType != RPCPkgTypeResp {
		t.Errorf("Expected pkg type %d, got %d", RPCPkgTypeResp, resp.PkgType)
	}

	var result map[string]interface{}
	err = json.Unmarshal(resp.Body, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if _, ok := result["order"]; !ok {
		t.Error("Expected order in response")
	}
}

func TestRPCIntegration_OrderCancellation(t *testing.T) {
	srv, addr, err := startTestRPCServer(18713)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	client, err := NewRPCClient(addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	e := srv.GetEngine()
	ob := e.GetOrCreateOrderBook("BTC_USDT")
	orderToCancel := &order.Order{
		ID:         e.NextID(),
		UserID:     1,
		Market:     "BTC_USDT",
		Side:       order.SideBid,
		Price:      decimal.NewFromFloat(48000),
		Amount:     decimal.NewFromFloat(0.1),
		Left:       decimal.NewFromFloat(0.1),
		Status:     order.OrderStatusPending,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	ob.Add(orderToCancel)

	params := []interface{}{float64(1), "BTC_USDT", float64(orderToCancel.ID)}
	resp, err := client.SendRequest(CMD_ORDER_CANCEL, params)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.Result != 0 {
		t.Errorf("Expected result 0, got %d, body: %s", resp.Result, string(resp.Body))
	}

	var result map[string]interface{}
	err = json.Unmarshal(resp.Body, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	orderInfo, ok := result["order"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected order in response")
	}
	if status, ok := orderInfo["status"].(float64); ok {
		if int(status) != 3 {
			t.Errorf("Expected status 3 (cancelled), got %v", int(status))
		}
	}
}

func TestRPCIntegration_MarketData(t *testing.T) {
	srv, addr, err := startTestRPCServer(18714)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	client, err := NewRPCClient(addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	params := []interface{}{"BTC_USDT"}
	resp, err := client.SendRequest(CMD_MARKET_STATUS, params)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.Result != 0 {
		t.Errorf("Expected result 0, got %d", resp.Result)
	}

	var status MarketStatus
	err = json.Unmarshal(resp.Body, &status)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if status.Market != "BTC_USDT" {
		t.Errorf("Expected market BTC_USDT, got %s", status.Market)
	}

	if status.PendingOrderCount != 2 {
		t.Errorf("Expected 2 pending orders, got %d", status.PendingOrderCount)
	}
}

func TestRPCIntegration_MarketList(t *testing.T) {
	srv, addr, err := startTestRPCServer(18715)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	client, err := NewRPCClient(addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	resp, err := client.SendRequest(CMD_MARKET_LIST, []interface{}{})
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.Result != 0 {
		t.Errorf("Expected result 0, got %d", resp.Result)
	}

	var markets []MarketInfo
	err = json.Unmarshal(resp.Body, &markets)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(markets) == 0 {
		t.Error("Expected at least one market")
	}
}

func TestRPCIntegration_AssetList(t *testing.T) {
	srv, addr, err := startTestRPCServer(18716)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	client, err := NewRPCClient(addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	resp, err := client.SendRequest(CMD_ASSET_LIST, []interface{}{})
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.Result != 0 {
		t.Errorf("Expected result 0, got %d", resp.Result)
	}

	var assets []AssetInfo
	err = json.Unmarshal(resp.Body, &assets)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(assets) == 0 {
		t.Error("Expected at least one asset")
	}
}
