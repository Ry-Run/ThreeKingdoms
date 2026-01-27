package net

import (
	"common/logs"
	"net/http"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Server struct {
	addr   string
	router *router
}

func New(add string) *Server {
	return &Server{
		addr: add,
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
	upgrade, err := upgrader.Upgrade(resp, req, nil)
	if err != nil {
		logs.Panic("websocket upgrade error", zap.Error(err))
	}

	logs.Info("websocket upgrade success")
	upgrade.WriteMessage(websocket.BinaryMessage, []byte("ok"))
}
