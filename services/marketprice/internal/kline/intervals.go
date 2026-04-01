package kline

type Interval string

const (
	Interval1m  Interval = "1m"
	Interval5m  Interval = "5m"
	Interval15m Interval = "15m"
	Interval1h  Interval = "1h"
	Interval4h  Interval = "4h"
	Interval1d  Interval = "1d"
)

func (i Interval) Seconds() int64 {
	switch i {
	case Interval1m:
		return 60
	case Interval5m:
		return 300
	case Interval15m:
		return 900
	case Interval1h:
		return 3600
	case Interval4h:
		return 14400
	case Interval1d:
		return 86400
	default:
		return 60
	}
}

func (i Interval) ToTimestamp(ts int64) int64 {
	return (ts / i.Seconds()) * i.Seconds()
}
