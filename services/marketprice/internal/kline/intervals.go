package kline

type Interval string

const (
	Interval1s  Interval = "1s"
	Interval1m  Interval = "1m"
	Interval5m  Interval = "5m"
	Interval15m Interval = "15m"
	Interval30m Interval = "30m"
	Interval1h  Interval = "1h"
	Interval2h  Interval = "2h"
	Interval4h  Interval = "4h"
	Interval6h  Interval = "6h"
	Interval12h Interval = "12h"
	Interval1d  Interval = "1d"
	Interval1w  Interval = "1w"
	Interval1M  Interval = "1M"
)

func (i Interval) Seconds() int64 {
	switch i {
	case Interval1s:
		return 1
	case Interval1m:
		return 60
	case Interval5m:
		return 300
	case Interval15m:
		return 900
	case Interval30m:
		return 1800
	case Interval1h:
		return 3600
	case Interval2h:
		return 7200
	case Interval4h:
		return 14400
	case Interval6h:
		return 21600
	case Interval12h:
		return 43200
	case Interval1d:
		return 86400
	case Interval1w:
		return 604800
	case Interval1M:
		return 2592000
	default:
		return 60
	}
}

func (i Interval) ToTimestamp(ts int64) int64 {
	return (ts / i.Seconds()) * i.Seconds()
}

func (i Interval) IsValid() bool {
	switch i {
	case Interval1s, Interval1m, Interval5m, Interval15m, Interval30m,
		Interval1h, Interval2h, Interval4h, Interval6h, Interval12h,
		Interval1d, Interval1w, Interval1M:
		return true
	}
	return false
}

func ParseInterval(s string) Interval {
	return Interval(s)
}
