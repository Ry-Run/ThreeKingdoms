package handler

import (
	"ThreeKingdoms/internal/gate/app"
	"ThreeKingdoms/internal/gate/app/model"
	"ThreeKingdoms/internal/gate/interfaces/handler/dto"
	"ThreeKingdoms/internal/shared/session"
	"ThreeKingdoms/internal/shared/transport"
	"ThreeKingdoms/internal/shared/transport/ws"
	"context"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
)

// ============ Types ============

type Gate struct {
	session     session.Manager
	gateService *app.GateService
}

type WsHandler struct {
	gate *Gate
}

type HttpHandler struct {
	gate *Gate
}

// ============ Constructors ============

func NewGate(s session.Manager, accountServiceClient app.AccountServiceClient) *Gate {
	gate := Gate{
		session: s,
	}
	gate.gateService = app.NewGateService(accountServiceClient)
	return &gate
}

func NewWsHandler(g *Gate) *WsHandler {
	return &WsHandler{gate: g}
}

func NewHttpHandler(g *Gate) *HttpHandler {
	return &HttpHandler{gate: g}
}

// ============ Route Registration ============

func (h *WsHandler) RegisterRoutes(r *ws.Router) {
	accountGroup := r.Group("account")
	accountGroup.Handle("login", h.Login)
}

func (h *HttpHandler) RegisterRoutes(group *gin.RouterGroup) {
	accountGroup := group.Group("/account")
	accountGroup.POST("/register", h.Register)
}

// ============ WS Handlers ============

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

	loginRespDTO, err := h.gate.gateService.Login(ctx, req)
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
	h.gate.session.Bind(loginRespDTO.UId, loginRespDTO.Session, wsReq.Conn)
	h.ok(wsResp, loginRespDTO)
}

// ============ HTTP Handlers ============

func (h *HttpHandler) Register(c *gin.Context) {
	ctx := c.Request.Context()

	var req model.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, transport.InvalidParam, "参数有误")
		return
	}

	if err := h.gate.gateService.Register(ctx, req); err != nil {
		h.error(ctx, c, err)
		return
	}
	h.ok(c, nil)
}

// ============ Shared Logic ============

func (g *Gate) handleError(ctx context.Context, err error) (int, string) {
	reason := app.GetErrorReasonCode(err)
	if reason != "" {
		transport.SetErrorReason(ctx, reason)
	}

	if app.IsBizRejectedError(err) {
		bizCode := mapBizReasonToClientCode(reason)
		return bizCode, app.GetErrorMessage(err)
	}

	bizCode := mapTechErrToClientCode(err)
	return bizCode, "系统繁忙，请稍后重试"
}

// ============ Response Helpers ============

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
	code, msg := h.gate.handleError(ctx, err)
	h.fail(resp, code, msg)
}

func (h *HttpHandler) ok(c *gin.Context, data any) {
	c.JSON(nethttp.StatusOK, dto.Success(transport.OK, data))
}

func (h *HttpHandler) fail(c *gin.Context, code int, msg string) {
	c.JSON(nethttp.StatusOK, dto.Error(code, msg))
}

func (h *HttpHandler) error(ctx context.Context, c *gin.Context, err error) {
	code, msg := h.gate.handleError(ctx, err)
	h.fail(c, code, msg)
}
