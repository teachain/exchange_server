package handler

import (
	"encoding/json"

	"github.com/teachain/exchange_server/internal/accessws/model"
	"github.com/teachain/exchange_server/internal/accessws/rpc"
)

type KlineHandler struct {
	rpcClient *rpc.RPCCLient
	subMgr    KlineSubMgr
}

type KlineSubMgr interface {
	KlineSubscribe(*model.ClientSession, string, string)
	KlineUnsubscribe(*model.ClientSession)
	GetKlineSubscribers(string) []*model.ClientSession
}

func NewKlineHandler(rpcClient *rpc.RPCCLient, subMgr KlineSubMgr) *KlineHandler {
	return &KlineHandler{rpcClient: rpcClient, subMgr: subMgr}
}

func (h *KlineHandler) HandleKlineQuery(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 2 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market and interval")
	}
	market, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid market type")
	}
	interval, ok := params[1].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid interval type")
	}
	limit := 100
	if len(params) >= 3 {
		if l, ok := params[2].(float64); ok {
			limit = int(l)
		}
	}

	body := rpc.BuildKlineQueryBody(market, interval, limit)
	resp, err := h.rpcClient.QueryMarketPrice(rpc.CMD_MARKET_KLINE, body)
	if err != nil {
		return model.NewErrorResponse(id, model.ERR_INTERNAL_ERROR, err.Error())
	}

	var result interface{}
	json.Unmarshal(resp.Body, &result)
	return model.NewSuccessResponse(id, result)
}

func (h *KlineHandler) HandleKlineSubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 2 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market and interval")
	}
	market, _ := params[0].(string)
	interval, _ := params[1].(string)
	h.subMgr.KlineSubscribe(sess, market, interval)
	return model.NewSuccessResponse(id, true)
}

func (h *KlineHandler) HandleKlineUnsubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	h.subMgr.KlineUnsubscribe(sess)
	return model.NewSuccessResponse(id, true)
}

type PriceHandler struct {
	rpcClient *rpc.RPCCLient
	subMgr    PriceSubMgr
}

type PriceSubMgr interface {
	PriceSubscribe(*model.ClientSession, string)
	PriceUnsubscribe(*model.ClientSession)
	GetPriceSubscribers(string) []*model.ClientSession
}

func NewPriceHandler(rpcClient *rpc.RPCCLient, subMgr PriceSubMgr) *PriceHandler {
	return &PriceHandler{rpcClient: rpcClient, subMgr: subMgr}
}

func (h *PriceHandler) HandlePriceQuery(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid market type")
	}

	body := rpc.BuildDealsQueryBody(market, 1, 0)
	resp, err := h.rpcClient.QueryMarketPrice(rpc.CMD_MARKET_DEALS, body)
	if err != nil {
		return model.NewErrorResponse(id, model.ERR_INTERNAL_ERROR, err.Error())
	}

	var result interface{}
	json.Unmarshal(resp.Body, &result)
	return model.NewSuccessResponse(id, result)
}

func (h *PriceHandler) HandlePriceSubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, _ := params[0].(string)
	h.subMgr.PriceSubscribe(sess, market)
	return model.NewSuccessResponse(id, true)
}

func (h *PriceHandler) HandlePriceUnsubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	h.subMgr.PriceUnsubscribe(sess)
	return model.NewSuccessResponse(id, true)
}

type DealsHandler struct {
	rpcClient *rpc.RPCCLient
	subMgr    DealsSubMgr
}

type DealsSubMgr interface {
	DealsSubscribe(*model.ClientSession, string)
	DealsUnsubscribe(*model.ClientSession)
	GetDealsSubscribers(string) []*model.ClientSession
	GetDealsBuffer(market string) *model.DealsBuffer
}

func NewDealsHandler(rpcClient *rpc.RPCCLient, subMgr DealsSubMgr) *DealsHandler {
	return &DealsHandler{rpcClient: rpcClient, subMgr: subMgr}
}

