package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/marketprice/internal/cache"
	"github.com/viabtc/go-project/services/marketprice/internal/consumer"
	"github.com/viabtc/go-project/services/marketprice/internal/kline"
	"github.com/viabtc/go-project/services/marketprice/internal/market"
	"github.com/viabtc/go-project/services/marketprice/internal/model"
)

const (
	cmdKline            = 1
	cmdMarketStatus     = 2
	cmdMarketKline      = 3
	cmdMarketDeals      = 4
	cmdMarketWeekKline  = 5
	cmdMarketMonthKline = 6
)

type Server struct {
	Router    *gin.Engine
	km        *kline.KlineManager
	marketMgr *market.Manager
	cache     *cache.DictCache
}

func New() *Server {
	return &Server{
		Router:    gin.Default(),
		km:        kline.NewKlineManager(),
		marketMgr: market.NewManager(),
	}
}

func NewWithCache(cacheTTL time.Duration) *Server {
	return &Server{
		Router:    gin.Default(),
		km:        kline.NewKlineManager(),
		marketMgr: market.NewManager(),
		cache:     cache.NewDictCache(cacheTTL),
	}
}

func (s *Server) GetMarketManager() *market.Manager {
	return s.marketMgr
}

func (s *Server) GetKlineManager() *kline.KlineManager {
	return s.km
}

func (s *Server) GetCache() *cache.DictCache {
	return s.cache
}

func (s *Server) Start(addr string) error {
	s.SetupRoutes()
	return s.Router.Run(addr)
}

func (s *Server) SetupRoutes() {
	s.Router.GET("/kline/:market/:interval", s.handleGetKline)
	s.Router.GET("/health", s.handleHealthCheck)
	s.Router.GET("/status/:market", s.HandleMarketStatus)
	s.Router.GET("/kline/:market", s.HandleMarketKline)
	s.Router.GET("/deals/:market", s.HandleMarketDeals)
	s.Router.GET("/last/:market", s.HandleMarketLast)
	s.Router.GET("/status_today/:market", s.HandleMarketStatusToday)
	s.Router.GET("/markets", s.HandleMarketList)
	s.Router.GET("/summary/:market", s.HandleMarketSummary)
	s.Router.GET("/kline_week/:market", s.HandleMarketWeekKline)
	s.Router.GET("/kline_month/:market", s.HandleMarketMonthKline)
}

func (s *Server) setupRoutes() {
	s.SetupRoutes()
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

	cacheKey := []byte(market + ":" + interval + ":" + strconv.FormatInt(ts, 10))
	if s.cache != nil {
		if cached, ok := s.cache.Get(cmdKline, cacheKey); ok {
			c.Data(http.StatusOK, "application/json", cached)
			return
		}
	}

	k := s.km.GetKline(market, kline.Interval(interval), ts)
	if k == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "kline not found"})
		return
	}

	if s.cache != nil {
		if data, err := json.Marshal(k); err == nil {
			s.cache.Set(cmdKline, cacheKey, data)
		}
	}

	c.JSON(http.StatusOK, k)
}

func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) HandleMarketStatus(c *gin.Context) {
	marketName := c.Param("market")
	periodStr := c.DefaultQuery("period", "60")
	period, _ := strconv.ParseInt(periodStr, 10, 64)

	cacheKey := []byte(marketName + ":" + periodStr)
	if s.cache != nil {
		if cached, ok := s.cache.Get(cmdMarketStatus, cacheKey); ok {
			c.Data(http.StatusOK, "application/json", cached)
			return
		}
	}

	info, ok := s.marketMgr.Get(marketName)
	if !ok {
		c.JSON(404, gin.H{"error": "market not found"})
		return
	}

	status := s.calculateMarketStatus(info, period)

	if s.cache != nil {
		if data, err := json.Marshal(status); err == nil {
			s.cache.Set(cmdMarketStatus, cacheKey, data)
		}
	}

	c.JSON(200, status)
}

