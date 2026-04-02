package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/viabtc/go-project/services/alertcenter/internal/alerter"
	"github.com/viabtc/go-project/services/alertcenter/internal/handler"
)

const magicHead = "373d26968a5a2b698045"
const magicHeadLen = 20

func decodePkg(data []byte) (int, error) {
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			size := i + 1
			if size <= magicHeadLen {
				return 0, fmt.Errorf("package too small")
			}
			if string(data[:magicHeadLen]) != magicHead {
				return 0, fmt.Errorf("invalid magic head")
			}
			return size, nil
		}
	}
	return 0, nil
}

func extractMessage(body []byte) (string, error) {
	size, err := decodePkg(body)
	if err != nil {
		return "", err
	}

	if size == 0 {
		return "", fmt.Errorf("need more data")
	}

	message := string(body[magicHeadLen:size])
	if len(message) > 0 && message[len(message)-1] == '\r' {
		message = message[:len(message)-1]
	}
	return strings.TrimSpace(message), nil
}

func magicHeadMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			c.Abort()
			return
		}

		message, err := extractMessage(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		c.Set("alert_message", message)
		c.Next()
	}
}

type Server struct {
	host    string
	port    int
	router  *gin.Engine
	handler *handler.Handler
	alerter *alerter.Alerter
	wg      sync.WaitGroup
}

func New(host string, port int, a *alerter.Alerter) *Server {
	h := handler.New(a)

	return &Server{
		host:    host,
		port:    port,
		router:  gin.Default(),
		handler: h,
		alerter: a,
	}
}

func (s *Server) Start() error {
	s.setupRoutes()

	go s.startTCPServer()
	go s.startUDPServer()

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	return s.router.Run(addr)
}

func (s *Server) startTCPServer() {
	s.wg.Add(1)
	defer s.wg.Done()

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("TCP server failed to listen: %v", err)
		return
	}
	defer ln.Close()

	log.Printf("Alertcenter TCP server listening on %s", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed") {
				return
			}
			log.Printf("TCP accept error: %v", err)
			continue
		}

		go s.handleTCP(conn)
	}
}

func (s *Server) handleTCP(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 65536)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed") {
				log.Printf("TCP read error: %v", err)
			}
			return
		}

		message, err := extractMessage(buf[:n])
		if err != nil {
			log.Printf("TCP decode error: %v", err)
			return
		}

		if err := s.alerter.SendAlert(context.Background(), message); err != nil {
			log.Printf("Failed to send alert: %v", err)
			return
		}
	}
}

func (s *Server) startUDPServer() {
	s.wg.Add(1)
	defer s.wg.Done()

	addr := fmt.Sprintf("%s:%d", s.host, s.port+1)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Printf("UDP server failed to listen: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Alertcenter UDP server listening on %s", addr)

	buf := make([]byte, 65536)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed") {
				log.Printf("UDP read error: %v", err)
			}
			return
		}

		message, err := extractMessage(buf[:n])
		if err != nil {
			log.Printf("UDP decode error: %v", err)
			continue
		}

		if err := s.alerter.SendAlert(context.Background(), message); err != nil {
			log.Printf("Failed to send alert: %v", err)
			continue
		}
	}
}

func (s *Server) Stop() {
	s.wg.Wait()
}

func (s *Server) setupRoutes() {
	s.router.POST("/alert", magicHeadMiddleware(), s.handler.HandleAlert)
	s.router.GET("/alerts/:level", s.handler.HandleGetAlerts)
	s.router.GET("/health", s.handler.HandleHealthCheck)
}
