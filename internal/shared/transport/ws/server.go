package ws

import (
	"net/http"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"ThreeKingdoms/internal/shared/logs"
)

type Server struct {
	addr   string
	router *Router
}

func NewServer(add string, r *Router) *Server {
	return &Server{
		addr:   add,
		router: r,
	}
}

func (s *Server) Run() {
	http.HandleFunc("/", s.wsHandle)
	http.ListenAndServe(s.addr, nil)
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
		logs.Panic("websocket upgrade error", zap.Error(err))
	}

	logs.Info("websocket upgrade success")

	wsServer := NewWsServer(wsConn)
	wsServer.Router(s.router)
	wsServer.Run()
	wsServer.handshake()
}
