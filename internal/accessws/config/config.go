package config

import "github.com/spf13/viper"

type Config struct {
	Server       ServerConfig   `mapstructure:"server"`
	MatchEngine  RPCConfig      `mapstructure:"matchengine"`
	MarketPrice  RPCConfig      `mapstructure:"marketprice"`
	ReadHistory  RPCConfig      `mapstructure:"readhistory"`
	Kafka        KafkaConfig    `mapstructure:"kafka"`
	AuthURL      string         `mapstructure:"auth_url"`
	SignURL      string         `mapstructure:"sign_url"`
	CacheTimeout float64        `mapstructure:"cache_timeout"`
	Intervals    IntervalConfig `mapstructure:"intervals"`
	DepthLimit   []int          `mapstructure:"depth_limit"`
	DepthMerge   []string       `mapstructure:"depth_merge"`
	Alert        AlertConfig    `mapstructure:"alert"`
}

type AlertConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type ServerConfig struct {
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	WorkerNum int    `mapstructure:"worker_num"`
}

type RPCConfig struct {
	Host    string  `mapstructure:"host"`
	Port    int     `mapstructure:"port"`
	Timeout float64 `mapstructure:"timeout"`
}

type KafkaConfig struct {
	Brokers  []string       `mapstructure:"brokers"`
	Consumer ConsumerConfig `mapstructure:"consumer"`
}

type ConsumerConfig struct {
	OrdersTopic   string `mapstructure:"orders_topic"`
	BalancesTopic string `mapstructure:"balances_topic"`
	Group         string `mapstructure:"group"`
}

type IntervalConfig struct {
	Depth         float64 `mapstructure:"depth"`
	Price         float64 `mapstructure:"price"`
	Kline         float64 `mapstructure:"kline"`
	Deals         float64 `mapstructure:"deals"`
	State         float64 `mapstructure:"state"`
	Today         float64 `mapstructure:"today"`
	CleanInterval float64 `mapstructure:"clean_interval"`
}

func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	var cfg Config
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
