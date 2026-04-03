package order

import "errors"

var ErrInvalidSide = errors.New("invalid side type")

type Side uint8

const (
	SideAsk Side = 1
	SideBid Side = 2
)

func (s Side) IsValid() bool {
	return s == SideAsk || s == SideBid
}

func (s Side) Opposite() Side {
	if s == SideAsk {
		return SideBid
	}
	return SideAsk
}

func (s Side) String() string {
	if s == SideAsk {
		return "sell"
	}
	return "buy"
}
