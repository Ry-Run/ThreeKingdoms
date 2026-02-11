package http

import (
	"ThreeKingdoms/internal/gate/app/model"
	"ThreeKingdoms/internal/gate/interfaces/handler"
	"ThreeKingdoms/internal/gate/interfaces/handler/http/dto"
	"ThreeKingdoms/internal/shared/transport"
	"context"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
)

type HttpHandler struct {
	gate *handler.Gate
}

func NewHttpHandler(g *handler.Gate) *HttpHandler {
	return &HttpHandler{gate: g}
}

func (h *HttpHandler) RegisterRoutes(group *gin.RouterGroup) {
	accountGroup := group.Group("/account")
	accountGroup.POST("/register", h.Register)
}

func (h *HttpHandler) Register(c *gin.Context) {
	ctx := c.Request.Context()

	var req model.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, transport.InvalidParam, "参数有误")
		return
	}

	if err := h.gate.GateService.Register(ctx, req); err != nil {
		h.error(ctx, c, err)
		return
	}
	h.ok(c, nil)
}

func (h *HttpHandler) ok(c *gin.Context, data any) {
	c.JSON(nethttp.StatusOK, dto.Success(transport.OK, data))
}

func (h *HttpHandler) fail(c *gin.Context, code int, msg string) {
	c.JSON(nethttp.StatusOK, dto.Error(code, msg))
}

func (h *HttpHandler) error(ctx context.Context, c *gin.Context, err error) {
	code, msg := handler.HandleError(ctx, err)
	h.fail(c, code, msg)
}
