package http

import (
	"ThreeKingdoms/internal/shared/transport/http/middleware"
	"ThreeKingdoms/modules/kit/logx"
	"context"
	nethttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct {
	engine *gin.Engine
	group  *gin.RouterGroup
	srv    *nethttp.Server
}

func NewHttpServer(add string, engine *gin.Engine, logger logx.Logger) *Server {
	if engine == nil {
		engine = gin.New()
		engine.Use(gin.Recovery())
	}
	engine.Use(middleware.Cors())
	engine.Use(middleware.AccessLog(logger))
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(nethttp.StatusOK, gin.H{"status": "ok"})
	})

	return &Server{
		engine: engine,
		group:  engine.Group(""),
		srv: &nethttp.Server{
			Addr:              add,
			Handler:           engine,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

// Start 启动 HTTP 服务（阻塞）。关闭时会返回 net/dto.ErrServerClosed。
func (s *Server) Start() error {
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) Group() *gin.RouterGroup {
	return s.group
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}
