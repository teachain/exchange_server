package persist

import (
	"database/sql"
	"fmt"
	"time"
)

const HistoryHashNum = 100

type HistoryWriter struct {
	db *sql.DB
}

func NewHistoryWriter(db *sql.DB) *HistoryWriter {
	return &HistoryWriter{db: db}
}

func (hw *HistoryWriter) InitDB() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS order_history (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			user_id INT UNSIGNED NOT NULL,
			market VARCHAR(32) NOT NULL,
			side TINYINT UNSIGNED NOT NULL,
			type TINYINT UNSIGNED NOT NULL,
			price VARCHAR(64) NOT NULL,
			amount VARCHAR(64) NOT NULL,
			left_amount VARCHAR(64) NOT NULL,
			deal_stock VARCHAR(64) NOT NULL,
			deal_money VARCHAR(64) NOT NULL,
			taker_fee VARCHAR(64) NOT NULL,
			maker_fee VARCHAR(64) NOT NULL,
			source VARCHAR(128),
			create_time DATETIME NOT NULL,
			update_time DATETIME NOT NULL,
			KEY idx_user_market (user_id, market),
			KEY idx_create_time (create_time)
		)`,
		`CREATE TABLE IF NOT EXISTS order_detail (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			order_id BIGINT UNSIGNED NOT NULL,
			user_id INT UNSIGNED NOT NULL,
			market VARCHAR(32) NOT NULL,
			side TINYINT UNSIGNED NOT NULL,
			type TINYINT UNSIGNED NOT NULL,
			price VARCHAR(64) NOT NULL,
			amount VARCHAR(64) NOT NULL,
			left_amount VARCHAR(64) NOT NULL,
			deal_stock VARCHAR(64) NOT NULL,
			deal_money VARCHAR(64) NOT NULL,
			taker_fee VARCHAR(64) NOT NULL,
			maker_fee VARCHAR(64) NOT NULL,
			source VARCHAR(128),
			create_time DATETIME NOT NULL,
			update_time DATETIME NOT NULL,
			KEY idx_order_id (order_id)
		)`,
		`CREATE TABLE IF NOT EXISTS deal_history (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			taker_order_id BIGINT UNSIGNED NOT NULL,
			maker_order_id BIGINT UNSIGNED NOT NULL,
			taker_user_id INT UNSIGNED NOT NULL,
			maker_user_id INT UNSIGNED NOT NULL,
			market VARCHAR(32) NOT NULL,
			side TINYINT UNSIGNED NOT NULL,
			price VARCHAR(64) NOT NULL,
			amount VARCHAR(64) NOT NULL,
			deal VARCHAR(64) NOT NULL,
			taker_fee VARCHAR(64) NOT NULL,
			maker_fee VARCHAR(64) NOT NULL,
			create_time DATETIME NOT NULL,
			KEY idx_taker_order (taker_order_id),
			KEY idx_maker_order (maker_order_id),
			KEY idx_create_time (create_time)
		)`,
		`CREATE TABLE IF NOT EXISTS user_deal_history (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			user_id INT UNSIGNED NOT NULL,
			market VARCHAR(32) NOT NULL,
			side TINYINT UNSIGNED NOT NULL,
			role TINYINT UNSIGNED NOT NULL,
			price VARCHAR(64) NOT NULL,
			amount VARCHAR(64) NOT NULL,
			deal VARCHAR(64) NOT NULL,
			fee VARCHAR(64) NOT NULL,
			create_time DATETIME NOT NULL,
			KEY idx_user_market (user_id, market),
			KEY idx_create_time (create_time)
		)`,
		`CREATE TABLE IF NOT EXISTS balance_history (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			user_id INT UNSIGNED NOT NULL,
			asset VARCHAR(32) NOT NULL,
			business VARCHAR(32) NOT NULL,
			change VARCHAR(64) NOT NULL,
			balance VARCHAR(64) NOT NULL,
			detail TEXT,
			create_time DATETIME NOT NULL,
			KEY idx_user_asset (user_id, asset),
			KEY idx_create_time (create_time)
		)`,
	}

	for _, q := range queries {
		if _, err := hw.db.Exec(q); err != nil {
			return fmt.Errorf("failed to create history table: %w", err)
		}
	}

	return nil
}

func (hw *HistoryWriter) hashUserID(userID uint32) int {
	return int(userID) % HistoryHashNum
}

func (hw *HistoryWriter) AppendOrderHistory(orderID uint64, userID uint32, market string, side, orderType uint8,
	price, amount, leftAmount, dealStock, dealMoney, takerFee, makerFee string,
	createTime, updateTime time.Time) error {

	tableSuffix := hw.hashUserID(userID)
	query := fmt.Sprintf(`INSERT INTO order_history_%d 
		(order_id, user_id, market, side, type, price, amount, left_amount, deal_stock, deal_money, taker_fee, maker_fee, source, create_time, update_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?)`,
		tableSuffix)

	_, err := hw.db.Exec(query, orderID, userID, market, side, orderType, price, amount, leftAmount, dealStock, dealMoney, takerFee, makerFee, createTime, updateTime)
	return err
}

func (hw *HistoryWriter) AppendOrderDetail(orderID uint64, userID uint32, market string, side, orderType uint8,
	price, amount, leftAmount, dealStock, dealMoney, takerFee, makerFee string,
	createTime, updateTime time.Time) error {

	tableSuffix := hw.hashUserID(userID)
	query := fmt.Sprintf(`INSERT INTO order_detail_%d 
		(order_id, user_id, market, side, type, price, amount, left_amount, deal_stock, deal_money, taker_fee, maker_fee, source, create_time, update_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?)`,
		tableSuffix)

	_, err := hw.db.Exec(query, orderID, userID, market, side, orderType, price, amount, leftAmount, dealStock, dealMoney, takerFee, makerFee, createTime, updateTime)
	return err
}

func (hw *HistoryWriter) AppendDealHistory(takerOrderID, makerOrderID uint64, takerUserID, makerUserID uint32,
	market string, side uint8, price, amount, deal, takerFee, makerFee string, createTime time.Time) error {

	query := `INSERT INTO deal_history 
		(taker_order_id, maker_order_id, taker_user_id, maker_user_id, market, side, price, amount, deal, taker_fee, maker_fee, create_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := hw.db.Exec(query, takerOrderID, makerOrderID, takerUserID, makerUserID, market, side, price, amount, deal, takerFee, makerFee, createTime)
	return err
}

func (hw *HistoryWriter) AppendUserDealHistory(userID uint32, market string, side, role uint8,
	price, amount, deal, fee string, createTime time.Time) error {

	tableSuffix := hw.hashUserID(userID)
	query := fmt.Sprintf(`INSERT INTO user_deal_history_%d 
		(user_id, market, side, role, price, amount, deal, fee, create_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tableSuffix)

	_, err := hw.db.Exec(query, userID, market, side, role, price, amount, deal, fee, createTime)
	return err
}

func (hw *HistoryWriter) AppendBalanceHistory(userID uint32, asset, business string,
	change, balance string, detail string, createTime time.Time) error {

	tableSuffix := hw.hashUserID(userID)
	query := fmt.Sprintf(`INSERT INTO balance_history_%d 
		(user_id, asset, business, change, balance, detail, create_time)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tableSuffix)

	_, err := hw.db.Exec(query, userID, asset, business, change, balance, detail, createTime)
	return err
}
