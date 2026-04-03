package handler

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/viabtc/go-project/services/readhistory/internal/reader"
	"github.com/viabtc/go-project/services/readhistory/internal/server"
)

func logDebugMarket(s *server.Server, format string, v ...interface{}) {
	if s.IsDebug() {
		log.Printf("[DEBUG] "+format, v...)
	}
}

const CMD_MARKET_USER_DEALS = 306

func RegisterMarketHandlers(s *server.Server) {
	s.Handle(CMD_MARKET_USER_DEALS, HandleMarketUserDeals)
}

func HandleMarketUserDeals(s *server.Server, pkg *server.RPCPkg) ([]byte, error) {
	logDebugMarket(s, "HandleMarketUserDeals called with body: %s", string(pkg.Body))

	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, errors.New("invalid params")
	}

	if len(params) != 4 {
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

	offset := int(params[2].(float64))
	limit := int(params[3].(float64))

	if limit == 0 || limit > 101 {
		return nil, errors.New("invalid argument")
	}

	r := s.GetReader().(*reader.Reader)
	records, err := r.GetUserMarketDeals(userID, market, offset, limit)
	if err != nil {
		logDebugMarket(s, "HandleMarketUserDeals error: %v", err)
		return nil, err
	}
	logDebugMarket(s, "HandleMarketUserDeals returned %d records", len(records))

	result := map[string]interface{}{
		"offset":  offset,
		"limit":   limit,
		"records": records,
	}

	return json.Marshal(result)
}
