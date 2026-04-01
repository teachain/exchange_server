package order

import "errors"

var ErrInvalidSide = errors.New("invalid side type")

type Side int

const (
	SideBuy  Side = 0
	SideSell Side = 1
)

func (s Side) IsValid() bool {
	return s == SideBuy || s == SideSell
}

func (s Side) Opposite() Side {
	if s == SideBuy {
		return SideSell
	}
	return SideBuy
}

func (s Side) String() string {
	if s == SideBuy {
		return "buy"
	}
	return "sell"
}
