package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/viabtc/go-project/services/accesshttp/internal/config"
	"github.com/viabtc/go-project/services/accesshttp/internal/model"
)

type BackendProxy struct {
	matchengine string
	marketprice string
	readhistory string
	httpClient  *http.Client
}

func NewBackendProxy(cfg *config.Config) *BackendProxy {
	return &BackendProxy{
		matchengine: "http://" + cfg.Backend.MatchEngine + "/",
		marketprice: "http://" + cfg.Backend.MarketPrice + "/",
		readhistory: "http://" + cfg.Backend.ReadHistory + "/",
		httpClient:  &http.Client{},
	}
}

func (p *BackendProxy) ForwardToMatchEngine(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	switch req.Method {
	case "balance.query", "balance.update", "balance.history":
		return nil, nil
	default:
		return p.forward(ctx, p.matchengine, req)
	}
}

func (p *BackendProxy) ForwardToBalance(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	switch req.Method {
	case "balance.history":
		return p.forward(ctx, p.readhistory, req)
	default:
		return p.forward(ctx, p.matchengine, req)
	}
}

func (p *BackendProxy) ForwardToAsset(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	return p.forward(ctx, p.matchengine, req)
}

func (p *BackendProxy) ForwardToMarketPrice(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	return p.forward(ctx, p.marketprice, req)
}

func (p *BackendProxy) ForwardToReadHistory(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	return p.forward(ctx, p.readhistory, req)
}

func (p *BackendProxy) forward(ctx context.Context, backend string, req *model.JSONRPCRequest) (interface{}, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", backend, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp model.JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}

	return rpcResp.Result, nil
}
