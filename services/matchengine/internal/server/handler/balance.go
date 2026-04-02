package handler

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/viabtc/go-project/services/matchengine/internal/model"
	"github.com/viabtc/go-project/services/matchengine/internal/server"
)

const (
	CMD_BALANCE_QUERY  = 101
	CMD_BALANCE_UPDATE = 102
	CMD_ASSET_LIST     = 104
	CMD_ASSET_SUMMARY  = 105
)

var knownAssets = []string{"BTC", "ETH", "USDT"}

func AssetPrec(asset string) int {
	switch asset {
	case "BTC", "ETH":
		return 8
	case "USDT":
		return 8
	default:
		return 8
	}
}

type BalanceInfo struct {
	Available string `json:"available"`
	Freeze    string `json:"freeze"`
}

type AssetInfo struct {
	Name string `json:"name"`
	Prec int    `json:"prec"`
}

type AssetSummary struct {
	Name             string `json:"name"`
	TotalBalance     string `json:"total_balance"`
	AvailableCount   int    `json:"available_count"`
	AvailableBalance string `json:"available_balance"`
	FreezeCount      int    `json:"freeze_count"`
	FreezeBalance    string `json:"freeze_balance"`
}

func HandleBalanceQuery(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("invalid arguments")
	}

	userID, ok := params[0].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user_id")
	}
	if userID == 0 {
		return nil, fmt.Errorf("invalid user_id")
	}

	result := make(map[string]BalanceInfo)
	engine := s.GetEngine()

	if len(params) == 1 {
		balances := engine.GetAllBalancesForUser(int64(userID))
		for asset, bal := range balances {
			result[asset] = BalanceInfo{
				Available: bal.Balance.Sub(bal.Frozen).String(),
				Freeze:    bal.Frozen.String(),
			}
		}
		for _, asset := range knownAssets {
			if _, exists := balances[asset]; !exists {
				result[asset] = BalanceInfo{
					Available: "0",
					Freeze:    "0",
				}
			}
		}
	} else {
		for i := 1; i < len(params); i++ {
			asset, ok := params[i].(string)
			if !ok {
				return nil, fmt.Errorf("invalid asset name")
			}
			prec := AssetPrec(asset)
			balance, frozen := engine.GetBalance(int64(userID), asset)

			result[asset] = BalanceInfo{
				Available: adjustPrecision(balance.Sub(frozen), prec).String(),
				Freeze:    adjustPrecision(frozen, prec).String(),
			}
		}
	}

	return json.Marshal(result)
}

func HandleBalanceUpdate(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []interface{}
	if err := json.Unmarshal(pkg.Body, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if len(params) != 6 {
		return nil, fmt.Errorf("invalid arguments: expected 6 params")
	}

	userID, ok := params[0].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid user_id")
	}

	asset, ok := params[1].(string)
	if !ok {
		return nil, fmt.Errorf("invalid asset")
	}

	business, ok := params[2].(string)
	if !ok {
		return nil, fmt.Errorf("invalid business")
	}

	_, ok = params[3].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid business_id")
	}

	changeStr, ok := params[4].(string)
	if !ok {
		return nil, fmt.Errorf("invalid change")
	}

	change, err := decimal.NewFromString(changeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid change amount")
	}

	detail, ok := params[5].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid detail")
	}

	_ = business
	_ = detail

	engine := s.GetEngine()
	prec := AssetPrec(asset)

	if change.IsPositive() {
		engine.GetBalances().AddBalance(int64(userID), asset, adjustPrecision(change, prec))
	} else if change.IsNegative() {
		err := engine.GetBalances().DeductBalance(int64(userID), asset, adjustPrecision(change.Abs(), prec))
		if err != nil {
			return nil, fmt.Errorf("balance not enough")
		}
	}

	return json.Marshal(map[string]string{"status": "success"})
}

func HandleAssetList(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	_ = pkg

	engine := s.GetEngine()
	assets := engine.GetAllAssets()

	result := make([]AssetInfo, 0, len(assets))
	for _, asset := range assets {
		result = append(result, AssetInfo{
			Name: asset.Name,
			Prec: asset.Prec,
		})
	}

	return json.Marshal(result)
}

func HandleAssetSummary(s *server.RPCServer, pkg *server.RPCPkg) ([]byte, error) {
	var params []string
	if len(pkg.Body) > 0 {
		if err := json.Unmarshal(pkg.Body, &params); err != nil {
			return nil, fmt.Errorf("invalid params")
		}
	}

	engine := s.GetEngine()
	result := make([]AssetSummary, 0)

	var assets []*model.AssetConfig
	if len(params) == 0 {
		assets = engine.GetAllAssets()
	} else {
		for _, name := range params {
			if asset, ok := engine.GetAsset(name); ok {
				assets = append(assets, asset)
			}
		}
	}

	for _, asset := range assets {
		balances := engine.GetAllBalancesForAsset(asset.Name)

		var totalBalance, availableBalance, freezeBalance decimal.Decimal
		var availableCount, freezeCount int

		for _, bal := range balances {
			totalBalance = totalBalance.Add(bal.Balance)
			if bal.Frozen.IsZero() {
				availableCount++
				availableBalance = availableBalance.Add(bal.Balance)
			} else {
				freezeCount++
				freezeBalance = freezeBalance.Add(bal.Frozen)
			}
		}

		result = append(result, AssetSummary{
			Name:             asset.Name,
			TotalBalance:     adjustPrecision(totalBalance, asset.Prec).String(),
			AvailableCount:   availableCount,
			AvailableBalance: adjustPrecision(availableBalance, asset.Prec).String(),
			FreezeCount:      freezeCount,
			FreezeBalance:    adjustPrecision(freezeBalance, asset.Prec).String(),
		})
	}

	return json.Marshal(result)
}

func adjustPrecision(d decimal.Decimal, prec int) decimal.Decimal {
	if prec <= 0 {
		return d
	}
	multiplier := decimal.NewFromInt(10)
	for i := 0; i < prec; i++ {
		multiplier = multiplier.Mul(decimal.NewFromInt(10))
	}
	truncated := d.Mul(multiplier).Floor().Div(multiplier)
	return truncated
}

func RegisterBalanceHandlers(s *server.RPCServer) {
	s.Handle(CMD_BALANCE_QUERY, HandleBalanceQuery)
	s.Handle(CMD_BALANCE_UPDATE, HandleBalanceUpdate)
	s.Handle(CMD_ASSET_LIST, HandleAssetList)
	s.Handle(CMD_ASSET_SUMMARY, HandleAssetSummary)
}
