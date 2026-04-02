package handler

import (
	"encoding/json"
	"github.com/viabtc/go-project/services/accessws/internal/model"
	"github.com/viabtc/go-project/services/accessws/internal/rpc"
)

type DepthHandler struct {
	rpcClient *rpc.RPCCLient
	subMgr    DepthSubMgr
}

type DepthSubMgr interface {
	DepthSubscribe(*model.ClientSession, string, string, int)
	DepthUnsubscribe(*model.ClientSession)
	GetDepthSubscribers(string) []*model.ClientSession
	GetDepthSnapshot(string) *model.DepthSnapshot
	SetDepthSnapshot(string, *model.DepthSnapshot)
}

func NewDepthHandler(rpcClient *rpc.RPCCLient, subMgr DepthSubMgr) *DepthHandler {
	return &DepthHandler{rpcClient: rpcClient, subMgr: subMgr}
}

func (h *DepthHandler) HandleDepthQuery(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 2 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market and limit")
	}
	market, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid market type")
	}
	limit := 50
	if l, ok := params[1].(float64); ok {
		limit = int(l)
	}

	body := rpc.BuildDepthQueryBody(market, limit)
	resp, err := h.rpcClient.QueryMatchEngine(rpc.CMD_ORDER_BOOK_DEPTH, body)
	if err != nil {
		return model.NewErrorResponse(id, model.ERR_INTERNAL_ERROR, err.Error())
	}

	var result interface{}
	json.Unmarshal(resp.Body, &result)
	return model.NewSuccessResponse(id, result)
}

func (h *DepthHandler) HandleDepthSubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 3 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market, interval, limit")
	}
	market, _ := params[0].(string)
	interval, _ := params[1].(string)
	limit := 50
	if l, ok := params[2].(float64); ok {
		limit = int(l)
	}

	h.subMgr.DepthSubscribe(sess, market, interval, limit)

	body := rpc.BuildDepthQueryBody(market, limit)
	resp, err := h.rpcClient.QueryMatchEngine(rpc.CMD_ORDER_BOOK_DEPTH, body)
	if err == nil {
		BroadcastDepthUpdate(sess, "depth.update", resp.Body, nil)
	}

	return model.NewSuccessResponse(id, true)
}

func (h *DepthHandler) HandleDepthUnsubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	h.subMgr.DepthUnsubscribe(sess)
	return model.NewSuccessResponse(id, true)
}
