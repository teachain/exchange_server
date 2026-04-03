package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/viabtc/go-project/services/readhistory/internal/reader"
)

type HandlerFunc func(s *Server, pkg *RPCPkg) ([]byte, error)
type HTTPHandlerFunc func(s *Server, c *gin.Context) ([]byte, error)

type Server struct {
	listener     net.Listener
	handlers     map[uint32]HandlerFunc
	httpHandlers map[string]HTTPHandlerFunc
	reader       interface{}
	mu           sync.RWMutex
	router       *gin.Engine
	timeout      time.Duration
	debug        bool
}

func New(reader interface{}) *Server {
	return &Server{
		handlers:     make(map[uint32]HandlerFunc),
		httpHandlers: make(map[string]HTTPHandlerFunc),
		reader:       reader,
		router:       gin.Default(),
		timeout:      30 * time.Second,
	}
}

func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, _ := context.WithTimeout(c.Request.Context(), timeout)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (s *Server) SetTimeout(timeout time.Duration) {
	s.timeout = timeout
}

func (s *Server) SetDebug(debug bool) {
	s.debug = debug
}

func (s *Server) IsDebug() bool {
	return s.debug
}

func (s *Server) Start(addr string) error {
	s.setupHTTPRoutes()
	go func() {
		s.router.Run(addr)
	}()
	quit := make(chan struct{})
	<-quit
	return nil
}

func (s *Server) setupHTTPRoutes() {
	s.router.Use(timeoutMiddleware(s.timeout))
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	s.router.POST("/balance_history", s.handleHTTPBalanceHistory)
	s.router.POST("/order.deals", s.handleHTTPOrderDeals)
	s.router.POST("/order.finished", s.handleHTTPOrderFinished)
	s.router.POST("/order.finished_detail", s.handleHTTPOrderFinishedDetail)
	s.router.POST("/", s.handleHTTPJSONRPC)
}

func (s *Server) handleHTTPJSONRPC(c *gin.Context) {
	var req struct {
		ID     int64         `json:"id"`
		Method string        `json:"method"`
		Params []interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32700, "message": "Parse error"}, "id": nil})
		return
	}

	r := s.GetReader().(*reader.Reader)

	switch req.Method {
	case "order.deals":
		handleHTTPOrderDealsWithReq(c, &req, r)
	case "order.finished":
		handleHTTPOrderFinishedWithReq(c, &req, r)
	case "order.finished_detail":
		handleHTTPOrderFinishedDetailWithReq(c, &req, r)
	case "balance.history":
		handleHTTPBalanceHistoryWithReq(c, &req, r)
	case "market.user_deals":
		handleHTTPUserDealsWithReq(c, &req, r)
	default:
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32601, "message": "Method not found"}, "id": req.ID})
	}
}

func handleHTTPOrderDealsWithReq(c *gin.Context, req *struct {
	ID     int64         `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}, r *reader.Reader) {
	if len(req.Params) != 3 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid params"}, "id": req.ID})
		return
	}

	orderID := uint64(req.Params[0].(float64))
	if orderID == 0 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid order_id"}, "id": req.ID})
		return
	}
	offset := int(req.Params[1].(float64))
	limit := int(req.Params[2].(float64))

	if limit == 0 || limit > 101 {
		limit = 101
	}

	records, err := r.GetOrderDeals(orderID, offset, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32603, "message": err.Error()}, "id": req.ID})
		return
	}

	result := make([]map[string]interface{}, 0, len(records))
	for _, rec := range records {
		result = append(result, map[string]interface{}{
			"time":          rec.Time,
			"deal_id":       rec.DealID,
			"order_id":      rec.OrderID,
			"deal_order_id": rec.DealOrderID,
			"role":          rec.Role,
			"price":         rec.Price.String(),
			"amount":        rec.Amount.String(),
			"deal":          rec.Deal.String(),
			"fee":           rec.Fee.String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result": gin.H{
			"offset":  offset,
			"limit":   limit,
			"records": result,
		},
		"id": req.ID,
	})
}

func handleHTTPOrderFinishedWithReq(c *gin.Context, req *struct {
	ID     int64         `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}, r *reader.Reader) {
	if len(req.Params) < 6 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid params"}, "id": req.ID})
		return
	}

	userID := uint32(req.Params[0].(float64))
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid user_id"}, "id": req.ID})
		return
	}

	market, ok := req.Params[1].(string)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid market"}, "id": req.ID})
		return
	}

	startTime := uint64(req.Params[2].(float64))
	endTime := uint64(req.Params[3].(float64))
	offset := int(req.Params[4].(float64))
	limit := int(req.Params[5].(float64))

	if limit == 0 || limit > 101 {
		limit = 101
	}

	side := 0
	if len(req.Params) >= 7 {
		side = int(req.Params[6].(float64))
	}

	records, err := r.GetFinishedOrders(userID, market, side, startTime, endTime, offset, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32603, "message": err.Error()}, "id": req.ID})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result": gin.H{
			"offset":  offset,
			"limit":   limit,
			"records": records,
		},
		"id": req.ID,
	})
}

