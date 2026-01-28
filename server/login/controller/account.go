package controller

import (
	net "ThreeKingdoms"
	"server/login/proto"
)

type Account struct {
}

func NewAccount() *Account {
	return &Account{}
}

func (a *Account) RegisterRoutes(r *net.Router) {
	g := r.Group("account")
	g.Handle("login", a.login)
}

func (a *Account) login(req net.WsMsgReq, resp *net.WsMsgResp) {
	resp.Body.Code = 0
	loginRes := &proto.LoginRsp{}
	loginRes.UId = 1
	loginRes.Username = "admin"
	loginRes.Session = "as"
	loginRes.Password = ""
	resp.Body.Msg = loginRes
}
