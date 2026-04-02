package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/viabtc/go-project/services/accessws/internal/config"
	"github.com/viabtc/go-project/services/accessws/internal/kafka"
	"github.com/viabtc/go-project/services/accessws/internal/logging"
	"github.com/viabtc/go-project/services/accessws/internal/server"
)

var logger *logging.Logger

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
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	setFileLimit(1000000)
	setCoreLimit(1000000000)

	logger = logging.NewLogger("logs", "accessws", 100*1024*1024, 10)
	if err := logger.Init(); err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	logger.Info("logger initialized")

	wsServer, err := server.NewWSServer(cfg)
	if err != nil {
		logger.Fatal("failed to create server: %v", err)
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
		logger.Warn("failed to start kafka consumer: %v (continuing without it)", err)
	}

	monitorServer := server.NewMonitorServer(":8082")
	go func() {
		logger.Info("Monitor server listening on :8082")
		if err := monitorServer.Start(); err != nil {
			logger.Error("Monitor server error: %v", err)
		}
	}()

	go func() {
		logger.Info("WebSocket server listening on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := wsServer.Start(); err != nil {
			logger.Fatal("server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	logger.Info("Received signal: %v", sig)
	logger.Info("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wsServer.Stop()

	kafkaConsumer.Stop()

	time.Sleep(2 * time.Second)

	if err := monitorServer.Shutdown(); err != nil {
		logger.Error("monitor server shutdown error: %v", err)
	}

	_ = ctx
	logger.Info("Shutdown complete")
}
