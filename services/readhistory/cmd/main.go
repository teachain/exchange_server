package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"

	"github.com/viabtc/go-project/internal/alert"
	"github.com/viabtc/go-project/internal/utils"
	rotatelog "github.com/viabtc/go-project/services/readhistory/internal/log"
	"github.com/viabtc/go-project/services/readhistory/internal/reader"
	"github.com/viabtc/go-project/services/readhistory/internal/server"
	"github.com/viabtc/go-project/services/readhistory/internal/server/handler"
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

func getDBPassword() string {
	if pw := os.Getenv("DB_PASSWORD"); pw != "" {
		return pw
	}
	return viper.GetString("database.password")
}

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	processName := "readhistory"
	if utils.ProcessExists(processName) {
		fmt.Println("process", processName, "already running, exiting")
		os.Exit(1)
	}

	viper.SetConfigFile(*configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("load config failed:", err.Error())
	}

	alertCfg := alert.AlertConfig{
		Host: viper.GetString("alert.host"),
		Port: viper.GetInt("alert.port"),
	}
	alerter, err := alert.NewAlerter(alertCfg)
	if err != nil {
		log.Printf("alert init failed: %v", err)
	} else {
		alerter.SendAlert("readhistory started")
	}
	defer alerter.Close()

	setFileLimit(1000000)
	setCoreLimit(1000000000)

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
		alerter.SendAlert("readhistory db connection failed: %v", err)
		log.Fatal("failed to connect db:", err)
	}
	defer db.Close()

	r := reader.New(db)
	srv := server.New(r)
	srv.SetTimeout(30 * time.Second)

	logger, err := rotatelog.NewLogger("readhistory.log")
	if err != nil {
		log.Fatal("failed to create logger:", err)
	}
	defer logger.Close()

	monitorPort := viper.GetInt("monitor.port")
	if monitorPort == 0 {
		monitorPort = 8080
	}
	monitorSrv := server.NewMonitorServer(monitorPort)
	go func() {
		log.Println("Starting monitor server on :", monitorPort)
		if err := monitorSrv.Start(); err != nil {
			log.Println("monitor server failed:", err)
		}
	}()

	handler.RegisterBalanceHandlers(srv)
	handler.RegisterOrderHandlers(srv)
	handler.RegisterMarketHandlers(srv)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Close()
		os.Exit(0)
	}()

	addr := fmt.Sprintf("%s:%d", host, port)
	log.Println("Starting RPC server on", addr)
	if err := srv.Start(addr); err != nil {
		alerter.SendAlert("readhistory server start failed: %v", err)
		log.Fatal("server failed:", err)
	}

	<-make(chan struct{})
}
