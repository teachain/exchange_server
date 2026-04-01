package test

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"testing"
	"time"
)

const (
	CMD_BALANCE_HISTORY       = 103
	CMD_ORDER_HISTORY         = 208
	CMD_ORDER_DEALS           = 209
	CMD_ORDER_DETAIL_FINISHED = 210
	CMD_MARKET_USER_DEALS     = 306
)

type RPCClient struct {
	conn net.Conn
}

func NewRPCClient(addr string) (*RPCClient, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &RPCClient{conn: conn}, nil
}

func (c *RPCClient) Close() error {
	return c.conn.Close()
}

func (c *RPCClient) SendRequest(cmd uint32, params []interface{}) ([]byte, error) {
	body, _ := json.Marshal(params)

	pkg := struct {
		Magic    uint32
		Command  uint32
		PkgType  uint16
		Result   uint32
		CRC32    uint32
		Sequence uint32
		ReqID    uint64
		BodySize uint32
		ExtSize  uint16
	}{
		Magic:    0x70656562,
		Command:  cmd,
		PkgType:  0,
		Sequence: 1,
		ReqID:    1,
		BodySize: uint32(len(body)),
	}

	buf := make([]byte, 36)
	binary.LittleEndian.PutUint32(buf[0:4], pkg.Magic)
	binary.LittleEndian.PutUint32(buf[4:8], pkg.Command)
	binary.LittleEndian.PutUint16(buf[8:10], pkg.PkgType)
	binary.LittleEndian.PutUint32(buf[10:14], pkg.Result)
	binary.LittleEndian.PutUint32(buf[14:18], 0)
	binary.LittleEndian.PutUint32(buf[18:22], pkg.Sequence)
	binary.LittleEndian.PutUint64(buf[22:30], pkg.ReqID)
	binary.LittleEndian.PutUint32(buf[30:34], pkg.BodySize)
	binary.LittleEndian.PutUint16(buf[34:36], pkg.ExtSize)

	buf = append(buf, body...)

	_, err := c.conn.Write(buf)
	if err != nil {
		return nil, err
	}

	resp := make([]byte, 4096)
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := c.conn.Read(resp)
	if err != nil {
		return nil, err
	}

	return resp[36:n], nil
}

func TestRPCIntegration(t *testing.T) {
	t.Skip("Integration test requires running server and database")
}
