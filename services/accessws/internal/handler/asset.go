package handler

import (
	"encoding/json"
	"github.com/teachain/exchange_server/services/accessws/internal/model"
	"github.com/teachain/exchange_server/services/accessws/internal/rpc"
)

type AssetHandler struct {
	rpcClient *rpc.RPCCLient
	subMgr    AssetSubMgr
}

type AssetSubMgr interface {
	AssetSubscribe(*model.ClientSession, string)
	AssetUnsubscribe(*model.ClientSession, string)
	GetAssetSubscribers(uint32, string) []*model.ClientSession
}

func NewAssetHandler(rpcClient *rpc.RPCCLient, subMgr AssetSubMgr) *AssetHandler {
	return &AssetHandler{rpcClient: rpcClient, subMgr: subMgr}
}

func (h *AssetHandler) HandleAssetQuery(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if !sess.Auth {
		return model.NewErrorResponse(id, model.ERR_AUTH_REQUIRED, "authentication required")
	}
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need asset")
	}
	asset, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid asset type")
	}

	body := rpc.BuildBalanceQueryBody(sess.UserID, asset)
	resp, err := h.rpcClient.QueryMatchEngine(rpc.CMD_BALANCE_QUERY, body)
	if err != nil {
		return model.NewErrorResponse(id, model.ERR_INTERNAL_ERROR, err.Error())
	}

	var result interface{}
	json.Unmarshal(resp.Body, &result)
	return model.NewSuccessResponse(id, result)
}

func (h *AssetHandler) HandleAssetHistory(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if !sess.Auth {
		return model.NewErrorResponse(id, model.ERR_AUTH_REQUIRED, "authentication required")
	}
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need asset")
	}
	asset, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid asset type")
	}

	body := rpc.BuildBalanceQueryBody(sess.UserID, asset)
	resp, err := h.rpcClient.QueryReadHistory(rpc.CMD_BALANCE_HISTORY, body)
	if err != nil {
		return model.NewErrorResponse(id, model.ERR_INTERNAL_ERROR, err.Error())
	}

	var result interface{}
	json.Unmarshal(resp.Body, &result)
	return model.NewSuccessResponse(id, result)
}

func (h *AssetHandler) HandleAssetSubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if !sess.Auth {
		return model.NewErrorResponse(id, model.ERR_AUTH_REQUIRED, "authentication required")
	}
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need asset")
	}
	asset, _ := params[0].(string)
	h.subMgr.AssetSubscribe(sess, asset)
	return model.NewSuccessResponse(id, true)
}

func (h *AssetHandler) HandleAssetUnsubscribe(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if !sess.Auth {
		return model.NewErrorResponse(id, model.ERR_AUTH_REQUIRED, "authentication required")
	}
	if len(params) < 1 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need asset")
	}
	asset, _ := params[0].(string)
	h.subMgr.AssetUnsubscribe(sess, asset)
	return model.NewSuccessResponse(id, true)
}
