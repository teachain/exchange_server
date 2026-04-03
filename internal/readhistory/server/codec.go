package server

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

const (
	RPCPkgMagic    = 0x70656562 // "keep" in little endian
	RPCPkgTypeReq  = 0
	RPCPkgTypeResp = 1
	RPCPkgTypePush = 2
	RPCPkgHeadSize = 36
)

var (
	ErrInvalidMagic   = errors.New("invalid magic")
	ErrInvalidCRC     = errors.New("invalid CRC")
	ErrBufferTooSmall = errors.New("buffer too small")
)

type RPCPkg struct {
	Magic    uint32
	Command  uint32
	PkgType  uint16
	Result   uint32
	CRC32    uint32
	Sequence uint32
	ReqID    uint64
	BodySize uint32
	ExtSize  uint16
	Body     []byte
}

func (p *RPCPkg) Pack() ([]byte, error) {
	totalSize := RPCPkgHeadSize + uint32(p.BodySize)
	buf := make([]byte, totalSize)

	binary.LittleEndian.PutUint32(buf[0:4], p.Magic)
	binary.LittleEndian.PutUint32(buf[4:8], p.Command)
	binary.LittleEndian.PutUint16(buf[8:10], p.PkgType)
	binary.LittleEndian.PutUint32(buf[10:14], p.Result)
	binary.LittleEndian.PutUint32(buf[14:18], 0) // CRC32 placeholder
	binary.LittleEndian.PutUint32(buf[18:22], p.Sequence)
	binary.LittleEndian.PutUint64(buf[22:30], p.ReqID)
	binary.LittleEndian.PutUint32(buf[30:34], p.BodySize)
	binary.LittleEndian.PutUint16(buf[34:36], p.ExtSize)

	copy(buf[RPCPkgHeadSize:], p.Body)

	crc := crc32.ChecksumIEEE(buf)
	binary.LittleEndian.PutUint32(buf[14:18], crc)

	return buf, nil
}

func (p *RPCPkg) Unpack(data []byte) error {
	if len(data) < RPCPkgHeadSize {
		return ErrBufferTooSmall
	}

	p.Magic = binary.LittleEndian.Uint32(data[0:4])
	if p.Magic != RPCPkgMagic {
		return ErrInvalidMagic
	}

	p.Command = binary.LittleEndian.Uint32(data[4:8])
	p.PkgType = binary.LittleEndian.Uint16(data[8:10])
	p.Result = binary.LittleEndian.Uint32(data[10:14])
	storedCRC := binary.LittleEndian.Uint32(data[14:18])
	p.Sequence = binary.LittleEndian.Uint32(data[18:22])
	p.ReqID = binary.LittleEndian.Uint64(data[22:30])
	p.BodySize = binary.LittleEndian.Uint32(data[30:34])
	p.ExtSize = binary.LittleEndian.Uint16(data[34:36])

	computedCRC := crc32.ChecksumIEEE(data)
	if computedCRC != storedCRC {
		return ErrInvalidCRC
	}

	if p.BodySize > 0 {
		bodyStart := int(RPCPkgHeadSize)
		bodyEnd := bodyStart + int(p.BodySize)
		if len(data) < bodyEnd {
			return ErrBufferTooSmall
		}
		p.Body = make([]byte, p.BodySize)
		copy(p.Body, data[bodyStart:bodyEnd])
	}

	return nil
}
