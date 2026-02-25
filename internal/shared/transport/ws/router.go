package ws

import (
	"ThreeKingdoms/internal/shared/transport"
	"ThreeKingdoms/modules/kit/logx"
	"context"
	"strings"
	"sync"

	"ThreeKingdoms/internal/shared/logs"
)

type Group struct {
	sync.RWMutex
	prefix        string
	handlers      map[string]HandlerFunc
	middlewareMap map[string][]MiddlewareFunc // 某个handler的中间件
	middlewares   []MiddlewareFunc            // 组中间件
}

type HandlerFunc func(ctx context.Context, req *WsMsgReq, resp *WsMsgResp)

type MiddlewareFunc func(next HandlerFunc) HandlerFunc

func (g *Group) Use(middlewares ...MiddlewareFunc) {
	g.middlewares = append(g.middlewares, middlewares...)
}

func (g *Group) HandleWithMiddleware(name string, handlerFunc HandlerFunc, middlewares ...MiddlewareFunc) {
	g.Lock()
	defer g.Unlock()
	g.handlers[name] = handlerFunc
	g.middlewareMap[name] = middlewares
}

func (g *Group) Handle(name string, h HandlerFunc) {
	g.handlers[name] = h
}

type Router struct {
	groups map[string]*Group
	log    logx.Logger
}

func NewRouter(l logx.Logger) *Router {
	if l == nil {
		l = logx.NewZapLogger(logs.Logger())
	}
	return &Router{
		groups: make(map[string]*Group),
		log:    l,
	}
}

func (r *Router) Group(prefix string) *Group {
	group := r.groups[prefix]
	if group == nil {
		group = &Group{
			prefix:   prefix,
			handlers: make(map[string]HandlerFunc),
		}
	}
	r.groups[prefix] = group
	return group
}

// req.Body.Name(路径)：例如，登录业务 account(组标识).account(路由标识)
func (r *Router) Dispatch(req *WsMsgReq, resp *WsMsgResp) {
	ctx := r.prepareDispatchContext(req, resp)
	defer r.writeAccessLog(ctx, resp)

	if !r.validateDispatchInput(req, resp) {
		return
	}

	g, router, handlerFunc := r.findHandler(req.Body.Name, resp)
	if handlerFunc == nil {
		return
	}

	// 顺序执行组全局的中间件
	for _, middleware := range g.middlewares {
		handlerFunc = middleware(handlerFunc)
	}

	// 顺序执行此 Handler 方法的中间件
	routerMiddlewares := g.middlewareMap[router]
	for _, middleware := range routerMiddlewares {
		handlerFunc = middleware(handlerFunc)
	}

	handlerFunc(ctx, req, resp)
}

func (r *Router) prepareDispatchContext(req *WsMsgReq, resp *WsMsgResp) context.Context {
	action := "WS unknown"
	if req != nil && req.Body != nil {
		action = "WS " + req.Body.Name
	}
	ctx := transport.NewContext(action)

	if resp != nil && resp.Body != nil {
		// 先置系统错误，避免 handler 漏设时出现“成功假象”。
		resp.Body.Code = transport.SystemError
		resp.Body.Msg = nil
	}
	return ctx
}

func (r *Router) validateDispatchInput(req *WsMsgReq, resp *WsMsgResp) bool {
	if req != nil && req.Body != nil && resp != nil && resp.Body != nil {
		return true
	}
	r.setErrorResponse(resp, transport.InvalidParam, "参数有误")
	return false
}

func (r *Router) findHandler(route string, resp *WsMsgResp) (*Group, string, HandlerFunc) {
	prefix, handler, ok := parseRouteName(route)
	if !ok {
		r.setErrorResponse(resp, transport.InvalidParam, "路由参数有误")
		return nil, "", nil
	}

	group := r.groups[prefix]
	if group == nil {
		r.setErrorResponse(resp, transport.InvalidParam, "路由组不存在")
		return nil, "", nil
	}

	handlerFunc := group.handlers[handler]
	if handlerFunc == nil {
		r.setErrorResponse(resp, transport.InvalidParam, "路由处理器不存在")
		return nil, "", nil
	}
	return group, handler, handlerFunc
}

func parseRouteName(name string) (string, string, bool) {
	split := strings.Split(name, ".")
	if len(split) != 2 {
		return "", "", false
	}
	prefix := split[0]
	handler := split[1]
	if prefix == "" || handler == "" {
		return "", "", false
	}
	return prefix, handler, true
}

func (r *Router) setErrorResponse(resp *WsMsgResp, code int, msg string) {
	if resp == nil || resp.Body == nil {
		return
	}
	resp.Body.Code = code
	resp.Body.Msg = msg
}

func (r *Router) writeAccessLog(ctx context.Context, resp *WsMsgResp) {
	bizCode := transport.SystemError
	if resp != nil && resp.Body != nil {
		bizCode = resp.Body.Code
	}
	transport.SetBizCode(ctx, transport.BizCode(bizCode))
	transport.WriteAccessLog(ctx, r.log)
}
