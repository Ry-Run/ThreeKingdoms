package ws

import (
	"ThreeKingdoms/modules/kit/logx"
	"net/http"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Server struct {
	router *Router
	log    logx.Logger
}

func NewServer(r *Router, l logx.Logger) *Server {
	return &Server{
		router: r,
		log:    l,
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
		s.log.Error("websocket upgrade error", zap.Error(err))
		return
	}

	s.log.Info("websocket upgrade success")

	wsServer := NewWsServer(wsConn, s.log)
	wsServer.Router(s.router)
	wsServer.Run()
	wsServer.handshake()
}
