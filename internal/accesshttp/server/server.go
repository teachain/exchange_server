package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/teachain/exchange_server/internal/accesshttp/config"
	"github.com/teachain/exchange_server/internal/accesshttp/handler"
	"github.com/teachain/exchange_server/internal/accesshttp/proxy"
)

type Server struct {
	cfg     *config.Config
	httpSrv *http.Server
	router  *gin.Engine
	handler *handler.Handler
	proxy   *proxy.BackendProxy
}

func New(cfg *config.Config, proxy *proxy.BackendProxy) *Server {
	return &Server{
		cfg:     cfg,
		router:  gin.Default(),
		handler: handler.New(proxy),
		proxy:   proxy,
	}
}

func (s *Server) GetBackendProxy() *proxy.BackendProxy {
	return s.proxy
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
