package handler

import (
	"encoding/json"
	"github.com/viabtc/go-project/services/accessws/internal/model"
	"github.com/viabtc/go-project/services/accessws/internal/rpc"
)

type DepthHandler struct {
	rpcClient  *rpc.RPCCLient
	subMgr     DepthSubMgr
	depthLimit []int
	depthMerge []string
}

type DepthSubMgr interface {
	DepthSubscribe(*model.ClientSession, string, string, int)
	DepthUnsubscribe(*model.ClientSession)
	GetDepthSubscribers(string) []*model.ClientSession
	GetDepthSnapshot(string) *model.DepthSnapshot
	SetDepthSnapshot(string, *model.DepthSnapshot)
}

func NewDepthHandler(rpcClient *rpc.RPCCLient, subMgr DepthSubMgr, depthLimit []int, depthMerge []string) *DepthHandler {
	return &DepthHandler{
		rpcClient:  rpcClient,
		subMgr:     subMgr,
		depthLimit: depthLimit,
		depthMerge: depthMerge,
	}
}

func (h *DepthHandler) validateLimit(limit int) bool {
	if len(h.depthLimit) == 0 {
		return true
	}
	for _, allowed := range h.depthLimit {
		if allowed == limit {
			return true
		}
	}
	return false
}

func (h *DepthHandler) validateInterval(interval string) bool {
	if len(h.depthMerge) == 0 {
		return true
	}
	for _, allowed := range h.depthMerge {
		if allowed == interval {
			return true
		}
	}
	return false
}

func (h *DepthHandler) getValidLimit(limit int) int {
	if h.validateLimit(limit) {
		return limit
	}
	if len(h.depthLimit) > 0 {
		return h.depthLimit[0]
	}
	return limit
}

func (h *DepthHandler) getValidInterval(interval string) string {
	if h.validateInterval(interval) {
		return interval
	}
	if len(h.depthMerge) > 0 {
		return h.depthMerge[0]
	}
	return interval
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

	validLimit := h.getValidLimit(limit)
	body := rpc.BuildDepthQueryBody(market, validLimit)
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

	validLimit := h.getValidLimit(limit)
	validInterval := h.getValidInterval(interval)

	h.subMgr.DepthSubscribe(sess, market, validInterval, validLimit)

	body := rpc.BuildDepthQueryBody(market, validLimit)
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
