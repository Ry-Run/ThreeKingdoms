package ws

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"ThreeKingdoms/internal/shared/logs"
)

type Server struct {
	addr   string
	router *Router
	srv    *http.Server
}

func NewServer(add string, r *Router) *Server {
	return &Server{
		addr:   add,
		router: r,
		srv: &http.Server{
			Addr:              add,
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

// Start 启动 WebSocket 服务（阻塞）。关闭时会返回 net/http.ErrServerClosed。
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.wsHandle)
	s.srv.Handler = mux
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) wsHandle(resp http.ResponseWriter, req *http.Request) {
	upgrader := websocket.Upgrader{
		// 允许所有CORS跨域请求
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	wsConn, err := upgrader.Upgrade(resp, req, nil)
	if err != nil {
		logs.Error("websocket upgrade error", zap.Error(err))
		return
	}

	logs.Info("websocket upgrade success")

	wsServer := NewWsServer(wsConn)
	wsServer.Router(s.router)
	wsServer.Run()
	wsServer.handshake()
}
