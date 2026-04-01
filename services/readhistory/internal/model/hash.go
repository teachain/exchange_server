package model

import "fmt"

const HISTORY_HASH_NUM = 100

func HashUserID(userID uint32) uint32 {
	return userID % HISTORY_HASH_NUM
}

func BalanceHistoryTable(userID uint32) string {
	return fmt.Sprintf("balance_history_%u", HashUserID(userID))
}

func OrderHistoryTable(userID uint32) string {
	return fmt.Sprintf("order_history_%u", HashUserID(userID))
}

func OrderDetailTable(orderID uint64) string {
	return fmt.Sprintf("order_detail_%u", orderID%uint64(HISTORY_HASH_NUM))
}

func DealHistoryTable(orderID uint64) string {
	return fmt.Sprintf("deal_history_%u", orderID%uint64(HISTORY_HASH_NUM))
}

func UserDealHistoryTable(userID uint32) string {
	return fmt.Sprintf("user_deal_history_%u", HashUserID(userID))
}
