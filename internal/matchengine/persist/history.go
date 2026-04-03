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

func (hw *HistoryWriter) hashUserID(userID uint32) int {
	return int(userID) % HistoryHashNum
}

func (hw *HistoryWriter) hashOrderID(orderID uint64) int {
	return int(orderID) % HistoryHashNum
}

func (hw *HistoryWriter) AppendOrderHistory(orderID uint64, userID uint32, market string, side, orderType uint8,
	price, amount, leftAmount, dealStock, dealMoney, takerFee, makerFee string,
	createTime, updateTime time.Time) error {

	tableSuffix := hw.hashUserID(userID)
	query := fmt.Sprintf(`INSERT INTO order_history_%d
		(id, create_time, finish_time, user_id, market, source, t, side, price, amount, taker_fee, maker_fee, deal_stock, deal_money, deal_fee)
		VALUES (?, ?, ?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tableSuffix)

	finishTime := time.Now()
	_, err := hw.db.Exec(query, orderID, createTime, finishTime, userID, market, orderType, side, price, amount, takerFee, makerFee, dealStock, dealMoney, dealMoney)
	return err
}

func (hw *HistoryWriter) AppendOrderDetail(orderID uint64, userID uint32, market string, side, orderType uint8,
	price, amount, leftAmount, dealStock, dealMoney, takerFee, makerFee string,
	createTime, updateTime time.Time) error {

	tableSuffix := hw.hashOrderID(orderID)
	query := fmt.Sprintf(`INSERT INTO order_detail_%d
		(id, create_time, finish_time, user_id, market, source, t, side, price, amount, taker_fee, maker_fee, deal_stock, deal_money, deal_fee)
		VALUES (?, ?, ?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tableSuffix)

	finishTime := time.Now()
	_, err := hw.db.Exec(query, orderID, createTime, finishTime, userID, market, orderType, side, price, amount, takerFee, makerFee, dealStock, dealMoney, dealMoney)
	return err
}

func (hw *HistoryWriter) AppendDealHistory(takerOrderID, makerOrderID uint64, takerUserID, makerUserID uint32,
	market string, side uint8, price, amount, deal, takerFee, makerFee string, createTime time.Time) error {

	tableSuffix := hw.hashOrderID(takerOrderID)
	query := fmt.Sprintf(`INSERT INTO deal_history_%d
		(time, user_id, deal_id, order_id, deal_order_id, role, price, amount, deal, fee, deal_fee)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tableSuffix)

	dealID := takerOrderID
	role := uint8(1)
	_, err := hw.db.Exec(query, createTime.Unix(), takerUserID, dealID, takerOrderID, makerOrderID, role, price, amount, deal, takerFee, takerFee)
	return err
}

func (hw *HistoryWriter) AppendUserDealHistory(userID uint32, market string, side, role uint8,
	price, amount, deal, fee string, createTime time.Time) error {

	tableSuffix := hw.hashUserID(userID)
	query := fmt.Sprintf(`INSERT INTO user_deal_history_%d
		(time, user_id, market, deal_id, order_id, deal_order_id, side, role, price, amount, deal, fee, deal_fee)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tableSuffix)

	dealID := uint64(0)
	orderID := uint64(0)
	dealOrderID := uint64(0)
	_, err := hw.db.Exec(query, createTime.Unix(), userID, market, dealID, orderID, dealOrderID, side, role, price, amount, deal, fee, fee)
	return err
}

func (hw *HistoryWriter) AppendBalanceHistory(userID uint32, asset, business string,
	change, balance string, detail string, createTime time.Time) error {

	tableSuffix := hw.hashUserID(userID)
	query := fmt.Sprintf(`INSERT INTO balance_history_%d
		(time, user_id, asset, business, change, balance, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tableSuffix)

	_, err := hw.db.Exec(query, createTime.Unix(), userID, asset, business, change, balance, detail)
	return err
}
