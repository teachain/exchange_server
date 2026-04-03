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

	"github.com/spf13/viper"
	"golang.org/x/sys/unix"

	"github.com/viabtc/go-project/internal/utils"
	"github.com/viabtc/go-project/services/accesshttp/internal/config"
	"github.com/viabtc/go-project/services/accesshttp/internal/server"
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

	processName := "accesshttp"
	if utils.ProcessExists(processName) {
		fmt.Println("process", processName, "already running, exiting")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Println("load config failed:", err.Error())
		os.Exit(1)
	}

	setFileLimit(1000000)
	setCoreLimit(1000000000)

	srv := server.New(cfg)

	monitorAddr := viper.GetString("monitor.bind")
	if monitorAddr == "" {
		monitorAddr = ":8081"
	}
	monitorServer := server.NewMonitorServer(monitorAddr)
	go func() {
		log.Printf("Monitor server listening on %s", monitorAddr)
		if err := monitorServer.Start(); err != nil {
			log.Printf("Monitor server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := srv.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	sig := <-sigCh
	log.Printf("Received signal: %v", sig)
	log.Println("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	time.Sleep(5 * time.Second)

	if err := monitorServer.Shutdown(); err != nil {
		log.Printf("Monitor shutdown error: %v", err)
	}

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Shutdown complete")
}