func handleHTTPOrderFinishedDetailWithReq(c *gin.Context, req *struct {
	ID     int64         `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}, r *reader.Reader) {
	if len(req.Params) != 1 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid params"}, "id": req.ID})
		return
	}

	orderID := uint64(req.Params[0].(float64))
	if orderID == 0 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid order_id"}, "id": req.ID})
		return
	}

	detail, err := r.GetFinishedOrderDetail(orderID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32603, "message": err.Error()}, "id": req.ID})
		return
	}

	if detail == nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "result": nil, "id": req.ID})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result":  detail,
		"id":      req.ID,
	})
}

func handleHTTPBalanceHistoryWithReq(c *gin.Context, req *struct {
	ID     int64         `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}, r *reader.Reader) {
	if len(req.Params) < 7 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "invalid params"}, "id": req.ID})
		return
	}

	userID := uint32(req.Params[0].(float64))
	asset, _ := req.Params[1].(string)
	business, _ := req.Params[2].(string)
	startTime := uint64(req.Params[3].(float64))
	endTime := uint64(req.Params[4].(float64))
	offset := int(req.Params[5].(float64))
	limit := int(req.Params[6].(float64))

	if limit == 0 || limit > 101 {
		limit = 101
	}

	records, err := r.GetBalanceHistory(userID, asset, business, startTime, endTime, offset, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32603, "message": err.Error()}, "id": req.ID})
		return
	}

	result := make([]map[string]interface{}, 0, len(records))
	for _, rec := range records {
		result = append(result, map[string]interface{}{
			"time":     rec.Time,
			"asset":    rec.Asset,
			"business": rec.Business,
			"change":   rec.Change.String(),
			"balance":  rec.Balance.String(),
			"detail":   rec.Detail,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result": gin.H{
			"offset":  offset,
			"limit":   limit,
			"records": result,
		},
		"id": req.ID,
	})
}

func (s *Server) Handle(cmd uint32, fn HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[cmd] = fn
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		header := make([]byte, RPCPkgHeadSize)
		_, err := conn.Read(header)
		if err != nil {
			return
		}

		var pkg RPCPkg
		if err := binary.Read(conn, binary.LittleEndian, &pkg); err != nil {
			return
		}

		if pkg.BodySize > 0 {
			body := make([]byte, pkg.BodySize)
			_, err := conn.Read(body)
			if err != nil {
				return
			}
			pkg.Body = body
		}

		go s.dispatch(&pkg, conn)
	}
}

func (s *Server) dispatch(pkg *RPCPkg, conn net.Conn) {
	s.mu.RLock()
	handler, ok := s.handlers[pkg.Command]
	s.mu.RUnlock()

	var result []byte
	var err error

	if ok {
		result, err = handler(s, pkg)
	} else {
		err = fmt.Errorf("unknown command: %d", pkg.Command)
	}

	s.sendResponse(pkg, result, err, conn)
}

func (s *Server) sendResponse(pkg *RPCPkg, result []byte, err error, conn net.Conn) {
	resp := &RPCPkg{
		Magic:    RPCPkgMagic,
		Command:  pkg.Command,
		PkgType:  RPCPkgTypeResp,
		Sequence: pkg.Sequence,
		ReqID:    pkg.ReqID,
	}

	if err != nil {
		resp.Result = 1
		errorResp := map[string]interface{}{
			"error": map[string]interface{}{
				"code":    1,
				"message": err.Error(),
			},
			"result": nil,
			"id":     pkg.ReqID,
		}
		resp.Body, _ = json.Marshal(errorResp)
	} else {
		resp.Result = 0
		successResp := map[string]interface{}{
			"error":  nil,
			"result": json.RawMessage(result),
			"id":     pkg.ReqID,
		}
		resp.Body, _ = json.Marshal(successResp)
	}
	resp.BodySize = uint32(len(resp.Body))

	data, _ := resp.Pack()
	conn.Write(data)
}

func (s *Server) GetReader() interface{} {
	return s.reader
}

func (s *Server) handleHTTPBalanceHistory(c *gin.Context) {
	var req struct {
		ID     int64         `json:"id"`
		Method string        `json:"method"`
		Params []interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}

	if len(req.Params) < 7 {
		c.JSON(http.StatusOK, gin.H{"error": "invalid params"})
		return
	}

	userID := uint32(req.Params[0].(float64))
	asset, _ := req.Params[1].(string)
	business, _ := req.Params[2].(string)
	startTime := uint64(req.Params[3].(float64))
	endTime := uint64(req.Params[4].(float64))
	offset := int(req.Params[5].(float64))
	limit := int(req.Params[6].(float64))

	if limit == 0 || limit > 101 {
		limit = 101
	}

	r := s.GetReader()
	read := r.(*reader.Reader)

	records, err := read.GetBalanceHistory(userID, asset, business, startTime, endTime, offset, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}

	result := make([]map[string]interface{}, 0, len(records))
	for _, rec := range records {
		result = append(result, map[string]interface{}{
			"time":     rec.Time,
			"asset":    rec.Asset,
			"business": rec.Business,
			"change":   rec.Change.String(),
			"balance":  rec.Balance.String(),
			"detail":   rec.Detail,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result": gin.H{
			"offset":  offset,
			"limit":   limit,
			"records": result,
		},
		"id": req.ID,
	})
}

