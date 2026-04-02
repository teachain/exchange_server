package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/viabtc/go-project/services/accesshttp/internal/config"
	"github.com/viabtc/go-project/services/accesshttp/internal/handler"
	"github.com/viabtc/go-project/services/accesshttp/internal/proxy"
)

type Server struct {
	cfg     *config.Config
	httpSrv *http.Server
	router  *gin.Engine
	handler *handler.Handler
}

func New(cfg *config.Config) *Server {
	proxy := proxy.NewBackendProxy(cfg)
	return &Server{
		cfg:     cfg,
		router:  gin.Default(),
		handler: handler.New(proxy),
	}
}

func (s *Server) Start() error {
	s.setupRoutes()
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.httpSrv = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) setupRoutes() {
	s.router.POST("/", s.handler.HandleJSONRPC)
	s.router.GET("/health", s.handler.HealthCheck)
}
