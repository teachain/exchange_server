package handler

import (
	"encoding/json"

	"github.com/teachain/exchange_server/internal/accessws/model"
	"github.com/teachain/exchange_server/internal/accessws/rpc"
)

type OrderHandler struct {
	rpcClient *rpc.RPCCLient
	subMgr    OrderSubMgr
}

type OrderSubMgr interface {
	OrderSubscribe(*model.ClientSession, string)
	OrderUnsubscribe(*model.ClientSession, string)
	OrderUnsubscribeAll(*model.ClientSession)
	GetOrderSubscribers(uint32, string) []*model.ClientSession
	GetAllOrderSubscribers(uint32) []*model.ClientSession
}

func NewOrderHandler(rpcClient *rpc.RPCCLient, subMgr OrderSubMgr) *OrderHandler {
	return &OrderHandler{rpcClient: rpcClient, subMgr: subMgr}
}

func (h *OrderHandler) HandleOrderQuery(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if !sess.Auth {
		return model.NewErrorResponse(id, model.ERR_AUTH_REQUIRED, "authentication required")
	}
	if len(params) < 2 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market and limit")
	}
	market, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid market type")
	}
	limit := 100
	if len(params) >= 2 {
		if f, ok := params[1].(float64); ok {
			limit = int(f)
		}
	}

	body := rpc.BuildOrderQueryBody(sess.UserID, market, limit)
	resp, err := h.rpcClient.QueryMatchEngine(rpc.CMD_ORDER_QUERY, body)
	if err != nil {
		return model.NewErrorResponse(id, model.ERR_INTERNAL_ERROR, err.Error())
	}

	var result interface{}
	json.Unmarshal(resp.Body, &result)
	return model.NewSuccessResponse(id, result)
}

func (h *OrderHandler) HandleOrderHistory(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if !sess.Auth {
		return model.NewErrorResponse(id, model.ERR_AUTH_REQUIRED, "authentication required")
	}
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid market type")
	}
	limit := 100
	if len(params) >= 2 {
		if f, ok := params[1].(float64); ok {
			limit = int(f)
		}
	}

	body := rpc.BuildOrderQueryBody(sess.UserID, market, limit)
	resp, err := h.rpcClient.QueryReadHistory(rpc.CMD_ORDER_HISTORY, body)
	if err != nil {
		return model.NewErrorResponse(id, model.ERR_INTERNAL_ERROR, err.Error())
	}

	var result interface{}
	json.Unmarshal(resp.Body, &result)
	return model.NewSuccessResponse(id, result)
}

func (h *OrderHandler) HandleOrderSubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if !sess.Auth {
		return model.NewErrorResponse(id, model.ERR_AUTH_REQUIRED, "authentication required")
	}
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid market type")
	}
	h.subMgr.OrderSubscribe(sess, market)
	return model.NewSuccessResponse(id, true)
}

func (h *OrderHandler) HandleOrderUnsubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if !sess.Auth {
		return model.NewErrorResponse(id, model.ERR_AUTH_REQUIRED, "authentication required")
	}
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need market")
	}
	market, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid market type")
	}
	h.subMgr.OrderUnsubscribe(sess, market)
	return model.NewSuccessResponse(id, true)
}