func (s *Server) HandleMarketKline(c *gin.Context) {
	marketName := c.Param("market")
	startStr := c.Query("start")
	endStr := c.Query("end")
	intervalStr := c.DefaultQuery("interval", "60")

	interval, _ := strconv.ParseInt(intervalStr, 10, 64)

	var start, end int64
	if startStr != "" {
		start, _ = strconv.ParseInt(startStr, 10, 64)
	} else {
		start = time.Now().Unix() - 3600
	}
	if endStr != "" {
		end, _ = strconv.ParseInt(endStr, 10, 64)
	} else {
		end = time.Now().Unix()
	}

	cacheKey := []byte(marketName + ":" + intervalStr + ":" + strconv.FormatInt(start, 10) + ":" + strconv.FormatInt(end, 10))
	if s.cache != nil {
		if cached, ok := s.cache.Get(cmdMarketKline, cacheKey); ok {
			c.Data(http.StatusOK, "application/json", cached)
			return
		}
	}

	info, ok := s.marketMgr.Get(marketName)
	if !ok {
		c.JSON(404, gin.H{"error": "market not found"})
		return
	}

	klines := s.getKlinesForInterval(info, interval, start, end)
	result := gin.H{"klines": klines}

	if s.cache != nil {
		if data, err := json.Marshal(result); err == nil {
			s.cache.Set(cmdMarketKline, cacheKey, data)
		}
	}

	c.JSON(200, result)
}

func (s *Server) HandleMarketDeals(c *gin.Context) {
	marketName := c.Param("market")
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	lastID, _ := strconv.ParseInt(c.DefaultQuery("last_id", "0"), 10, 64)

	cacheKey := []byte(marketName + ":" + strconv.FormatInt(limit, 10) + ":" + strconv.FormatInt(lastID, 10))
	if s.cache != nil {
		if cached, ok := s.cache.Get(cmdMarketDeals, cacheKey); ok {
			c.Data(http.StatusOK, "application/json", cached)
			return
		}
	}

	info, ok := s.marketMgr.Get(marketName)
	if !ok {
		c.JSON(404, gin.H{"error": "market not found"})
		return
	}

	deals := s.getDealsAfterID(info, lastID, limit)
	result := gin.H{"deals": deals}

	if s.cache != nil {
		if data, err := json.Marshal(result); err == nil {
			s.cache.Set(cmdMarketDeals, cacheKey, data)
		}
	}

	c.JSON(200, result)
}

func (s *Server) HandleMarketLast(c *gin.Context) {
	marketName := c.Param("market")
	info, ok := s.marketMgr.Get(marketName)
	if !ok {
		c.JSON(404, gin.H{"error": "market not found"})
		return
	}
	c.JSON(200, gin.H{"last": info.LastPrice.String()})
}

func (s *Server) HandleMarketStatusToday(c *gin.Context) {
	marketName := c.Param("market")
	info, ok := s.marketMgr.Get(marketName)
	if !ok {
		c.JSON(404, gin.H{"error": "market not found"})
		return
	}

	todayStart := time.Now().Truncate(24 * time.Hour).Unix()
	status := s.calculateMarketStatus(info, todayStart)
	c.JSON(200, status)
}

func (s *Server) HandleMarketList(c *gin.Context) {
	markets := s.marketMgr.ListMarkets()
	c.JSON(200, gin.H{"markets": markets})
}

func (s *Server) HandleMarketSummary(c *gin.Context) {
	marketName := c.Param("market")
	info, ok := s.marketMgr.Get(marketName)
	if !ok {
		c.JSON(404, gin.H{"error": "market not found"})
		return
	}
	summary := s.calculateMarketSummary(info)
	c.JSON(200, summary)
}

func (s *Server) calculateMarketStatus(info *model.MarketInfo, period int64) *model.MarketStatus {
	now := time.Now().Unix()
	startTime := now - period

	var open, close, high, low decimal.Decimal
	var volume, deal decimal.Decimal
	first := true

	for _, dealItem := range info.Deals {
		if dealItem.Time < float64(startTime) {
			continue
		}
		if first {
			open = dealItem.Price
			high = dealItem.Price
			low = dealItem.Price
			first = false
		}
		close = dealItem.Price
		if dealItem.Price.GreaterThan(high) {
			high = dealItem.Price
		}
		if dealItem.Price.LessThan(low) {
			low = dealItem.Price
		}
		volume = volume.Add(dealItem.Amount)
		deal = deal.Add(dealItem.Price.Mul(dealItem.Amount))
	}

	if first {
		open = info.LastPrice
		close = info.LastPrice
		high = info.LastPrice
		low = info.LastPrice
	}

	return &model.MarketStatus{
		Period: period,
		Last:   info.LastPrice,
		Open:   open,
		Close:  close,
		High:   high,
		Low:    low,
		Volume: volume,
		Deal:   deal,
	}
}

