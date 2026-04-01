package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
	"github.com/viabtc/go-project/services/marketprice/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	viper.SetConfigFile(*configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("load config failed:", err.Error())
		os.Exit(1)
	}

	srv := server.New()

	brokers := viper.GetStringSlice("kafka.brokers")
	topic := viper.GetString("kafka.consumer.topic")
	group := viper.GetString("kafka.consumer.group")
	redisHost := viper.GetString("redis.host")
	redisPort := viper.GetInt("redis.port")
	redisAddr := fmt.Sprintf("%s:%d", redisHost, redisPort)

	go srv.StartConsumer(brokers, group, topic, redisAddr)

	host := viper.GetString("server.host")
	port := viper.GetInt("server.port")
	addr := fmt.Sprintf("%s:%d", host, port)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		os.Exit(0)
	}()

	srv.Start(addr)
}
