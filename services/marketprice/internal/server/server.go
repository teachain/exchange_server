package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/marketprice/internal/consumer"
	"github.com/viabtc/go-project/services/marketprice/internal/kline"
)

type Server struct {
	router *gin.Engine
	km     *kline.KlineManager
}

func New() *Server {
	return &Server{
		router: gin.Default(),
		km:     kline.NewKlineManager(),
	}
}

func (s *Server) Start(addr string) error {
	s.setupRoutes()
	return s.router.Run(addr)
}

func (s *Server) setupRoutes() {
	s.router.GET("/kline/:market/:interval", s.handleGetKline)
	s.router.GET("/health", s.handleHealthCheck)
}

func (s *Server) handleGetKline(c *gin.Context) {
	market := c.Param("market")
	interval := c.Param("interval")
	tsStr := c.Query("ts")

	var ts int64
	if tsStr != "" {
		var err error
		ts, err = strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid timestamp"})
			return
		}
	} else {
		ts = time.Now().Unix()
	}

	k := s.km.GetKline(market, kline.Interval(interval), ts)
	if k == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "kline not found"})
		return
	}

	c.JSON(http.StatusOK, k)
}

func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) StartConsumer(brokers []string, group string, topic string, redisAddr string) {
	dealConsumer, err := consumer.NewDealConsumer(brokers, group, func(deal *consumer.Deal) {
		price, _ := decimal.NewFromString(deal.Price)
		amount, _ := decimal.NewFromString(deal.Amount)
		s.km.AddDeal(deal.Market, price, amount, deal.CreatedAt)
	}, redisAddr)

	if err != nil {
		panic("create deal consumer failed: " + err.Error())
	}

	if err := dealConsumer.Start(topic); err != nil {
		panic("start deal consumer failed: " + err.Error())
	}
}
