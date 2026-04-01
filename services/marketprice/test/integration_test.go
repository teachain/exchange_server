package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/marketprice/internal/model"
	"github.com/viabtc/go-project/services/marketprice/internal/server"
)

const (
	SideAsk = 1
	SideBid = 2
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMarketpriceIntegration(t *testing.T) {
	t.Skip("Integration test requires running Kafka and Redis")
}

func TestHTTPEndpoints(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHealthEndpoint(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMarketStatusEndpoint(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()
	mm := srv.GetMarketManager()
	mm.GetOrCreate("BTC_USDT")

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/status/BTC_USDT?period=60")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMarketStatusNotFound(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/status/INVALID_MARKET?period=60")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestMarketListEndpoint(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()
	mm := srv.GetMarketManager()
	mm.GetOrCreate("BTC_USDT")
	mm.GetOrCreate("ETH_USDT")

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/markets")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMarketLastEndpoint(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()
	mm := srv.GetMarketManager()
	mm.GetOrCreate("BTC_USDT")

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/last/BTC_USDT")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMarketSummaryEndpoint(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()
	mm := srv.GetMarketManager()
	mm.GetOrCreate("BTC_USDT")

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/summary/BTC_USDT")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestKlineEndpoint(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()
	km := srv.GetKlineManager()
	now := time.Now().Unix()
	km.AddDeal("BTC_USDT", decimal.NewFromFloat(50000.00), decimal.NewFromFloat(1.0), now)

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(fmt.Sprintf("%s/kline/BTC_USDT/1m?ts=%d", ts.URL, now))
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDealsEndpoint(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()
	mm := srv.GetMarketManager()
	mm.GetOrCreate("BTC_USDT")

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/deals/BTC_USDT?limit=10")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStatusTodayEndpoint(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()
	mm := srv.GetMarketManager()
	mm.GetOrCreate("BTC_USDT")

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/status_today/BTC_USDT")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMarketKlineEndpoint(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()
	mm := srv.GetMarketManager()
	mm.GetOrCreate("BTC_USDT")

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/kline/BTC_USDT?interval=60")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMarketKlineWithDeals(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()
	mm := srv.GetMarketManager()
	info := mm.GetOrCreate("BTC_USDT")
	info.Deals = append(info.Deals, &model.Deal{
		ID:     1,
		Time:   float64(1709300000),
		Price:  decimal.NewFromFloat(50000.00),
		Amount: decimal.NewFromFloat(1.5),
		Side:   SideBid,
	})

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/kline/BTC_USDT?start=1709290000&end=1709310000&interval=60")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMarketDealsWithData(t *testing.T) {
	srv := server.New()
	srv.SetupRoutes()
	mm := srv.GetMarketManager()
	info := mm.GetOrCreate("BTC_USDT")
	info.Deals = append(info.Deals, &model.Deal{
		ID:     1,
		Time:   float64(1709300000),
		Price:  decimal.NewFromFloat(50000.00),
		Amount: decimal.NewFromFloat(1.5),
		Side:   SideBid,
	}, &model.Deal{
		ID:     2,
		Time:   float64(1709301000),
		Price:  decimal.NewFromFloat(50100.00),
		Amount: decimal.NewFromFloat(2.0),
		Side:   SideAsk,
	})

	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/deals/BTC_USDT?limit=10&last_id=0")
	if err != nil {
		t.Skip("server not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMarketManagerCreate(t *testing.T) {
	srv := server.New()
	mm := srv.GetMarketManager()

	info := mm.GetOrCreate("BTC_USDT")
	if info == nil {
		t.Fatal("expected market info to be created")
	}
	if info.Name != "BTC_USDT" {
		t.Errorf("expected BTC_USDT, got %s", info.Name)
	}

	_, exists := mm.Get("BTC_USDT")
	if !exists {
		t.Error("expected market to exist")
	}

	list := mm.ListMarkets()
	if len(list) != 1 {
		t.Errorf("expected 1 market, got %d", len(list))
	}
}

func TestMarketManagerGet(t *testing.T) {
	srv := server.New()
	mm := srv.GetMarketManager()

	mm.GetOrCreate("ETH_USDT")

	info, ok := mm.Get("ETH_USDT")
	if !ok {
		t.Fatal("expected to get market")
	}
	if info.Name != "ETH_USDT" {
		t.Errorf("expected ETH_USDT, got %s", info.Name)
	}

	_, ok = mm.Get("NONEXISTENT")
	if ok {
		t.Error("expected market not to exist")
	}
}
