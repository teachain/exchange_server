package reader

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/viabtc/go-project/services/readhistory/internal/model"
)

type Reader struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *Reader {
	return &Reader{db: db}
}

func (r *Reader) GetBalanceHistory(userID uint32, asset, business string, startTime, endTime uint64, offset, limit int) ([]*model.BalanceHistory, error) {
	table := model.BalanceHistoryTable(userID)

	query := "SELECT time, asset, business, `change`, balance, detail FROM " + table + " WHERE user_id = ?"
	args := []interface{}{userID}

	if asset != "" {
		query += " AND asset = ?"
		args = append(args, asset)
	}
	if business != "" {
		query += " AND business = ?"
		args = append(args, business)
	}
	if startTime > 0 {
		query += " AND time >= ?"
		args = append(args, startTime)
	}
	if endTime > 0 {
		query += " AND time < ?"
		args = append(args, endTime)
	}

	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	var history []*model.BalanceHistory
	err := r.db.Select(&history, query, args...)
	return history, err
}

func (r *Reader) GetFinishedOrders(userID uint32, market string, side int, startTime, endTime uint64, offset, limit int) ([]*model.OrderHistory, error) {
	table := model.OrderHistoryTable(userID)

	query := `SELECT id, create_time, finish_time, user_id, market, source, t, side, price, amount, 
              taker_fee, maker_fee, deal_stock, deal_money, deal_fee FROM ` + table +
		" WHERE user_id = ? AND market = ?"
	args := []interface{}{userID, market}

	if side != 0 {
		query += " AND side = ?"
		args = append(args, side)
	}
	if startTime > 0 {
		query += " AND create_time >= ?"
		args = append(args, startTime)
	}
	if endTime > 0 {
		query += " AND create_time < ?"
		args = append(args, endTime)
	}

	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	var orders []*model.OrderHistory
	err := r.db.Select(&orders, query, args...)
	return orders, err
}

func (r *Reader) GetOrderDeals(orderID uint64, offset, limit int) ([]*model.DealHistory, error) {
	table := model.DealHistoryTable(orderID)

	query := `SELECT time, user_id, deal_id, role, price, amount, deal, fee, deal_order_id FROM ` + table +
		" WHERE order_id = ? ORDER BY id DESC LIMIT ? OFFSET ?"

	var deals []*model.DealHistory
	err := r.db.Select(&deals, query, orderID, limit, offset)
	return deals, err
}

func (r *Reader) GetFinishedOrderDetail(orderID uint64) (*model.OrderHistory, error) {
	table := model.OrderDetailTable(orderID)

	query := `SELECT id, create_time, finish_time, user_id, market, source, t, side, price, amount, 
              taker_fee, maker_fee, deal_stock, deal_money, deal_fee FROM ` + table +
		" WHERE id = ?"

	var order model.OrderHistory
	err := r.db.Get(&order, query, orderID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *Reader) GetUserMarketDeals(userID uint32, market string, offset, limit int) ([]*model.DealHistory, error) {
	table := model.UserDealHistoryTable(userID)

	query := `SELECT time, user_id, deal_id, side, role, price, amount, deal, fee, deal_order_id, market FROM ` + table +
		" WHERE user_id = ? AND market = ? ORDER BY id DESC LIMIT ? OFFSET ?"

	var deals []*model.DealHistory
	err := r.db.Select(&deals, query, userID, market, limit, offset)
	return deals, err
}
