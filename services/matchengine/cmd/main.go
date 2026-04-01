package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
	"github.com/viabtc/go-project/services/matchengine/internal/engine"
	"github.com/viabtc/go-project/services/matchengine/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	viper.SetConfigFile(*configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("load config failed:", err.Error())
		os.Exit(1)
	}

	e := engine.NewEngine()
	e.SetBalance(1, "USDT", decimal.NewFromFloat(10000), decimal.Zero)
	e.SetBalance(1, "BTC", decimal.NewFromFloat(1), decimal.Zero)
	e.SetBalance(2, "USDT", decimal.NewFromFloat(10000), decimal.Zero)
	e.SetBalance(2, "BTC", decimal.NewFromFloat(1), decimal.Zero)

	srv := server.New(e)

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
