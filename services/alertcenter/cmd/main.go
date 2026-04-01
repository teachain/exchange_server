package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"github.com/viabtc/go-project/services/alertcenter/internal/alerter"
	"github.com/viabtc/go-project/services/alertcenter/internal/server"
)

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

	viper.SetConfigFile(*configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("load config failed:", err.Error())
		os.Exit(1)
	}

	host := viper.GetString("server.host")
	port := viper.GetInt("server.port")

	redisClient := newRedisClient(viper.GetViper())
	a := alerter.NewAlerter(redisClient)
	srv := server.New(host, port, a)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		os.Exit(0)
	}()

	srv.Start()
}