func (h *DealsHandler) HandleDealsQuery(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid market type")
	}
	limit := 50
	since := int64(0)
	if len(params) >= 2 {
		if l, ok := params[1].(float64); ok {
			limit = int(l)
		}
	}
	if len(params) >= 3 {
		if s, ok := params[2].(float64); ok {
			since = int64(s)
		}
	}

	body := rpc.BuildDealsQueryBody(market, limit, since)
	resp, err := h.rpcClient.QueryMarketPrice(rpc.CMD_MARKET_DEALS, body)
	if err != nil {
		return model.NewErrorResponse(id, model.ERR_INTERNAL_ERROR, err.Error())
	}

	var result interface{}
	json.Unmarshal(resp.Body, &result)
	return model.NewSuccessResponse(id, result)
}

func (h *DealsHandler) HandleDealsSubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, _ := params[0].(string)

	buf := h.subMgr.GetDealsBuffer(market)
	records := buf.GetAll()

	result := make([]map[string]interface{}, 0, len(records))
	for _, deal := range records {
		result = append(result, map[string]interface{}{
			"id":     deal.ID,
			"time":   deal.Time,
			"type":   deal.Type,
			"amount": deal.Amount,
			"price":  deal.Price,
		})
	}
	SendNotify(sess.Conn, "deals.update", result)

	h.subMgr.DealsSubscribe(sess, market)
	return model.NewSuccessResponse(id, true)
}

func (h *DealsHandler) HandleDealsUnsubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	h.subMgr.DealsUnsubscribe(sess)
	return model.NewSuccessResponse(id, true)
}

type StateHandler struct {
	rpcClient *rpc.RPCCLient
	subMgr    StateSubMgr
}

type StateSubMgr interface {
	StateSubscribe(*model.ClientSession, string)
	StateUnsubscribe(*model.ClientSession)
	GetStateSubscribers(string) []*model.ClientSession
}

func NewStateHandler(rpcClient *rpc.RPCCLient, subMgr StateSubMgr) *StateHandler {
	return &StateHandler{rpcClient: rpcClient, subMgr: subMgr}
}

func (h *StateHandler) HandleStateQuery(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid market type")
	}

	body := rpc.BuildStateQueryBody(market)
	resp, err := h.rpcClient.QueryMarketPrice(rpc.CMD_MARKET_STATUS, body)
	if err != nil {
		return model.NewErrorResponse(id, model.ERR_INTERNAL_ERROR, err.Error())
	}

	var result interface{}
	json.Unmarshal(resp.Body, &result)
	return model.NewSuccessResponse(id, result)
}

func (h *StateHandler) HandleStateSubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, _ := params[0].(string)
	h.subMgr.StateSubscribe(sess, market)
	return model.NewSuccessResponse(id, true)
}

func (h *StateHandler) HandleStateUnsubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	h.subMgr.StateUnsubscribe(sess)
	return model.NewSuccessResponse(id, true)
}

type TodayHandler struct {
	rpcClient *rpc.RPCCLient
	subMgr    TodaySubMgr
}

type TodaySubMgr interface {
	TodaySubscribe(*model.ClientSession, string)
	TodayUnsubscribe(*model.ClientSession)
	GetTodaySubscribers(string) []*model.ClientSession
}

func NewTodayHandler(rpcClient *rpc.RPCCLient, subMgr TodaySubMgr) *TodayHandler {
	return &TodayHandler{rpcClient: rpcClient, subMgr: subMgr}
}

func (h *TodayHandler) HandleTodayQuery(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid market type")
	}

	body := rpc.BuildTodayQueryBody(market)
	resp, err := h.rpcClient.QueryMarketPrice(rpc.CMD_MARKET_STATUS_TODAY, body)
	if err != nil {
		return model.NewErrorResponse(id, model.ERR_INTERNAL_ERROR, err.Error())
	}

	var result interface{}
	json.Unmarshal(resp.Body, &result)
	return model.NewSuccessResponse(id, result)
}

func (h *TodayHandler) HandleTodaySubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, _ := params[0].(string)
	h.subMgr.TodaySubscribe(sess, market)
	return model.NewSuccessResponse(id, true)
}

func (h *TodayHandler) HandleTodayUnsubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	h.subMgr.TodayUnsubscribe(sess)
	return model.NewSuccessResponse(id, true)
}
