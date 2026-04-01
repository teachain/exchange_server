package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/viabtc/go-project/services/accesshttp/internal/config"
	"github.com/viabtc/go-project/services/accesshttp/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Println("load config failed:", err.Error())
		os.Exit(1)
	}

	srv := server.New(cfg)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		os.Exit(0)
	}()

	srv.Start()
}
