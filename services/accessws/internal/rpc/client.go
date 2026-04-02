package rpc

import (
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	addr    string
	timeout time.Duration
	conn    net.Conn
	mu      sync.Mutex
	seq     uint32
}

func NewClient(addr string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	return &Client{
		addr:    addr,
		timeout: timeout,
		conn:    conn,
		seq:     0,
	}, nil
}

func (c *Client) NextSeq() uint32 {
	c.seq++
	return c.seq
}

func (c *Client) Send(cmd uint32, reqID uint64, body []byte) (*Package, error) {
	c.mu.Lock()
	seq := c.NextSeq()
	pkg := &Package{
		PkgType:  PKG_TYPE_REQUEST,
		Command:  cmd,
		Sequence: seq,
		ReqID:    reqID,
		Body:     body,
	}
	_, err := c.conn.Write(pkg.Serialize())
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 65536)
	c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return ParsePackage(buf[:n])
}

func (c *Client) Close() error {
	return c.conn.Close()
}

type RPCCLient struct {
	matchEngine string
	marketPrice string
	readHistory string
	timeout     time.Duration
	lastReqID   uint64
	reqIDMu     sync.Mutex

	meClient  *Client
	mpClient  *Client
	rhClient  *Client
	clientsMu sync.Mutex
}

func NewRPCClient(matchEngine, marketPrice, readHistory string, timeout time.Duration) *RPCCLient {
	return &RPCCLient{
		matchEngine: matchEngine,
		marketPrice: marketPrice,
		readHistory: readHistory,
		timeout:     timeout,
	}
}

func (r *RPCCLient) getMatchEngineClient() (*Client, error) {
	r.clientsMu.Lock()
	defer r.clientsMu.Unlock()

	if r.meClient != nil {
		return r.meClient, nil
	}

	client, err := NewClient(r.matchEngine, r.timeout)
	if err != nil {
		return nil, err
	}
	r.meClient = client
	return client, nil
}

func (r *RPCCLient) getMarketPriceClient() (*Client, error) {
	r.clientsMu.Lock()
	defer r.clientsMu.Unlock()

	if r.mpClient != nil {
		return r.mpClient, nil
	}

	client, err := NewClient(r.marketPrice, r.timeout)
	if err != nil {
		return nil, err
	}
	r.mpClient = client
	return client, nil
}

func (r *RPCCLient) getReadHistoryClient() (*Client, error) {
	r.clientsMu.Lock()
	defer r.clientsMu.Unlock()

	if r.rhClient != nil {
		return r.rhClient, nil
	}

	client, err := NewClient(r.readHistory, r.timeout)
	if err != nil {
		return nil, err
	}
	r.rhClient = client
	return client, nil
}

func (r *RPCCLient) Close() {
	r.clientsMu.Lock()
	defer r.clientsMu.Unlock()

	if r.meClient != nil {
		r.meClient.Close()
		r.meClient = nil
	}
	if r.mpClient != nil {
		r.mpClient.Close()
		r.mpClient = nil
	}
	if r.rhClient != nil {
		r.rhClient.Close()
		r.rhClient = nil
	}
}

func (r *RPCCLient) NextReqID() uint64 {
	r.reqIDMu.Lock()
	id := atomic.AddUint64(&r.lastReqID, 1)
	r.reqIDMu.Unlock()
	return id
}

func (r *RPCCLient) QueryMatchEngine(cmd uint32, body []byte) (*Package, error) {
	client, err := r.getMatchEngineClient()
	if err != nil {
		return nil, err
	}
	return client.Send(cmd, r.NextReqID(), body)
}

func (r *RPCCLient) QueryMarketPrice(cmd uint32, body []byte) (*Package, error) {
	client, err := r.getMarketPriceClient()
	if err != nil {
		return nil, err
	}
	return client.Send(cmd, r.NextReqID(), body)
}

func (r *RPCCLient) QueryReadHistory(cmd uint32, body []byte) (*Package, error) {
	client, err := r.getReadHistoryClient()
	if err != nil {
		return nil, err
	}
	return client.Send(cmd, r.NextReqID(), body)
}

func BuildBalanceQueryBody(userID uint32, asset string) []byte {
	m := map[string]interface{}{
		"user_id": userID,
		"asset":   asset,
	}
	b, _ := json.Marshal(m)
	return b
}

func BuildOrderQueryBody(userID uint32, market string, limit int) []byte {
	m := map[string]interface{}{
		"user_id": userID,
		"market":  market,
		"limit":   limit,
	}
	b, _ := json.Marshal(m)
	return b
}

func BuildDepthQueryBody(market string, limit int) []byte {
	m := map[string]interface{}{
		"market": market,
		"limit":  limit,
	}
	b, _ := json.Marshal(m)
	return b
}

func BuildKlineQueryBody(market, interval string, limit int) []byte {
	m := map[string]interface{}{
		"market":   market,
		"interval": interval,
		"limit":    limit,
	}
	b, _ := json.Marshal(m)
	return b
}

func BuildDealsQueryBody(market string, limit int, since int64) []byte {
	m := map[string]interface{}{
		"market": market,
		"limit":  limit,
		"since":  since,
	}
	b, _ := json.Marshal(m)
	return b
}

func BuildStateQueryBody(market string) []byte {
	m := map[string]interface{}{
		"market": market,
	}
	b, _ := json.Marshal(m)
	return b
}

func BuildTodayQueryBody(market string) []byte {
	m := map[string]interface{}{
		"market": market,
	}
	b, _ := json.Marshal(m)
	return b
}