func (s *Server) getKlinesForInterval(info *model.MarketInfo, interval, start, end int64) []*model.KlineInfo {
	var klines []*model.KlineInfo

	var klineMap map[int64]*model.KlineInfo
	switch interval {
	case 1:
		klineMap = info.SecKlines
	case 60:
		klineMap = info.MinKlines
	case 3600:
		klineMap = info.HourKlines
	case 86400:
		klineMap = info.DayKlines
	default:
		klineMap = info.MinKlines
	}

	for ts, k := range klineMap {
		if ts >= start && ts <= end {
			klines = append(klines, k)
		}
	}

	return klines
}

func (s *Server) getDealsAfterID(info *model.MarketInfo, lastID, limit int64) []*model.Deal {
	var result []*model.Deal
	count := 0
	for i := len(info.Deals) - 1; i >= 0 && count < int(limit); i-- {
		if info.Deals[i].ID > lastID {
			result = append(result, info.Deals[i])
			count++
		}
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func (s *Server) calculateMarketSummary(info *model.MarketInfo) gin.H {
	var totalVolume, totalDeal decimal.Decimal
	for _, k := range info.DayKlines {
		totalVolume = totalVolume.Add(k.Volume)
		totalDeal = totalDeal.Add(k.Deal)
	}

	open := decimal.Zero
	close := info.LastPrice
	high := decimal.Zero
	low := decimal.Zero

	if len(info.Deals) > 0 {
		open = info.Deals[0].Price
		high = info.Deals[0].Price
		low = info.Deals[0].Price
		for _, d := range info.Deals {
			if d.Price.GreaterThan(high) {
				high = d.Price
			}
			if d.Price.LessThan(low) {
				low = d.Price
			}
		}
	}

	return gin.H{
		"market":  info.Name,
		"open":    open.String(),
		"close":   close.String(),
		"high":    high.String(),
		"low":     low.String(),
		"volume":  totalVolume.String(),
		"deal":    totalDeal.String(),
		"period":  86400,
		"periods": []int64{60, 300, 900, 3600, 14400, 86400},
	}
}

func (s *Server) HandleMarketWeekKline(c *gin.Context) {
	marketName := c.Param("market")
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

	info, ok := s.marketMgr.Get(marketName)
	if !ok {
		c.JSON(404, gin.H{"error": "market not found"})
		return
	}

	kline := s.km.GetWeekKline(marketName, ts, time.UTC)
	if kline == nil {
		c.JSON(404, gin.H{"error": "week kline not found"})
		return
	}

	c.JSON(200, gin.H{
		"market":   info.Name,
		"interval": "1w",
		"kline":    kline,
	})
}

func (s *Server) HandleMarketMonthKline(c *gin.Context) {
	marketName := c.Param("market")
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

	info, ok := s.marketMgr.Get(marketName)
	if !ok {
		c.JSON(404, gin.H{"error": "market not found"})
		return
	}

	kline := s.km.GetMonthKline(marketName, ts, time.UTC)
	if kline == nil {
		c.JSON(404, gin.H{"error": "month kline not found"})
		return
	}

	c.JSON(200, gin.H{
		"market":   info.Name,
		"interval": "1M",
		"kline":    kline,
	})
}

func (s *Server) StartConsumer(brokers []string, group string, topic string, redisAddr string, redisPassword string, partition int32) {
	dealConsumer, err := consumer.NewDealConsumer(brokers, group, func(deal *consumer.Deal) {
		price, _ := decimal.NewFromString(deal.Price)
		amount, _ := decimal.NewFromString(deal.Amount)
		s.km.AddDeal(deal.Market, price, amount, deal.CreatedAt)
		s.marketMgr.AddDeal(s.marketMgr.GetOrCreate(deal.Market), &model.Deal{
			ID:     deal.ID,
			Time:   float64(deal.CreatedAt),
			Price:  price,
			Amount: amount,
			Side:   deal.Side,
		})
	}, redisAddr, redisPassword)

	if err != nil {
		panic("create deal consumer failed: " + err.Error())
	}

	if err := dealConsumer.Start(topic, partition); err != nil {
		panic("start deal consumer failed: " + err.Error())
	}
}
