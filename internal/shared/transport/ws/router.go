package ws

import (
	"ThreeKingdoms/internal/shared/transport"
	"ThreeKingdoms/modules/kit/logx"
	"context"
	"strings"

	"ThreeKingdoms/internal/shared/logs"
)

type Group struct {
	prefix   string
	handlers map[string]HandlerFunc
}

type HandlerFunc func(ctx context.Context, req *WsMsgReq, resp *WsMsgResp)

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

// req.Body.Name(路径)：例如，登录业务 account(组标识).login(路由标识)
func (r *Router) Dispatch(req *WsMsgReq, resp *WsMsgResp) {
	ctx := r.prepareDispatchContext(req, resp)
	defer r.writeAccessLog(ctx, resp)

	if !r.validateDispatchInput(req, resp) {
		return
	}

	handlerFunc := r.findHandler(req.Body.Name, resp)
	if handlerFunc == nil {
		return
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

func (r *Router) findHandler(route string, resp *WsMsgResp) HandlerFunc {
	prefix, handler, ok := parseRouteName(route)
	if !ok {
		r.setErrorResponse(resp, transport.InvalidParam, "路由参数有误")
		return nil
	}

	group := r.groups[prefix]
	if group == nil {
		r.setErrorResponse(resp, transport.InvalidParam, "路由组不存在")
		return nil
	}

	handlerFunc := group.handlers[handler]
	if handlerFunc == nil {
		r.setErrorResponse(resp, transport.InvalidParam, "路由处理器不存在")
		return nil
	}
	return handlerFunc
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
