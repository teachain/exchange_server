package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/teachain/exchange_server/services/accessws/internal/auth"
	"github.com/teachain/exchange_server/services/accessws/internal/cache"
	"github.com/teachain/exchange_server/services/accessws/internal/config"
	"github.com/teachain/exchange_server/services/accessws/internal/handler"
	"github.com/teachain/exchange_server/services/accessws/internal/model"
	"github.com/teachain/exchange_server/services/accessws/internal/rpc"
	"github.com/teachain/exchange_server/services/accessws/internal/subscription"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type WSServer struct {
	config      *config.Config
	authService *auth.AuthService
	cache       *cache.Cache
	subMgr      *subscription.Manager
	rpcClient   *rpc.RPCCLient

	serverHandler *handler.ServerHandler
	orderHandler  *handler.OrderHandler
	assetHandler  *handler.AssetHandler
	depthHandler  *handler.DepthHandler
	klineHandler  *handler.KlineHandler
	priceHandler  *handler.PriceHandler
	dealsHandler  *handler.DealsHandler
	stateHandler  *handler.StateHandler
	todayHandler  *handler.TodayHandler

	sessions  map[uint64]*model.ClientSession
	nextSesID uint64
	sesMu     sync.RWMutex

	workerNum int
	msgChan   chan *Message
	stopCh    chan struct{}
}

type Message struct {
	Sess *model.ClientSession
	Data []byte
}

func NewWSServer(cfg *config.Config) (*WSServer, error) {
	s := &WSServer{
		config:    cfg,
		cache:     cache.NewCache(cfg.CacheTimeout),
		subMgr:    subscription.NewManager(),
		sessions:  make(map[uint64]*model.ClientSession),
		workerNum: cfg.Server.WorkerNum,
		msgChan:   make(chan *Message, 10000),
		stopCh:    make(chan struct{}),
	}

	meAddr := fmt.Sprintf("%s:%d", cfg.MatchEngine.Host, cfg.MatchEngine.Port)
	mpAddr := fmt.Sprintf("%s:%d", cfg.MarketPrice.Host, cfg.MarketPrice.Port)
	rhAddr := fmt.Sprintf("%s:%d", cfg.ReadHistory.Host, cfg.ReadHistory.Port)
	timeout := time.Duration(cfg.MatchEngine.Timeout * float64(time.Second))
	s.rpcClient = rpc.NewRPCClient(meAddr, mpAddr, rhAddr, timeout, cfg.CacheTimeout)

	s.authService = auth.NewAuthService(cfg.AuthURL, cfg.SignURL, cfg.CacheTimeout)
	s.serverHandler = handler.NewServerHandler(s.authService)
	s.orderHandler = handler.NewOrderHandler(s.rpcClient, s.subMgr)
	s.assetHandler = handler.NewAssetHandler(s.rpcClient, s.subMgr)
	s.depthHandler = handler.NewDepthHandler(s.rpcClient, s.subMgr, cfg.DepthLimit, cfg.DepthMerge)
	s.klineHandler = handler.NewKlineHandler(s.rpcClient, s.subMgr)
	s.priceHandler = handler.NewPriceHandler(s.rpcClient, s.subMgr)
	s.dealsHandler = handler.NewDealsHandler(s.rpcClient, s.subMgr)
	s.stateHandler = handler.NewStateHandler(s.rpcClient, s.subMgr)
	s.todayHandler = handler.NewTodayHandler(s.rpcClient, s.subMgr)

	return s, nil
}

func (s *WSServer) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	for i := 0; i < s.config.Server.WorkerNum; i++ {
		go s.worker()
	}

	go s.startTimers()

	httpServer := &http.Server{Handler: s}
	return httpServer.Serve(ln)
}

func (s *WSServer) worker() {
	for {
		select {
		case msg := <-s.msgChan:
			s.handleMessage(msg.Sess, msg.Data)
		case <-s.stopCh:
			return
		}
	}
}

func (s *WSServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	sess := s.createSession(conn)
	go s.readLoop(sess)
}

func (s *WSServer) createSession(conn *websocket.Conn) *model.ClientSession {
	s.sesMu.Lock()
	id := atomic.AddUint64(&s.nextSesID, 1)
	sess := &model.ClientSession{
		ID:      id,
		Conn:    conn,
		Auth:    false,
		Markets: make(map[string]bool),
		Assets:  make(map[string]bool),
	}
	s.sessions[id] = sess
	s.sesMu.Unlock()
	return sess
}

