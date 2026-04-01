package rpc

import (
	"encoding/binary"
	"errors"
)

const (
	PKG_TYPE_REQUEST  = 0
	PKG_TYPE_RESPONSE = 1
)

const (
	CMD_BALANCE_QUERY       = 101
	CMD_BALANCE_HISTORY     = 103
	CMD_ORDER_QUERY         = 203
	CMD_ORDER_HISTORY       = 208
	CMD_ORDER_BOOK_DEPTH    = 206
	CMD_MARKET_KLINE        = 302
	CMD_MARKET_DEALS        = 303
	CMD_MARKET_LAST         = 304
	CMD_MARKET_STATUS       = 301
	CMD_MARKET_STATUS_TODAY = 305
)

var (
	ErrInvalidPackage = errors.New("invalid rpc package")
)

type Package struct {
	PkgType  uint32
	Command  uint32
	Sequence uint32
	ReqID    uint64
	Body     []byte
}

func (p *Package) Serialize() []byte {
	buf := make([]byte, 20+len(p.Body))
	binary.LittleEndian.PutUint32(buf[0:4], p.PkgType)
	binary.LittleEndian.PutUint32(buf[4:8], p.Command)
	binary.LittleEndian.PutUint32(buf[8:12], p.Sequence)
	binary.LittleEndian.PutUint64(buf[12:20], p.ReqID)
	copy(buf[20:], p.Body)
	return buf
}

func ParsePackage(data []byte) (*Package, error) {
	if len(data) < 20 {
		return nil, ErrInvalidPackage
	}
	return &Package{
		PkgType:  binary.LittleEndian.Uint32(data[0:4]),
		Command:  binary.LittleEndian.Uint32(data[4:8]),
		Sequence: binary.LittleEndian.Uint32(data[8:12]),
		ReqID:    binary.LittleEndian.Uint64(data[12:20]),
		Body:     data[20:],
	}, nil
}
