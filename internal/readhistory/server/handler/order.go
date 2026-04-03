package handler

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/teachain/exchange_server/internal/readhistory/reader"
	"github.com/teachain/exchange_server/internal/readhistory/server"
)

func logDebug(s *server.Server, format string, v ...interface{}) {
	if s.IsDebug() {
		log.Printf("[DEBUG] "+format, v...)
	}
}

const (
	CMD_ORDER_HISTORY         = 208
	CMD_ORDER_DEALS           = 209
	CMD_ORDER_DETAIL_FINISHED = 210
)

func RegisterOrderHandlers(s *server.Server) {
	s.Handle(CMD_ORDER_HISTORY, HandleOrderHistory)
	s.Handle(CMD_ORDER_DEALS, HandleOrderDeals)
	s.Handle(CMD_ORDER_DETAIL_FINISHED, HandleOrderDetailFinished)
}

func HandleOrderHistory(s *server.Server, pkg *server.RPCPkg) ([]byte, error) {
	logDebug(s, "HandleOrderHistory called with body: %s", string(pkg.Body))

	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, errors.New("invalid params")
	}

	if len(params) < 6 {
		return nil, errors.New("invalid argument")
	}

	userID := uint32(params[0].(float64))
	if userID == 0 {
		return nil, errors.New("invalid argument")
	}

	market, ok := params[1].(string)
	if !ok {
		return nil, errors.New("invalid argument")
	}

	startTime := uint64(params[2].(float64))
	endTime := uint64(params[3].(float64))
	offset := int(params[4].(float64))
	limit := int(params[5].(float64))

	if limit == 0 || limit > 101 {
		return nil, errors.New("invalid argument")
	}

	side := 0
	if len(params) >= 7 {
		side = int(params[6].(float64))
		if side != 0 && side != 1 && side != 2 {
			return nil, errors.New("invalid argument")
		}
	}

	r := s.GetReader().(*reader.Reader)
	records, err := r.GetFinishedOrders(userID, market, side, startTime, endTime, offset, limit)
	if err != nil {
		logDebug(s, "HandleOrderHistory error: %v", err)
		return nil, err
	}
	logDebug(s, "HandleOrderHistory returned %d records", len(records))

	result := map[string]interface{}{
		"offset":  offset,
		"limit":   limit,
		"records": records,
	}

	return json.Marshal(result)
}

func HandleOrderDeals(s *server.Server, pkg *server.RPCPkg) ([]byte, error) {
	logDebug(s, "HandleOrderDeals called with body: %s", string(pkg.Body))

	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, errors.New("invalid params")
	}

	if len(params) != 3 {
		return nil, errors.New("invalid argument")
	}

	orderID := uint64(params[0].(float64))
	if orderID == 0 {
		return nil, errors.New("invalid argument")
	}
	offset := int(params[1].(float64))
	limit := int(params[2].(float64))

	if limit == 0 || limit > 101 {
		return nil, errors.New("invalid argument")
	}

	r := s.GetReader().(*reader.Reader)
	records, err := r.GetOrderDeals(orderID, offset, limit)
	if err != nil {
		logDebug(s, "HandleOrderDeals error: %v", err)
		return nil, err
	}
	logDebug(s, "HandleOrderDeals returned %d records", len(records))

	result := map[string]interface{}{
		"offset":  offset,
		"limit":   limit,
		"records": records,
	}

	return json.Marshal(result)
}

func HandleOrderDetailFinished(s *server.Server, pkg *server.RPCPkg) ([]byte, error) {
	logDebug(s, "HandleOrderDetailFinished called with body: %s", string(pkg.Body))
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, errors.New("invalid params")
	}

	if len(params) != 1 {
		return nil, errors.New("invalid argument")
	}

	orderID := uint64(params[0].(float64))
	if orderID == 0 {
		return nil, errors.New("invalid argument")
	}

	r := s.GetReader().(*reader.Reader)
	detail, err := r.GetFinishedOrderDetail(orderID)
	if err != nil {
		logDebug(s, "HandleOrderDetailFinished error: %v", err)
		return nil, err
	}
	logDebug(s, "HandleOrderDetailFinished returned detail: %v", detail != nil)

	if detail == nil {
		return []byte("null"), nil
	}

	return json.Marshal(detail)
}
