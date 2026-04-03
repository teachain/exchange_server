package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/spf13/viper"
	"github.com/teachain/exchange_server/internal/alert"
	"github.com/teachain/exchange_server/internal/utils"
	"github.com/teachain/exchange_server/services/marketprice/internal/cache"
	appLog "github.com/teachain/exchange_server/services/marketprice/internal/log"
	"github.com/teachain/exchange_server/services/marketprice/internal/market"
	"github.com/teachain/exchange_server/services/marketprice/internal/server"
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

	processName := "marketprice"
	if utils.ProcessExists(processName) {
		fmt.Println("process", processName, "already running, exiting")
		os.Exit(1)
	}

	viper.SetConfigFile(*configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("load config failed:", err.Error())
		os.Exit(1)
	}

	debug := viper.GetBool("debug")
	if debug {
		log.Printf("DEBUG MODE ENABLED")
	}

	alertCfg := alert.AlertConfig{
		Host: viper.GetString("alert.host"),
		Port: viper.GetInt("alert.port"),
	}
	alerter, err := alert.NewAlerter(alertCfg)
	if err != nil {
		fmt.Println("alert init failed:", err.Error())
	} else {
		alerter.SendAlert("marketprice started")
	}
	defer alerter.Close()

	setFileLimit(1000000)
	setCoreLimit(1000000000)

	logger, err := appLog.NewLogger(appLog.LoggerConfig{
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
	redisDB := viper.GetInt("redis.db")
	redisAddr := fmt.Sprintf("%s:%d", redisHost, redisPort)

	sentinelEnabled := viper.GetBool("redis.sentinel.enabled")
	sentinelMaster := viper.GetString("redis.sentinel.master")
	sentinelAddrs := viper.GetStringSlice("redis.sentinel.addrs")

	var redisCache *cache.RedisCache
	var redisErr error

	if sentinelEnabled && len(sentinelAddrs) > 0 {
		redisCache, redisErr = cache.NewRedisCacheWithSentinel(sentinelMaster, sentinelAddrs, redisPassword, redisDB)
	} else {
		redisCache, redisErr = cache.NewRedisCacheWithPassword(redisAddr, redisPassword, redisDB)
	}

	if redisErr != nil {
		alerter.SendAlert("redis connection failed: %v", redisErr)
		fmt.Println("create redis cache failed:", redisErr.Error())
		os.Exit(1)
	}

	cacheTimeout := viper.GetFloat64("cache.timeout")
	if cacheTimeout <= 0 {
		cacheTimeout = 0.45
	}
	srv = server.NewWithCache(time.Duration(cacheTimeout*float64(time.Second)), debug)

	go srv.StartConsumer(brokers, group, topic, redisAddr, redisPassword, partition, debug)

	marketMgr := srv.GetMarketManager()
	startFlushTimer(debug, marketMgr, redisCache)
	startClearTimer(debug, marketMgr, 86400, 604800, 2592000)

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
	if err := srv.Start(addr); err != nil {
		alerter.SendAlert("server start failed: %v", err)
	}
}

func startFlushTimer(debug bool, marketMgr *market.Manager, cache *cache.RedisCache) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			if debug {
				log.Printf("[DEBUG] flush timer triggered")
			}
			marketMgr.FlushDirty(cache)
		}
	}()
}

func startClearTimer(debug bool, marketMgr *market.Manager, secMax, minMax, hourMax int64) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			if debug {
				log.Printf("[DEBUG] clear timer triggered")
			}
			marketMgr.ClearOldKlines(secMax, minMax, hourMax)
		}
	}()
}