func (s *Server) handleHTTPOrderDeals(c *gin.Context) {
	var req struct {
		ID     int64         `json:"id"`
		Method string        `json:"method"`
		Params []interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32700, "message": "Parse error"}, "id": nil})
		return
	}

	if len(req.Params) != 3 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid params"}, "id": req.ID})
		return
	}

	orderID := uint64(req.Params[0].(float64))
	if orderID == 0 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid order_id"}, "id": req.ID})
		return
	}
	offset := int(req.Params[1].(float64))
	limit := int(req.Params[2].(float64))

	if limit == 0 || limit > 101 {
		limit = 101
	}

	r := s.GetReader().(*reader.Reader)
	records, err := r.GetOrderDeals(orderID, offset, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32603, "message": err.Error()}, "id": req.ID})
		return
	}

	result := make([]map[string]interface{}, 0, len(records))
	for _, rec := range records {
		result = append(result, map[string]interface{}{
			"time":          rec.Time,
			"deal_id":       rec.DealID,
			"order_id":      rec.OrderID,
			"deal_order_id": rec.DealOrderID,
			"role":          rec.Role,
			"price":         rec.Price.String(),
			"amount":        rec.Amount.String(),
			"deal":          rec.Deal.String(),
			"fee":           rec.Fee.String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result": gin.H{
			"offset":  offset,
			"limit":   limit,
			"records": result,
		},
		"id": req.ID,
	})
}

func (s *Server) handleHTTPOrderFinished(c *gin.Context) {
	var req struct {
		ID     int64         `json:"id"`
		Method string        `json:"method"`
		Params []interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32700, "message": "Parse error"}, "id": nil})
		return
	}

	if len(req.Params) < 6 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid params"}, "id": req.ID})
		return
	}

	userID := uint32(req.Params[0].(float64))
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid user_id"}, "id": req.ID})
		return
	}

	market, ok := req.Params[1].(string)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid market"}, "id": req.ID})
		return
	}

	startTime := uint64(req.Params[2].(float64))
	endTime := uint64(req.Params[3].(float64))
	offset := int(req.Params[4].(float64))
	limit := int(req.Params[5].(float64))

	if limit == 0 || limit > 101 {
		limit = 101
	}

	side := 0
	if len(req.Params) >= 7 {
		side = int(req.Params[6].(float64))
	}

	r := s.GetReader().(*reader.Reader)
	records, err := r.GetFinishedOrders(userID, market, side, startTime, endTime, offset, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32603, "message": err.Error()}, "id": req.ID})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result": gin.H{
			"offset":  offset,
			"limit":   limit,
			"records": records,
		},
		"id": req.ID,
	})
}

func (s *Server) handleHTTPOrderFinishedDetail(c *gin.Context) {
	var req struct {
		ID     int64         `json:"id"`
		Method string        `json:"method"`
		Params []interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32700, "message": "Parse error"}, "id": nil})
		return
	}

	if len(req.Params) != 1 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid params"}, "id": req.ID})
		return
	}

	orderID := uint64(req.Params[0].(float64))
	if orderID == 0 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid order_id"}, "id": req.ID})
		return
	}

	r := s.GetReader().(*reader.Reader)
	detail, err := r.GetFinishedOrderDetail(orderID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32603, "message": err.Error()}, "id": req.ID})
		return
	}

	if detail == nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "result": nil, "id": req.ID})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result":  detail,
		"id":      req.ID,
	})
}

func handleHTTPUserDealsWithReq(c *gin.Context, req *struct {
	ID     int64         `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}, r *reader.Reader) {
	if len(req.Params) < 4 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid params"}, "id": req.ID})
		return
	}

	userID := uint32(req.Params[0].(float64))
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid user_id"}, "id": req.ID})
		return
	}

	market, ok := req.Params[1].(string)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32602, "message": "Invalid market"}, "id": req.ID})
		return
	}

	offset := int(req.Params[2].(float64))
	limit := int(req.Params[3].(float64))

	if limit == 0 || limit > 101 {
		limit = 101
	}

	records, err := r.GetUserMarketDeals(userID, market, offset, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32603, "message": err.Error()}, "id": req.ID})
		return
	}

	result := make([]map[string]interface{}, 0, len(records))
	for _, rec := range records {
		result = append(result, map[string]interface{}{
			"id":            rec.ID,
			"time":          rec.Time,
			"user":          rec.UserID,
			"side":          rec.Side,
			"role":          rec.Role,
			"amount":        rec.Amount.String(),
			"price":         rec.Price.String(),
			"deal":          rec.Deal.String(),
			"fee":           rec.Fee.String(),
			"deal_order_id": rec.DealOrderID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result": gin.H{
			"offset":  offset,
			"limit":   limit,
			"records": result,
		},
		"id": req.ID,
	})
}
