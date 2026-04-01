package config

import "fmt"

type ServerConfig struct {
	Host   string `mapstructure:"host"`
	Port   int    `mapstructure:"port"`
	Socket string `mapstructure:"socket"`
}

func (s ServerConfig) Addr() string {
	if s.Socket != "" {
		return s.Socket
	}
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}
