package model

import "github.com/shopspring/decimal"

type MarketConfig struct {
	Name      string          `mapstructure:"name"`
	Stock     string          `mapstructure:"stock"`
	Money     string          `mapstructure:"money"`
	StockPrec int             `mapstructure:"stock_prec"`
	MoneyPrec int             `mapstructure:"money_prec"`
	FeePrec   int             `mapstructure:"fee_prec"`
	MinAmount decimal.Decimal `mapstructure:"min_amount"`
}

type AssetConfig struct {
	Name     string `mapstructure:"name"`
	Prec     int    `mapstructure:"prec"`
	PrecShow int    `mapstructure:"prec_show"`
}

type Config struct {
	Markets []*MarketConfig `mapstructure:"markets"`
	Assets  []*AssetConfig  `mapstructure:"assets"`
}
