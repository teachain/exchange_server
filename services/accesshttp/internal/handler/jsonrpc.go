package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viabtc/go-project/services/accesshttp/internal/model"
	"github.com/viabtc/go-project/services/accesshttp/internal/proxy"
)

type Handler struct {
	cfg *proxy.BackendProxy
}

func New(cfg *proxy.BackendProxy) *Handler {
	return &Handler{
		cfg: cfg,
	}
}

func (h *Handler) HandleJSONRPC(c *gin.Context) {
	var req model.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, model.JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &model.RPCError{
				Code:    -32700,
				Message: "Parse error",
			},
			ID: nil,
		})
		return
	}

	if req.ID == nil {
		c.JSON(http.StatusOK, model.JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &model.RPCError{
				Code:    -32600,
				Message: "Invalid Request",
			},
			ID: nil,
		})
		return
	}

	var resp interface{}
	var err error

	switch req.Method {
	case "asset.list", "asset.summary":
		resp, err = h.cfg.ForwardToAsset(c.Request.Context(), &req)

	case "balance.query", "balance.update":
		resp, err = h.cfg.ForwardToBalance(c.Request.Context(), &req)
	case "balance.history":
		resp, err = h.cfg.ForwardToBalance(c.Request.Context(), &req)

	case "order.put_limit", "order.put_market", "order.cancel",
		"order.book", "order.depth", "order.pending", "order.pending_detail":
		resp, err = h.cfg.ForwardToMatchEngine(c.Request.Context(), &req)
	case "order.deals", "order.finished", "order.finished_detail":
		resp, err = h.cfg.ForwardToReadHistory(c.Request.Context(), &req)

	case "market.last", "market.deals", "market.kline",
		"market.status", "market.status_today":
		resp, err = h.cfg.ForwardToMarketPrice(c.Request.Context(), &req)
	case "market.list", "market.summary":
		resp, err = h.cfg.ForwardToMatchEngine(c.Request.Context(), &req)
	case "market.user_deals":
		resp, err = h.cfg.ForwardToReadHistory(c.Request.Context(), &req)

	default:
		err = &model.RPCError{Code: -32601, Message: "Method not found"}
	}

	if err != nil {
		rpcErr, _ := err.(*model.RPCError)
		c.JSON(http.StatusOK, model.JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   rpcErr,
			ID:      req.ID,
		})
		return
	}

	c.JSON(http.StatusOK, model.JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  resp,
		ID:      req.ID,
	})
}

func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
