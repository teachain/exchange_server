package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
	"github.com/viabtc/go-project/internal/alert"
	"github.com/viabtc/go-project/internal/utils"
	"github.com/viabtc/go-project/services/matchengine/internal/cli"
	"github.com/viabtc/go-project/services/matchengine/internal/engine"
	"github.com/viabtc/go-project/services/matchengine/internal/history"
	"github.com/viabtc/go-project/services/matchengine/internal/persist"
	"github.com/viabtc/go-project/services/matchengine/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	processName := "matchengine"
	if utils.ProcessExists(processName) {
		fmt.Println("process", processName, "already running, exiting")
		os.Exit(1)
	}

	viper.SetConfigFile(*configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("load config failed:", err.Error())
		os.Exit(1)
	}

	alertCfg := alert.AlertConfig{
		Host: viper.GetString("alert.host"),
		Port: viper.GetInt("alert.port"),
	}
	alerter, err := alert.NewAlerter(alertCfg)
	if err != nil {
		fmt.Printf("alert init failed: %v\n", err)
	} else {
		alerter.SendAlert("matchengine started")
	}
	defer func() {
		if alerter != nil {
			alerter.SendAlert("matchengine stopped")
			alerter.Close()
		}
	}()

	e := engine.NewEngine()

	dbHost := viper.GetString("database.host")
	dbPort := viper.GetInt("database.port")
	dbName := viper.GetString("database.name")
	dbUser := viper.GetString("database.username")
	dbPass := viper.GetString("database.password")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True",
		dbUser, dbPass, dbHost, dbPort, dbName)
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		fmt.Println("connect to database failed:", err.Error())
		if alerter != nil {
			alerter.SendAlert("matchengine database connection failed: %v", err)
		}
		os.Exit(1)
	}
	defer db.Close()

	sliceInterval := viper.GetDuration("slice_interval")
	if sliceInterval == 0 {
		sliceInterval = time.Minute
	}
	sliceKeepTime := viper.GetDuration("slice_keep_time")
	if sliceKeepTime == 0 {
		sliceKeepTime = 24 * time.Hour
	}
	sliceDir := viper.GetString("slice_dir")
	if sliceDir == "" {
		sliceDir = "./slices"
	}

	sm := persist.NewSliceManager(db, e, sliceInterval, sliceKeepTime, sliceDir)
	if err := sm.InitDB(); err != nil {
		fmt.Println("init slice db failed:", err.Error())
		if alerter != nil {
			alerter.SendAlert("matchengine init slice db failed: %v", err)
		}
		os.Exit(1)
	}

	operLogWriter := persist.NewOperLogWriter(db)
	e.SetOperLogWriter(operLogWriter)

	if err := InitFromDB(db, e, sm, operLogWriter); err != nil {
		fmt.Println("init from db failed:", err.Error())
		if alerter != nil {
			alerter.SendAlert("matchengine init from db failed: %v", err)
		}
		os.Exit(1)
	}

	go sm.StartPeriodicSlices(sliceInterval, e)

	historyWriter := history.NewHistoryWriter(db, "matchengine")
	historyWriter.Start()
	e.SetHistoryWriter(historyWriter)

	startStopOrderCron(e)

	srv := server.New(e)

	host := viper.GetString("server.host")
	port := viper.GetInt("server.port")
	addr := fmt.Sprintf("%s:%d", host, port)

	cliHost := viper.GetString("cli.host")
	cliPort := viper.GetInt("cli.port")
	cliAddr := fmt.Sprintf("%s:%d", cliHost, cliPort)
	cliServer := cli.NewCLI(cliAddr, e)
	if err := cliServer.Start(); err != nil {
		fmt.Printf("start CLI server failed: %v\n", err)
	} else {
		fmt.Printf("CLI server listening on %s\n", cliAddr)
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cliServer.Stop()
		os.Exit(0)
	}()

	srv.Start(addr)
}

func InitFromDB(db *sqlx.DB, e *engine.Engine, sm *persist.SliceManager, operLogWriter *persist.OperLogWriter) error {
	loadedSlice, err := sm.LoadFromSlice()
	if err != nil {
		return fmt.Errorf("load from slice failed: %w", err)
	}

	if loadedSlice == nil {
		fmt.Println("no slice found, starting fresh")
		return nil
	}

	fmt.Printf("loaded slice id=%d, timestamp=%s\n", loadedSlice.SliceID, loadedSlice.Timestamp)

	if err := e.LoadBalancesToEngine(loadedSlice.Balances); err != nil {
		return fmt.Errorf("load balances failed: %w", err)
	}
	fmt.Printf("loaded %d balance records\n", len(loadedSlice.Balances))

	if err := e.LoadOrdersToEngine(loadedSlice.Orders); err != nil {
		return fmt.Errorf("load orders failed: %w", err)
	}

	orderCount := 0
	for _, orders := range loadedSlice.Orders {
		orderCount += len(orders)
	}
	fmt.Printf("loaded %d orders\n", orderCount)

	if loadedSlice.LastOperLogID > 0 {
		fmt.Printf("replaying operlogs from id=%d\n", loadedSlice.LastOperLogID)
		if err := e.ReplayOperLogs(loadedSlice.LastOperLogID, operLogWriter); err != nil {
			return fmt.Errorf("replay operlogs failed: %w", err)
		}
		fmt.Println("operlog replay completed")
	}

	return nil
}

func startStopOrderCron(e *engine.Engine) {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			markets := e.ListMarkets()
			for _, marketName := range markets {
				lastPrice := e.GetLastPrice(marketName)
				if lastPrice.IsZero() {
					continue
				}
				trades, err := e.ProcessTriggeredStopOrders(marketName, lastPrice)
				if err != nil || len(trades) == 0 {
					continue
				}
				for _, trade := range trades {
					fmt.Printf("stop order triggered: market=%s, price=%s, amount=%s\n",
						marketName, trade.Price.String(), trade.Amount.String())
				}
			}
		}
	}()
}
