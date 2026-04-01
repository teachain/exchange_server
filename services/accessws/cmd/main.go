package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/viabtc/go-project/services/accessws/internal/config"
	"github.com/viabtc/go-project/services/accessws/internal/kafka"
	"github.com/viabtc/go-project/services/accessws/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	wsServer, err := server.NewWSServer(cfg)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	kafkaConsumer := kafka.NewConsumer(
		cfg.Kafka.Brokers,
		cfg.Kafka.Consumer.Group,
		cfg.Kafka.Consumer.OrdersTopic,
		cfg.Kafka.Consumer.BalancesTopic,
		wsServer.SubMgr(),
		wsServer.RPCClient(),
	)

	if err := kafkaConsumer.Start(); err != nil {
		log.Printf("failed to start kafka consumer: %v (continuing without it)", err)
	}

	go func() {
		if err := wsServer.Start(); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")
}
