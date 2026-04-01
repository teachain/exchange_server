package handler

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/viabtc/go-project/services/accessws/internal/auth"
	"github.com/viabtc/go-project/services/accessws/internal/model"
	"time"
)

type ServerHandler struct {
	authService *auth.AuthService
}

func NewServerHandler(authService *auth.AuthService) *ServerHandler {
	return &ServerHandler{authService: authService}
}

func (h *ServerHandler) HandlePing(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	return model.NewSuccessResponse(id, "pong")
}

func (h *ServerHandler) HandleTime(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	return model.NewSuccessResponse(id, time.Now().Unix())
}

func (h *ServerHandler) HandleAuth(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 2 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need token and source")
	}
	token, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid token type")
	}
	source, ok := params[1].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid source type")
	}
	if err := h.authService.AuthenticateSession(sess, token, source); err != nil {
		return model.NewErrorResponse(id, model.ERR_AUTH_FAILED, err.Error())
	}
	return model.NewSuccessResponse(id, map[string]interface{}{"user_id": sess.UserID})
}

func (h *ServerHandler) HandleSign(sess *model.ClientSession, id interface{}, params []interface{}) *model.JSONRPCResponse {
	if len(params) < 3 {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "need access_id, authorisation, tonce")
	}
	accessID, ok := params[0].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid access_id type")
	}
	authorisation, ok := params[1].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid authorisation type")
	}
	tonce, ok := params[2].(string)
	if !ok {
		return model.NewErrorResponse(id, model.ERR_INVALID_PARAMS, "invalid tonce type")
	}
	if err := h.authService.VerifySessionSignature(sess, accessID, authorisation, tonce); err != nil {
		return model.NewErrorResponse(id, model.ERR_AUTH_FAILED, err.Error())
	}
	return model.NewSuccessResponse(id, map[string]interface{}{"user_id": sess.UserID})
}

func SendResponse(conn *websocket.Conn, resp *model.JSONRPCResponse) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, b)
}

func SendNotify(conn *websocket.Conn, method string, params interface{}) error {
	msg := map[string]interface{}{"method": method, "params": params}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, b)
}

func BroadcastToSessions(sessions []*model.ClientSession, method string, params interface{}) {
	for _, sess := range sessions {
		if sess.Dead {
			continue
		}
		SendNotify(sess.Conn, method, params)
	}
}
