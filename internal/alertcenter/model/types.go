package model

import "time"

type Alert struct {
	Level     string    `json:"level"`
	Service   string    `json:"service"`
	Message   string    `json:"message"`
	Trace     string    `json:"trace,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type AlertList string

const (
	AlertListFatal AlertList = "alerts:fatal"
	AlertListError AlertList = "alerts:error"
	AlertListWarn  AlertList = "alerts:warn"
)
