package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/viabtc/go-project/services/accesshttp/internal/config"
	"github.com/viabtc/go-project/services/accesshttp/internal/model"
)

type BackendProxy struct {
	matchenginePool *Pool
	marketpricePool *Pool
	readhistoryPool *Pool
}

func NewBackendProxy(cfg *config.Config) *BackendProxy {
	return &BackendProxy{
		matchenginePool: NewPool("http://"+cfg.Backend.MatchEngine, 10),
		marketpricePool: NewPool("http://"+cfg.Backend.MarketPrice, 10),
		readhistoryPool: NewPool("http://"+cfg.Backend.ReadHistory, 10),
	}
}

func (p *BackendProxy) ForwardToMatchEngine(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	return p.forward(ctx, p.matchenginePool, req)
}

func (p *BackendProxy) ForwardToBalance(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	switch req.Method {
	case "balance.history":
		return p.forwardJSON(ctx, p.readhistoryPool, "/balance_history", req)
	case "balance.query":
		return p.queryBalance(ctx, req)
	case "balance.update":
		return p.forward(ctx, p.matchenginePool, req)
	default:
		return p.forward(ctx, p.matchenginePool, req)
	}
}

func (p *BackendProxy) forwardJSON(ctx context.Context, pool *Pool, path string, req *model.JSONRPCRequest) (interface{}, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", pool.backend+path, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := pool.GetClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf(errMsg)
	}

	return result["result"], nil
}

func (p *BackendProxy) queryBalance(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	var params []json.RawMessage
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &model.RPCError{Code: -32602, Message: "Invalid params"}
	}
	if len(params) < 2 {
		return nil, &model.RPCError{Code: -32602, Message: "Invalid params"}
	}

	var userID float64
	if err := json.Unmarshal(params[0], &userID); err != nil {
		return nil, &model.RPCError{Code: -32602, Message: "Invalid user_id"}
	}
	var asset string
	if err := json.Unmarshal(params[1], &asset); err != nil {
		return nil, &model.RPCError{Code: -32602, Message: "Invalid asset"}
	}

	url := fmt.Sprintf("%s/balance/%d/%s", p.matchenginePool.backend, int(userID), asset)
	httpReq, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := p.matchenginePool.GetClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	balance := result["balance"].(string)
	frozen := result["frozen"].(string)

	return map[string]map[string]string{
		asset: {
			"available": balance,
			"freeze":    frozen,
		},
	}, nil
}

func (p *BackendProxy) ForwardToAsset(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	switch req.Method {
	case "asset.list":
		return p.forwardGet(ctx, p.matchenginePool, "/asset/list", req)
	case "asset.summary":
		return p.forwardGet(ctx, p.matchenginePool, "/asset/summary", req)
	default:
		return p.forward(ctx, p.matchenginePool, req)
	}
}

func (p *BackendProxy) ForwardToMarketPrice(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	switch req.Method {
	case "market.list":
		return p.forwardGet(ctx, p.marketpricePool, "/markets", req)
	case "market.summary":
		var params []string
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &model.RPCError{Code: -32602, Message: "Invalid params"}
		}
		market := ""
		if len(params) > 0 {
			market = params[0]
		}
		return p.forwardGet(ctx, p.marketpricePool, "/summary/"+market, req)
	default:
		return p.forward(ctx, p.marketpricePool, req)
	}
}

func (p *BackendProxy) forwardGet(ctx context.Context, pool *Pool, path string, req *model.JSONRPCRequest) (interface{}, error) {
	httpReq, _ := http.NewRequestWithContext(ctx, "GET", pool.backend+path, nil)

	resp, err := pool.GetClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

func (p *BackendProxy) ForwardToReadHistory(ctx context.Context, req *model.JSONRPCRequest) (interface{}, error) {
	return p.forward(ctx, p.readhistoryPool, req)
}

func (p *BackendProxy) forward(ctx context.Context, pool *Pool, req *model.JSONRPCRequest) (interface{}, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", pool.backend, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := pool.GetClient().Do(httpReq)
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

func (p *BackendProxy) Close() {
	p.matchenginePool.Close()
	p.marketpricePool.Close()
	p.readhistoryPool.Close()
}