func (s *WSServer) readLoop(sess *model.ClientSession) {
	defer func() {
		s.removeSession(sess)
		sess.Conn.Close()
	}()

	sess.Conn.SetReadLimit(65536)
	sess.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	sess.Conn.SetPongHandler(func(string) error {
		sess.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := sess.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket read error: %v", err)
			}
			break
		}
		s.msgChan <- &Message{Sess: sess, Data: data}
	}
}

func (s *WSServer) removeSession(sess *model.ClientSession) {
	s.sesMu.Lock()
	delete(s.sessions, sess.ID)
	s.sesMu.Unlock()

	sess.Dead = true
	s.subMgr.OrderUnsubscribeAll(sess)
	s.subMgr.DepthUnsubscribe(sess)
	s.subMgr.KlineUnsubscribe(sess)
	s.subMgr.PriceUnsubscribe(sess)
	s.subMgr.DealsUnsubscribe(sess)
	s.subMgr.StateUnsubscribe(sess)
	s.subMgr.TodayUnsubscribe(sess)
}

func (s *WSServer) handleMessage(sess *model.ClientSession, data []byte) {
	var req model.JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		handler.SendResponse(sess.Conn, model.NewErrorResponse(nil, model.ERR_INVALID_PARAMS, "invalid json"))
		return
	}

	var resp *model.JSONRPCResponse
	switch req.Method {
	case "server.ping":
		resp = s.serverHandler.HandlePing(sess, req.ID, req.Params)
	case "server.time":
		resp = s.serverHandler.HandleTime(sess, req.ID, req.Params)
	case "server.auth":
		resp = s.serverHandler.HandleAuth(sess, req.ID, req.Params)
	case "server.sign":
		resp = s.serverHandler.HandleSign(sess, req.ID, req.Params)
	case "order.query":
		resp = s.orderHandler.HandleOrderQuery(sess, req.ID, req.Params)
	case "order.history":
		resp = s.orderHandler.HandleOrderHistory(sess, req.ID, req.Params)
	case "order.subscribe":
		resp = s.orderHandler.HandleOrderSubscribe(sess, req.ID, req.Params)
	case "order.unsubscribe":
		resp = s.orderHandler.HandleOrderUnsubscribe(sess, req.ID, req.Params)
	case "asset.query":
		resp = s.assetHandler.HandleAssetQuery(sess, req.ID, req.Params)
	case "asset.history":
		resp = s.assetHandler.HandleAssetHistory(sess, req.ID, req.Params)
	case "asset.subscribe":
		resp = s.assetHandler.HandleAssetSubscribe(sess, req.ID, req.Params)
	case "asset.unsubscribe":
		resp = s.assetHandler.HandleAssetUnsubscribe(sess, req.ID, req.Params)
	case "depth.query":
		resp = s.depthHandler.HandleDepthQuery(sess, req.ID, req.Params)
	case "depth.subscribe":
		resp = s.depthHandler.HandleDepthSubscribe(sess, req.ID, req.Params)
	case "depth.unsubscribe":
		resp = s.depthHandler.HandleDepthUnsubscribe(sess, req.ID, req.Params)
	case "kline.query":
		resp = s.klineHandler.HandleKlineQuery(sess, req.ID, req.Params)
	case "kline.subscribe":
		resp = s.klineHandler.HandleKlineSubscribe(sess, req.ID, req.Params)
	case "kline.unsubscribe":
		resp = s.klineHandler.HandleKlineUnsubscribe(sess, req.ID, req.Params)
	case "price.query":
		resp = s.priceHandler.HandlePriceQuery(sess, req.ID, req.Params)
	case "price.subscribe":
		resp = s.priceHandler.HandlePriceSubscribe(sess, req.ID, req.Params)
	case "price.unsubscribe":
		resp = s.priceHandler.HandlePriceUnsubscribe(sess, req.ID, req.Params)
	case "deals.query":
		resp = s.dealsHandler.HandleDealsQuery(sess, req.ID, req.Params)
	case "deals.subscribe":
		resp = s.dealsHandler.HandleDealsSubscribe(sess, req.ID, req.Params)
	case "deals.unsubscribe":
		resp = s.dealsHandler.HandleDealsUnsubscribe(sess, req.ID, req.Params)
	case "state.query":
		resp = s.stateHandler.HandleStateQuery(sess, req.ID, req.Params)
	case "state.subscribe":
		resp = s.stateHandler.HandleStateSubscribe(sess, req.ID, req.Params)
	case "state.unsubscribe":
		resp = s.stateHandler.HandleStateUnsubscribe(sess, req.ID, req.Params)
	case "today.query":
		resp = s.todayHandler.HandleTodayQuery(sess, req.ID, req.Params)
	case "today.subscribe":
		resp = s.todayHandler.HandleTodaySubscribe(sess, req.ID, req.Params)
	case "today.unsubscribe":
		resp = s.todayHandler.HandleTodayUnsubscribe(sess, req.ID, req.Params)
	default:
		resp = model.NewErrorResponse(req.ID, model.ERR_METHOD_NOT_FOUND, "method not found")
	}

	if resp != nil {
		handler.SendResponse(sess.Conn, resp)
	}
}

