package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/spf13/viper"
	"github.com/viabtc/go-project/services/marketprice/internal/cache"
	"github.com/viabtc/go-project/services/marketprice/internal/log"
	"github.com/viabtc/go-project/services/marketprice/internal/market"
	"github.com/viabtc/go-project/services/marketprice/internal/server"
)

func setFileLimit(max uint64) {
	var rlimit unix.Rlimit
	rlimit.Cur = max
	rlimit.Max = max
	unix.Setrlimit(unix.RLIMIT_NOFILE, &rlimit)
}

func setCoreLimit(max uint64) {
	var rlimit unix.Rlimit
	rlimit.Cur = max
	rlimit.Max = max
	unix.Setrlimit(unix.RLIMIT_CORE, &rlimit)
}

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	viper.SetConfigFile(*configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("load config failed:", err.Error())
		os.Exit(1)
	}

	setFileLimit(1000000)
	setCoreLimit(1000000000)

	logger, err := log.NewLogger(log.LoggerConfig{
		Filename: viper.GetString("log.file"),
		MaxSize:  int64(viper.GetInt("log.max_size")),
		MaxFiles: viper.GetInt("log.max_files"),
	})
	if err != nil {
		fmt.Println("create logger failed:", err.Error())
		os.Exit(1)
	}
	defer logger.Close()

	srv := server.New()

	monitorPort := viper.GetInt("monitor.port")
	monitorSrv := server.NewMonitorServer(monitorPort, srv.GetMarketManager(), srv.GetKlineManager())
	go monitorSrv.Start()

	brokers := viper.GetStringSlice("kafka.brokers")
	topic := viper.GetString("kafka.topic")
	group := viper.GetString("kafka.consumer.group")
	partition := int32(viper.GetInt("kafka.consumer.partition"))
	redisHost := viper.GetString("redis.host")
	redisPort := viper.GetInt("redis.port")
	redisPassword := viper.GetString("redis.password")
	redisAddr := fmt.Sprintf("%s:%d", redisHost, redisPort)

	redisCache, err := cache.NewRedisCacheWithPassword(redisAddr, redisPassword, 0)
	if err != nil {
		fmt.Println("create redis cache failed:", err.Error())
		os.Exit(1)
	}

	go srv.StartConsumer(brokers, group, topic, redisAddr, redisPassword, partition)

	marketMgr := srv.GetMarketManager()
	startFlushTimer(marketMgr, redisCache)
	startClearTimer(marketMgr, 86400, 604800, 2592000)

	host := viper.GetString("server.host")
	port := viper.GetInt("server.port")
	addr := fmt.Sprintf("%s:%d", host, port)

	shutdownCh := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		close(shutdownCh)
	}()

	go func() {
		<-shutdownCh
		logger.Write([]byte("shutting down...\n"))
		marketMgr.FlushDirty(redisCache)
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}()

	logger.Write([]byte("server started\n"))
	srv.Start(addr)
}

func startFlushTimer(marketMgr *market.Manager, cache *cache.RedisCache) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			marketMgr.FlushDirty(cache)
		}
	}()
}

func startClearTimer(marketMgr *market.Manager, secMax, minMax, hourMax int64) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			marketMgr.ClearOldKlines(secMax, minMax, hourMax)
		}
	}()
}
