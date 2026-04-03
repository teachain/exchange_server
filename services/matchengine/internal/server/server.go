package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/teachain/exchange_server/services/matchengine/internal/engine"
	"github.com/teachain/exchange_server/services/matchengine/internal/order"
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
	s.router.GET("/asset/list", s.handleAssetList)
	s.router.GET("/asset/summary", s.handleAssetSummary)
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

type CreateOrderRequest struct {
	UserID uint32 `json:"user_id" binding:"required"`
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
		ID:         s.engine.NextID(),
		UserID:     req.UserID,
		Market:     req.Market,
		Side:       side,
		Price:      price,
		Amount:     amount,
		Left:       amount,
		Status:     order.OrderStatusPending,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
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
	OrderID uint64 `json:"order_id" binding:"required"`
	Market  string `json:"market" binding:"required"`
	UserID  uint32 `json:"user_id" binding:"required"`
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
	id, err := strconv.ParseUint(idStr, 10, 64)
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
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	asset := c.Param("asset")
	if asset == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "asset is required"})
		return
	}

	balance, frozen := s.engine.GetBalance(uint32(userID), asset)

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

	bids := ob.GetDepth(20, order.SideBid)
	asks := ob.GetDepth(20, order.SideAsk)

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
		return order.SideBid, nil
	case "sell":
		return order.SideAsk, nil
	default:
		return 0, order.ErrInvalidSide
	}
}

func (s *Server) handleAssetList(c *gin.Context) {
	assets := s.engine.GetAllAssets()

	result := make([]gin.H, 0, len(assets))
	for _, asset := range assets {
		result = append(result, gin.H{
			"name": asset.Name,
			"prec": asset.Prec,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"assets": result,
	})
}

func (s *Server) handleAssetSummary(c *gin.Context) {
	assets := s.engine.GetAllAssets()

	result := make([]gin.H, 0, len(assets))
	for _, asset := range assets {
		balances := s.engine.GetAllBalancesForAsset(asset.Name)

		var totalBalance, availableBalance, freezeBalance decimal.Decimal
		var availableCount, freezeCount int

		for _, bal := range balances {
			totalBalance = totalBalance.Add(bal.Available).Add(bal.Frozen)
			if bal.Frozen.IsZero() {
				availableCount++
				availableBalance = availableBalance.Add(bal.Available)
			} else {
				freezeCount++
				freezeBalance = freezeBalance.Add(bal.Frozen)
			}
		}

		result = append(result, gin.H{
			"name":              asset.Name,
			"total_balance":     adjustPrecision(totalBalance, asset.Prec).String(),
			"available_count":   availableCount,
			"available_balance": adjustPrecision(availableBalance, asset.Prec).String(),
			"freeze_count":      freezeCount,
			"freeze_balance":    adjustPrecision(freezeBalance, asset.Prec).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"assets": result,
	})
}

func adjustPrecision(d decimal.Decimal, prec int) decimal.Decimal {
	if prec <= 0 {
		return d
	}
	multiplier := decimal.NewFromInt(10)
	for i := 0; i < prec; i++ {
		multiplier = multiplier.Mul(decimal.NewFromInt(10))
	}
	truncated := d.Mul(multiplier).Floor().Div(multiplier)
	return truncated
}