func (s *WSServer) startTimers() {
	depthTicker := time.NewTicker(time.Duration(s.config.Intervals.Depth * float64(time.Second)))
	priceTicker := time.NewTicker(time.Duration(s.config.Intervals.Price * float64(time.Second)))
	klineTicker := time.NewTicker(time.Duration(s.config.Intervals.Kline * float64(time.Second)))
	dealsTicker := time.NewTicker(time.Duration(s.config.Intervals.Deals * float64(time.Second)))
	stateTicker := time.NewTicker(time.Duration(s.config.Intervals.State * float64(time.Second)))
	todayTicker := time.NewTicker(time.Duration(s.config.Intervals.Today * float64(time.Second)))

	for {
		select {
		case <-depthTicker.C:
			s.pollDepth()
		case <-priceTicker.C:
			s.pollPrice()
		case <-klineTicker.C:
			s.pollKline()
		case <-dealsTicker.C:
			s.pollDeals()
		case <-stateTicker.C:
			s.pollState()
		case <-todayTicker.C:
			s.pollToday()
		case <-s.stopCh:
			depthTicker.Stop()
			priceTicker.Stop()
			klineTicker.Stop()
			dealsTicker.Stop()
			stateTicker.Stop()
			todayTicker.Stop()
			return
		}
	}
}

func (s *WSServer) pollDepth() {
	currentTime := time.Now().Unix()
	cleanInterval := int64(s.config.Intervals.CleanInterval)

	for key := range s.subMgr.GetAllDepthSubs() {
		parts := splitKey(key)
		if len(parts) != 3 {
			continue
		}
		market, _, limitStr := parts[0], parts[1], parts[2]
		limit := 50
		fmt.Sscanf(limitStr, "%d", &limit)

		body := rpc.BuildDepthQueryBody(market, limit)
		resp, err := s.rpcClient.QueryMatchEngine(rpc.CMD_ORDER_BOOK_DEPTH, body)
		if err != nil {
			continue
		}

		oldSnapshot := s.subMgr.GetDepthSnapshot(key)

		var depth struct {
			Bids [][]string `json:"bids"`
			Asks [][]string `json:"asks"`
		}
		json.Unmarshal(resp.Body, &depth)

		newSnapshot := &model.DepthSnapshot{
			Bids: make(map[string]string),
			Asks: make(map[string]string),
		}
		for _, bid := range depth.Bids {
			if len(bid) >= 2 {
				newSnapshot.Bids[bid[0]] = bid[1]
			}
		}
		for _, ask := range depth.Asks {
			if len(ask) >= 2 {
				newSnapshot.Asks[ask[0]] = ask[1]
			}
		}

		subs := s.subMgr.GetDepthSubscribers(key)

		lastClean := s.subMgr.GetDepthLastClean(key)
		needClean := cleanInterval > 0 && (currentTime-lastClean) >= cleanInterval

		if oldSnapshot == nil || needClean {
			handler.BroadcastToSessions(subs, "depth.update", json.RawMessage(resp.Body))
			s.subMgr.SetDepthLastClean(key, currentTime)
		} else {
			diff := computeDepthDiffModel(oldSnapshot, newSnapshot)
			diffJSON, _ := json.Marshal(diff)
			handler.BroadcastToSessions(subs, "depth.update", diffJSON)
		}

		s.subMgr.SetDepthSnapshot(key, newSnapshot)
	}
}

