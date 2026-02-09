package ws

import (
	"net/http"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"ThreeKingdoms/internal/shared/logs"
)

type Server struct {
	router *Router
}

func NewServer(r *Router) *Server {
	return &Server{
		router: r,
	}
}

func (s *Server) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
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
