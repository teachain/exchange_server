package config

import "github.com/teachain/exchange_server/pkg/alert"

type Config struct {
	Alert alert.AlertConfig `mapstructure:"alert"`
}