func computeDepthDiffModel(oldSnap, newSnap *model.DepthSnapshot) map[string]interface{} {
	result := make(map[string]interface{})

	var updatedBids [][]string
	for price, amount := range newSnap.Bids {
		if oldAmount, exists := oldSnap.Bids[price]; !exists || oldAmount != amount {
			updatedBids = append(updatedBids, []string{price, amount})
		}
	}
	for price := range oldSnap.Bids {
		if _, exists := newSnap.Bids[price]; !exists {
			updatedBids = append(updatedBids, []string{price, "0"})
		}
	}

	var updatedAsks [][]string
	for price, amount := range newSnap.Asks {
		if oldAmount, exists := oldSnap.Asks[price]; !exists || oldAmount != amount {
			updatedAsks = append(updatedAsks, []string{price, amount})
		}
	}
	for price := range oldSnap.Asks {
		if _, exists := newSnap.Asks[price]; !exists {
			updatedAsks = append(updatedAsks, []string{price, "0"})
		}
	}

	result["bids"] = updatedBids
	result["asks"] = updatedAsks
	return result
}

func (s *WSServer) pollPrice() {
	for market := range s.subMgr.GetAllPriceSubs() {
		body := rpc.BuildDealsQueryBody(market, 1, 0)
		resp, err := s.rpcClient.QueryMarketPrice(rpc.CMD_MARKET_DEALS, body)
		if err != nil {
			continue
		}

		subs := s.subMgr.GetPriceSubscribers(market)
		handler.BroadcastToSessions(subs, "price.update", json.RawMessage(resp.Body))
	}
}

func (s *WSServer) pollKline() {
	for key := range s.subMgr.GetAllKlineSubs() {
		parts := splitKey(key)
		if len(parts) != 2 {
			continue
		}
		market, interval := parts[0], parts[1]

		body := rpc.BuildKlineQueryBody(market, interval, 1)
		resp, err := s.rpcClient.QueryMarketPrice(rpc.CMD_MARKET_KLINE, body)
		if err != nil {
			continue
		}

		subs := s.subMgr.GetKlineSubscribers(key)
		handler.BroadcastToSessions(subs, "kline.update", json.RawMessage(resp.Body))
	}
}

func (s *WSServer) pollDeals() {
	for market := range s.subMgr.GetAllDealsSubs() {
		buf := s.subMgr.GetDealsBuffer(market)
		lastID := buf.GetLastID()

		body := rpc.BuildDealsQueryBody(market, 100, lastID)
		resp, err := s.rpcClient.QueryMarketPrice(rpc.CMD_MARKET_DEALS, body)
		if err != nil {
			continue
		}

		var deals []struct {
			ID     int64   `json:"id"`
			Time   float64 `json:"time"`
			Type   string  `json:"type"`
			Amount string  `json:"amount"`
			Price  string  `json:"price"`
		}
		json.Unmarshal(resp.Body, &deals)

		if len(deals) == 0 {
			continue
		}

		for _, d := range deals {
			buf.Add(model.DealRecord{
				ID:     d.ID,
				Time:   d.Time,
				Type:   d.Type,
				Amount: d.Amount,
				Price:  d.Price,
			})
		}

		subs := s.subMgr.GetDealsSubscribers(market)

		newDeals := make([]map[string]interface{}, 0, len(deals))
		for _, d := range deals {
			if d.ID > lastID {
				newDeals = append(newDeals, map[string]interface{}{
					"id":     d.ID,
					"time":   d.Time,
					"type":   d.Type,
					"amount": d.Amount,
					"price":  d.Price,
				})
			}
		}

		if len(newDeals) > 0 {
			handler.BroadcastToSessions(subs, "deals.update", newDeals)
		}
	}
}

func (s *WSServer) pollState() {
	for market := range s.subMgr.GetAllStateSubs() {
		body := rpc.BuildStateQueryBody(market)
		resp, err := s.rpcClient.QueryMarketPrice(rpc.CMD_MARKET_STATUS, body)
		if err != nil {
			continue
		}

		subs := s.subMgr.GetStateSubscribers(market)
		handler.BroadcastToSessions(subs, "state.update", json.RawMessage(resp.Body))
	}
}

func (s *WSServer) pollToday() {
	for market := range s.subMgr.GetAllTodaySubs() {
		body := rpc.BuildTodayQueryBody(market)
		resp, err := s.rpcClient.QueryMarketPrice(rpc.CMD_MARKET_STATUS_TODAY, body)
		if err != nil {
			continue
		}

		subs := s.subMgr.GetTodaySubscribers(market)
		handler.BroadcastToSessions(subs, "today.update", json.RawMessage(resp.Body))
	}
}

func splitKey(key string) []string {
	var parts []string
	var current string
	for _, c := range key {
		if c == ':' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func (s *WSServer) SubMgr() *subscription.Manager {
	return s.subMgr
}

func (s *WSServer) RPCClient() *rpc.RPCCLient {
	return s.rpcClient
}

func (s *WSServer) Stop() {
	close(s.stopCh)
}
