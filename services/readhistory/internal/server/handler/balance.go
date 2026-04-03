package handler

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/viabtc/go-project/services/readhistory/internal/reader"
	"github.com/viabtc/go-project/services/readhistory/internal/server"
)

func logDebugBalance(s *server.Server, format string, v ...interface{}) {
	if s.IsDebug() {
		log.Printf("[DEBUG] "+format, v...)
	}
}

const CMD_BALANCE_HISTORY = 103

func RegisterBalanceHandlers(s *server.Server) {
	s.Handle(CMD_BALANCE_HISTORY, HandleBalanceHistory)
}

func HandleBalanceHistory(s *server.Server, pkg *server.RPCPkg) ([]byte, error) {
	logDebugBalance(s, "HandleBalanceHistory called with body: %s", string(pkg.Body))

	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, errors.New("invalid params")
	}

	if len(params) != 7 {
		return nil, errors.New("invalid argument")
	}

	userID := uint32(params[0].(float64))
	if userID == 0 {
		return nil, errors.New("invalid argument")
	}

	asset, ok := params[1].(string)
	if !ok {
		return nil, errors.New("invalid argument")
	}

	business, ok := params[2].(string)
	if !ok {
		return nil, errors.New("invalid argument")
	}

	startTime := uint64(params[3].(float64))
	endTime := uint64(params[4].(float64))
	offset := int(params[5].(float64))
	limit := int(params[6].(float64))

	if limit == 0 || limit > 101 {
		return nil, errors.New("invalid argument")
	}

	r := s.GetReader().(*reader.Reader)
	records, err := r.GetBalanceHistory(userID, asset, business, startTime, endTime, offset, limit)
	if err != nil {
		logDebugBalance(s, "HandleBalanceHistory error: %v", err)
		return nil, err
	}
	logDebugBalance(s, "HandleBalanceHistory returned %d records", len(records))

	result := map[string]interface{}{
		"offset":  offset,
		"limit":   limit,
		"records": records,
	}

	return json.Marshal(result)
}
