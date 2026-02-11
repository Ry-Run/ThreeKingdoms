package ws

import (
	"ThreeKingdoms/internal/gate/app"
	"ThreeKingdoms/internal/gate/app/model"
	"ThreeKingdoms/internal/gate/interfaces/handler"
	"ThreeKingdoms/internal/gate/interfaces/handler/ws/dto"
	"ThreeKingdoms/internal/shared/transport"
	"ThreeKingdoms/internal/shared/transport/ws"
	"context"
)

type WsHandler struct {
	gate *handler.Gate
}

func NewWsHandler(g *handler.Gate) *WsHandler {
	return &WsHandler{gate: g}
}

func (h *WsHandler) RegisterRoutes(r *ws.Router) {
	accountGroup := r.Group("account")
	accountGroup.Handle("login", h.Login)

	roleGroup := r.Group("role")
	roleGroup.Handle("enterServer", h.enterServer)
}

func (h *WsHandler) Login(ctx context.Context, wsReq *ws.WsMsgReq, wsResp *ws.WsMsgResp) {
	if wsReq == nil || wsReq.Body == nil || wsReq.Conn == nil || wsResp == nil || wsResp.Body == nil {
		h.fail(wsResp, transport.InvalidParam, "参数有误")
		return
	}

	var req model.LoginReq
	if err := ws.BindJSON(wsReq, &req); err != nil {
		h.fail(wsResp, transport.InvalidParam, "参数有误")
		return
	}

	loginRespDTO, err := h.gate.GateService.Login(ctx, req)
	if err != nil {
		h.error(ctx, wsResp, err)
		return
	}

	if loginRespDTO == nil {
		h.error(
			ctx,
			wsResp,
			app.ErrInternalServer.WithReason(app.ReasonUpstreamBadResponse),
		)
		return
	}

	wsReq.Conn.SetProperty(ws.ConnKeyUID, loginRespDTO.UId)
	h.gate.Session.Bind(loginRespDTO.UId, loginRespDTO.Session, wsReq.Conn)
	h.ok(wsResp, loginRespDTO)
}

func (h *WsHandler) enterServer(ctx context.Context, wsReq *ws.WsMsgReq, wsResp *ws.WsMsgResp) {
	if wsReq == nil || wsReq.Body == nil || wsReq.Conn == nil || wsResp == nil || wsResp.Body == nil {
		h.fail(wsResp, transport.InvalidParam, "参数有误")
		return
	}

	enterReqDTO := dto.EnterServerReq{}
	enterRespDTO := dto.EnterServerResp{}

	err := ws.BindJSON(wsReq, &enterReqDTO)
	if err != nil {
		h.fail(wsResp, transport.InvalidParam, err.Error())
		return
	}

	uid, ok := h.gate.Session.GetUID(wsReq.Conn)
	if !ok {
		h.fail(wsResp, transport.SessionInvalid, "session 无效")
		return
	}

	enterReq := model.EnterServerReq{Uid: uid}
	enterResp, err := h.gate.GateService.EnterServer(ctx, enterReq)
	if err != nil {
		h.error(ctx, wsResp, err)
		return
	}

	if enterResp == nil {
		h.error(
			ctx,
			wsResp,
			app.ErrInternalServer.WithReason(app.ReasonUpstreamBadResponse),
		)
		return
	}

	enterRespDTO.Role = enterResp.Role
	enterRespDTO.RoleRes = enterResp.RoleRes
	enterRespDTO.Time = enterResp.Time
	enterRespDTO.Token = enterResp.Token
	h.ok(wsResp, enterRespDTO)
}

func (h *WsHandler) ok(resp *ws.WsMsgResp, data any) {
	if resp == nil || resp.Body == nil {
		return
	}
	resp.Body.Code = transport.OK
	resp.Body.Msg = data
}

func (h *WsHandler) fail(resp *ws.WsMsgResp, code int, msg string) {
	if resp == nil || resp.Body == nil {
		return
	}
	resp.Body.Code = code
	if msg != "" {
		resp.Body.Msg = msg
	}
}

func (h *WsHandler) error(ctx context.Context, resp *ws.WsMsgResp, err error) {
	code, msg := handler.HandleError(ctx, err)
	h.fail(resp, code, msg)
}
