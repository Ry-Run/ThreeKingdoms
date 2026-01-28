package ws

import (
	"strings"

	"go.uber.org/zap"

	"ThreeKingdoms/internal/shared/logs"
)

type Group struct {
	prefix   string
	handlers map[string]HandlerFunc
}

type HandlerFunc func(req WsMsgReq, resp *WsMsgResp)

func (g *Group) Handle(name string, h HandlerFunc) {
	g.handlers[name] = h
}

type Router struct {
	groups map[string]*Group
}

func NewRouter() *Router {
	return &Router{
		groups: make(map[string]*Group),
	}
}

func (r *Router) Group(prefix string) *Group {
	g := &Group{
		prefix:   prefix,
		handlers: make(map[string]HandlerFunc),
	}
	r.groups[prefix] = g
	return g
}

// req.Body.Name(路径)：例如，登录业务 account(组标识).login(路由标识)
func (r *Router) Dispatch(req WsMsgReq, resp *WsMsgResp) {
	split := strings.Split(req.Body.Name, ".")
	if len(split) < 2 {
		logs.Error("Router Dispatch err, len(split) < 2", zap.String("name", req.Body.Name))
		return
	}
	prefix := split[0]
	handler := split[1]
	handlerFunc := r.groups[prefix].handlers[handler]
	if handlerFunc == nil {
		logs.Error("Router Dispatch err, handler not exist", zap.String("prefix", prefix), zap.String("handler", handler))
		return
	}
	handlerFunc(req, resp)
}
