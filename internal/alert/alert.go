package alert

import (
	"fmt"
	"net"
	"time"
)

const MagicHead = "373d26968a5a2b698045"
const MaxMsgSize = 10240

type AlertConfig struct {
	Host string
	Port int
}

type Alerter struct {
	host string
	port int
	conn *net.UDPConn
}

func NewAlerter(cfg AlertConfig) (*Alerter, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	return &Alerter{
		host: cfg.Host,
		port: cfg.Port,
		conn: conn,
	}, nil
}

func (a *Alerter) SendAlert(format string, args ...interface{}) error {
	if a.conn == nil {
		return fmt.Errorf("alerter not initialized")
	}

	msg := fmt.Sprintf("%s: %s\n", a.host, fmt.Sprintf(format, args...))
	if len(msg) > MaxMsgSize {
		msg = msg[:MaxMsgSize]
	}

	a.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	_, err := a.conn.Write([]byte(msg))
	return err
}

func (a *Alerter) Close() error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}
