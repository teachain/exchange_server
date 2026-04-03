package config

import (
	"time"

	"strconv"

	"github.com/spf13/viper"
)

type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Workers int           `mapstructure:"workers"`
	Backend BackendConfig `mapstructure:"backend"`
	Timeout time.Duration `mapstructure:"timeout"`
	Alert   AlertConfig   `mapstructure:"alert"`
	Log     LogConfig     `mapstructure:"log"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type AlertConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

func (s ServerConfig) Addr() string {
	return s.Host + ":" + strconv.Itoa(s.Port)
}

type BackendConfig struct {
	MatchEngine string `mapstructure:"matchengine"`
	MarketPrice string `mapstructure:"marketprice"`
	ReadHistory string `mapstructure:"readhistory"`
}

func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
