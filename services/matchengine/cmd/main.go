package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/go-sql-driver/mysql"
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

	dbHost := viper.GetString("database.host")
	dbPort := viper.GetInt("database.port")
	dbName := viper.GetString("database.name")
	dbUser := viper.GetString("database.username")
	dbPass := viper.GetString("database.password")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True",
		dbUser, dbPass, dbHost, dbPort, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println("connect to database failed:", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	if err := loadBalanceFromDB(db, e); err != nil {
		fmt.Println("load balance failed:", err.Error())
		os.Exit(1)
	}

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

func loadBalanceFromDB(db *sql.DB, e *engine.Engine) error {
	rows, err := db.Query("SHOW TABLES LIKE 'slice_balance_%'")
	if err != nil {
		return err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		tableNames = append(tableNames, name)
	}

	if len(tableNames) == 0 {
		fmt.Println("no balance snapshot tables found")
		return nil
	}

	var latestTable string
	var latestTimestamp int64
	for _, name := range tableNames {
		if name == "slice_balance_example" {
			continue
		}
		var ts int64
		fmt.Sscanf(name, "slice_balance_%d", &ts)
		if ts > latestTimestamp {
			latestTimestamp = ts
			latestTable = name
		}
	}

	if latestTable == "" {
		fmt.Println("no valid balance snapshot table found")
		return nil
	}

	fmt.Printf("loading balance from table: %s\n", latestTable)

	query := fmt.Sprintf("SELECT user_id, asset, t, balance FROM %s", latestTable)
	rows, err = db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	loaded := 0
	for rows.Next() {
		var userID int64
		var asset string
		var balType int
		var balanceStr string
		if err := rows.Scan(&userID, &asset, &balType, &balanceStr); err != nil {
			return err
		}

		balance, err := decimal.NewFromString(balanceStr)
		if err != nil {
			continue
		}

		if balType == 0 {
			e.SetBalance(uint32(userID), asset, balance, decimal.Zero)
			loaded++
		} else if balType == 1 {
			e.SetBalance(uint32(userID), asset, balance, decimal.Zero)
			loaded++
		}
	}

	fmt.Printf("loaded %d balance records\n", loaded)
	return nil
}
