package handler

import (
	"encoding/json"
	"errors"
	"github.com/viabtc/go-project/services/readhistory/internal/reader"
	"github.com/viabtc/go-project/services/readhistory/internal/server"
)

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
		return nil, err
	}

	result := map[string]interface{}{
		"offset":  offset,
		"limit":   limit,
		"records": records,
	}

	return json.Marshal(result)
}

func HandleOrderDeals(s *server.Server, pkg *server.RPCPkg) ([]byte, error) {
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
		return nil, err
	}

	result := map[string]interface{}{
		"offset":  offset,
		"limit":   limit,
		"records": records,
	}

	return json.Marshal(result)
}

func HandleOrderDetailFinished(s *server.Server, pkg *server.RPCPkg) ([]byte, error) {
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
		return nil, err
	}

	if detail == nil {
		return []byte("null"), nil
	}

	return json.Marshal(detail)
}
