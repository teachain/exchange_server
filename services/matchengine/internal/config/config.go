package config

import "github.com/teachain/exchange_server/internal/alert"

type Config struct {
	Alert alert.AlertConfig `mapstructure:"alert"`
}
