package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
	"golang.org/x/sys/unix"

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

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		os.Exit(0)
	}()

	srv.Start()
}
