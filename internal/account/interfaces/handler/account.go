package handler

import (
	"ThreeKingdoms/internal/account/app"
	"ThreeKingdoms/internal/account/app/model"
	"ThreeKingdoms/internal/account/interfaces/handler/dto"
	"ThreeKingdoms/internal/shared/session"
	"ThreeKingdoms/internal/shared/transport"
	"ThreeKingdoms/internal/shared/transport/ws"
	"ThreeKingdoms/modules/kit/logx"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Account struct {
	userService *app.UserService
	log         logx.Logger
	session     session.Manager
}

func NewAccount(userRepo app.UserRepo, pwdEncrypter app.PwdEncrypter, log logx.Logger,
	lhRepo app.LoginHistoryRepo, llRepo app.LoginLastRepo, session session.Manager, randSeq app.RandSeq) *Account {
	return &Account{
		userService: app.NewUserService(userRepo, pwdEncrypter, log, lhRepo, llRepo, randSeq),
		log:         log,
		session:     session,
	}
}

func (a *Account) RegisterWsRoutes(r *ws.Router) {
	g := r.Group("account")
	g.Handle("login", a.login)
}

func (a *Account) RegisterHttpRoutes(g *gin.RouterGroup) {
	g.Group("/account").GET("/register", a.register)
}

func (a *Account) login(req *ws.WsMsgReq, resp *ws.WsMsgResp) {
	loginReq := &model.LoginReq{}

	// ReqBody.Msg 由 json.Unmarshal 解码到 interface{}，通常是 map[string]any。
	// 这里用 json 二次编解码做“宽松解析”，避免 copier 的方向/类型陷阱。
	raw, err := json.Marshal(req.Body.Msg)
	if err != nil {
		a.log.Error("marshal req.Body.Msg failed", zap.Error(err), zap.Any("msg", req.Body.Msg))
		resp.Body.Code = transport.InvalidParam
		return
	}
	if err := json.Unmarshal(raw, loginReq); err != nil {
		a.log.Error("unmarshal loginReq failed", zap.Error(err), zap.ByteString("raw", raw))
		resp.Body.Code = transport.InvalidParam
		return
	}

	loginResp, err := a.userService.Login(context.Background(), *loginReq)
	if err != nil {
		ReportError("login fail", err)
		switch {
		case errors.Is(err, app.ErrInvalidCredentials):
			resp.Body.Code = transport.PwdIncorrect
		default:
			resp.Body.Code = transport.SystemError
		}
		return
	}

	resp.Body.Code = transport.OK
	resp.Body.Msg = loginResp

	// 缓存 ws连接 和当前用户数据
	req.Conn.SetProperty(ws.ConnKeyUID, loginResp.UId)
	a.session.Bind(loginResp.UId, loginResp.Session, req.Conn)
}

func (a *Account) register(c *gin.Context) {
	var req model.RegisterReq
	err := c.Bind(&req)
	if err != nil {
		a.log.Error("register request param invalidate", zap.Error(err))
		c.JSON(http.StatusOK, dto.Error(transport.InvalidParam, "param invalidate"))
		return
	}

	err = a.userService.Register(context.Background(), req)
	if err != nil {
		ReportError("register fail", err)
		switch {
		case errors.Is(err, app.ErrUserExist):
			c.JSON(http.StatusOK, dto.Error(transport.UserExist, "user exist"))
		default:
			c.JSON(http.StatusOK, dto.Error(transport.DBError, "db error"))
		}
		return
	}
	c.JSON(http.StatusOK, dto.Success(transport.OK, nil))
}
