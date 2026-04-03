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
	"github.com/viabtc/go-project/internal/alert"
	"github.com/viabtc/go-project/internal/utils"
	"github.com/viabtc/go-project/services/marketprice/internal/cache"
	appLog "github.com/viabtc/go-project/services/marketprice/internal/log"
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
	redisAddr := fmt.Sprintf("%s:%d", redisHost, redisPort)

	redisCache, err := cache.NewRedisCacheWithPassword(redisAddr, redisPassword, 0)
	if err != nil {
		alerter.SendAlert("redis connection failed: %v", err)
		fmt.Println("create redis cache failed:", err.Error())
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
