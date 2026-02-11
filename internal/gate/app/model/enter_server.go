package model

type EnterServerReq struct {
	Uid int `json:"uid"`
}

type EnterServerResp struct {
	Role    Role     `json:"role"`
	RoleRes Resource `json:"role_res"`
	Time    int64    `json:"time"`
	Token   string   `json:"token"`
}
