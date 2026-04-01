package server

import (
	"fmt"
	"io"
	"net/http"

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

func magicHeadMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			c.Abort()
			return
		}

		size, err := decodePkg(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		if size == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "need more data"})
			c.Abort()
			return
		}

		message := string(body[magicHeadLen:size])
		if len(message) > 0 && message[len(message)-1] == '\r' {
			message = message[:len(message)-1]
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
}

func New(host string, port int, a *alerter.Alerter) *Server {
	h := handler.New(a)

	return &Server{
		host:    host,
		port:    port,
		router:  gin.Default(),
		handler: h,
	}
}

func (s *Server) Start() error {
	s.setupRoutes()
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	return s.router.Run(addr)
}

func (s *Server) setupRoutes() {
	s.router.POST("/alert", magicHeadMiddleware(), s.handler.HandleAlert)
	s.router.GET("/alerts/:level", s.handler.HandleGetAlerts)
	s.router.GET("/health", s.handler.HandleHealthCheck)
}
