package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"github.com/viabtc/go-project/internal/utils"
	"github.com/viabtc/go-project/services/alertcenter/internal/alerter"
	rotatelog "github.com/viabtc/go-project/services/alertcenter/internal/log"
	"github.com/viabtc/go-project/services/alertcenter/internal/server"
	"golang.org/x/sys/unix"
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

func newRedisClient(cfg *viper.Viper) redis.Cmdable {
	sentinelEnabled := cfg.GetBool("redis.sentinel.enabled")
	sentinelMaster := cfg.GetString("redis.sentinel.master")
	sentinelAddrs := cfg.GetStringSlice("redis.sentinel.addrs")
	password := cfg.GetString("redis.password")
	db := cfg.GetInt("redis.db")

	if sentinelEnabled && len(sentinelAddrs) > 0 {
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    sentinelMaster,
			SentinelAddrs: sentinelAddrs,
			Password:      password,
			DB:            db,
		})
	}

	addr := fmt.Sprintf("%s:%d", cfg.GetString("redis.host"), cfg.GetInt("redis.port"))
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	processName := "alertcenter"
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

	logFile := viper.GetString("log.file")
	maxSize := viper.GetInt64("log.max_size")
	maxBackups := viper.GetInt("log.max_backups")
	maxAge := viper.GetInt("log.max_age")

	if maxSize == 0 {
		maxSize = 100 * 1024 * 1024
	}
	if maxBackups == 0 {
		maxBackups = 7
	}
	if maxAge == 0 {
		maxAge = 7
	}

	rotatelog.InitLogger(logFile, maxSize, maxBackups, maxAge)

	setFileLimit(1000000)
	setCoreLimit(1000000000)

	host := viper.GetString("server.host")
	port := viper.GetInt("server.port")

	redisClient := newRedisClient(viper.GetViper())
	a := alerter.NewAlerter(redisClient)
	emailSender := alerter.NewEmailSender(viper.GetViper())
	consumer := alerter.NewConsumer(redisClient, emailSender, 60*time.Second)
	srv := server.New(host, port, a)

	ctx := context.Background()
	go consumer.Start(ctx)

	stopCh := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)

		go srv.Stop()
		consumer.Stop()

		shutdownTimeout := time.Duration(viper.GetInt("server.shutdown_timeout")) * time.Second
		if shutdownTimeout == 0 {
			shutdownTimeout = 30 * time.Second
		}

		select {
		case <-stopCh:
		case <-time.After(shutdownTimeout):
			log.Printf("Shutdown timeout reached, forcing exit")
		}

		os.Exit(0)
	}()

	srv.Start()
}
