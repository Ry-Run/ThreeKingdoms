package controller

import (
	proto "ThreeKingdoms/internal/account/dto"
	ws "ThreeKingdoms/internal/shared/transport/ws"
)

type Account struct {
}

func NewAccount() *Account {
	return &Account{}
}

func (a *Account) RegisterRoutes(r *ws.Router) {
	g := r.Group("account")
	g.Handle("login", a.login)
}

func (a *Account) login(req ws.WsMsgReq, resp *ws.WsMsgResp) {
	resp.Body.Code = 0
	loginRes := &proto.LoginRsp{}
	loginRes.UId = 1
	loginRes.Username = "admin"
	loginRes.Session = "as"
	loginRes.Password = ""
	resp.Body.Msg = loginRes
}
