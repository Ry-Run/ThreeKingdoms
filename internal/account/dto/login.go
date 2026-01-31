package dto

type LoginResp struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Session  string `json:"session"` // token
	UId      int    `json:"uid"`
}

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Ip       string `json:"ip"`
	Hardware string `json:"hardware"`
}
