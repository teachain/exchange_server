package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/matchengine/internal/engine"
	"github.com/viabtc/go-project/services/matchengine/internal/order"
)

type Server struct {
	router *gin.Engine
	engine *engine.Engine
}

func New(e *engine.Engine) *Server {
	return &Server{
		router: gin.Default(),
		engine: e,
	}
}

func (s *Server) Start(addr string) error {
	s.setupRoutes()
	return s.router.Run(addr)
}

func (s *Server) setupRoutes() {
	s.router.GET("/health", s.handleHealth)
	s.router.POST("/order/create", s.handleCreateOrder)
	s.router.POST("/order/cancel", s.handleCancelOrder)
	s.router.GET("/order/:id", s.handleGetOrder)
	s.router.GET("/balance/:user_id/:asset", s.handleGetBalance)
	s.router.GET("/depth/:market", s.handleGetDepth)
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

type CreateOrderRequest struct {
	UserID int64  `json:"user_id" binding:"required"`
	Market string `json:"market" binding:"required"`
	Side   string `json:"side" binding:"required"`
	Price  string `json:"price" binding:"required"`
	Amount string `json:"amount" binding:"required"`
}

func (s *Server) handleCreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	side, err := parseSide(req.Side)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid side"})
		return
	}

	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid price"})
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount"})
		return
	}

	if price.LessThanOrEqual(decimal.Zero) || amount.LessThanOrEqual(decimal.Zero) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "price and amount must be positive"})
		return
	}

	incoming := &order.Order{
		ID:        s.engine.NextID(),
		UserID:    req.UserID,
		Market:    req.Market,
		Side:      side,
		Price:     price,
		Amount:    amount,
		Deal:      decimal.Zero,
		Status:    order.OrderStatusPending,
		CreatedAt: time.Now(),
	}

	trades, err := s.engine.ProcessOrder(incoming)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order":  incoming,
		"trades": trades,
	})
}

type CancelOrderRequest struct {
	OrderID int64  `json:"order_id" binding:"required"`
	Market  string `json:"market" binding:"required"`
	UserID  int64  `json:"user_id" binding:"required"`
}

func (s *Server) handleCancelOrder(c *gin.Context) {
	var req CancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.engine.CancelOrder(req.OrderID, req.Market)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleGetOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	ord, found := s.engine.GetOrder(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	c.JSON(http.StatusOK, ord)
}

func (s *Server) handleGetBalance(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	asset := c.Param("asset")
	if asset == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "asset is required"})
		return
	}

	balance, frozen := s.engine.GetBalance(userID, asset)

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"asset":   asset,
		"balance": balance.String(),
		"frozen":  frozen.String(),
	})
}

type DepthLevel struct {
	Price  string `json:"price"`
	Amount string `json:"amount"`
}

func (s *Server) handleGetDepth(c *gin.Context) {
	market := c.Param("market")
	if market == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "market is required"})
		return
	}

	ob, found := s.engine.GetOrderBook(market)
	if !found {
		c.JSON(http.StatusOK, gin.H{
			"bids": []DepthLevel{},
			"asks": []DepthLevel{},
		})
		return
	}

	bids := ob.GetDepth(20, order.SideBuy)
	asks := ob.GetDepth(20, order.SideSell)

	bidsResp := make([]DepthLevel, len(bids))
	for i, d := range bids {
		bidsResp[i] = DepthLevel{Price: d.Price.String(), Amount: d.Amount.String()}
	}

	asksResp := make([]DepthLevel, len(asks))
	for i, d := range asks {
		asksResp[i] = DepthLevel{Price: d.Price.String(), Amount: d.Amount.String()}
	}

	c.JSON(http.StatusOK, gin.H{
		"bids": bidsResp,
		"asks": asksResp,
	})
}

func parseSide(s string) (order.Side, error) {
	switch s {
	case "buy":
		return order.SideBuy, nil
	case "sell":
		return order.SideSell, nil
	default:
		return 0, order.ErrInvalidSide
	}
}
