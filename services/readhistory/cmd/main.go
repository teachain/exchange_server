package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
	"github.com/viabtc/go-project/services/readhistory/internal/reader"
	"github.com/viabtc/go-project/services/readhistory/internal/server"
	"github.com/viabtc/go-project/services/readhistory/internal/server/handler"
)

func getDBPassword() string {
	if pw := os.Getenv("DB_PASSWORD"); pw != "" {
		return pw
	}
	return viper.GetString("database.password")
}

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	viper.SetConfigFile(*configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("load config failed:", err.Error())
	}

	host := viper.GetString("server.host")
	port := viper.GetInt("server.port")
	dbHost := viper.GetString("database.host")
	dbPort := viper.GetInt("database.port")
	dbName := viper.GetString("database.name")
	dbUser := viper.GetString("database.username")
	dbPass := getDBPassword()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", dbUser, dbPass, dbHost, dbPort, dbName)
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		log.Fatal("failed to connect db:", err)
	}
	defer db.Close()

	r := reader.New(db)
	srv := server.New(r)

	handler.RegisterBalanceHandlers(srv)
	handler.RegisterOrderHandlers(srv)
	handler.RegisterMarketHandlers(srv)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		os.Exit(0)
	}()

	addr := fmt.Sprintf("%s:%d", host, port)
	log.Println("Starting RPC server on", addr)
	if err := srv.Start(addr); err != nil {
		log.Fatal("server failed:", err)
	}
}
