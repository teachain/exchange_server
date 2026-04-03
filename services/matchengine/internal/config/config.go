package config

import "github.com/viabtc/go-project/internal/alert"

type Config struct {
	Alert alert.AlertConfig `mapstructure:"alert"`
}
