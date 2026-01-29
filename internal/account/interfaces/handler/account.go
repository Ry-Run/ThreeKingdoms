package handler

import (
	"ThreeKingdoms/internal/account/app"
	"ThreeKingdoms/internal/account/dto"
	ws "ThreeKingdoms/internal/shared/transport/ws"
	"ThreeKingdoms/modules/kit/logx"
	"context"
	"encoding/json"
	"errors"

	"go.uber.org/zap"
)

type Account struct {
	userService *app.UserService
	log         logx.Logger
}

func NewAccount(userRepo app.UserRepo, pwdEncrypter app.PwdEncrypter, log logx.Logger, lhRepo app.LoginHistoryRepo, llRepo app.LoginLastRepo) *Account {
	return &Account{
		userService: app.NewUserService(userRepo, pwdEncrypter, log, lhRepo, llRepo),
		log:         log,
	}
}

func (a *Account) RegisterRoutes(r *ws.Router) {
	g := r.Group("account")
	g.Handle("login", a.login)
}

func (a *Account) login(req *ws.WsMsgReq, resp *ws.WsMsgResp) {
	loginReq := &dto.LoginReq{}

	// ReqBody.Msg 由 json.Unmarshal 解码到 interface{}，通常是 map[string]any。
	// 这里用 json 二次编解码做“宽松解析”，避免 copier 的方向/类型陷阱。
	raw, err := json.Marshal(req.Body.Msg)
	if err != nil {
		a.log.Error("marshal req.Body.Msg failed", zap.Error(err), zap.Any("msg", req.Body.Msg))
		resp.Body.Code = ws.InvalidParam
		return
	}
	if err := json.Unmarshal(raw, loginReq); err != nil {
		a.log.Error("unmarshal loginReq failed", zap.Error(err), zap.ByteString("raw", raw))
		resp.Body.Code = ws.InvalidParam
		return
	}

	loginResp, err := a.userService.Login(context.Background(), *loginReq)
	if err != nil {
		ReportError("login fail", err)
		switch {
		case errors.Is(err, app.ErrInvalidCredentials):
			resp.Body.Code = ws.PwdIncorrect
		default:
			resp.Body.Code = ws.SystemError
		}
		return
	}

	resp.Body.Code = ws.OK
	resp.Body.Msg = &loginResp

}
