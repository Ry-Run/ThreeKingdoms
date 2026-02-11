package dto

import "ThreeKingdoms/internal/gate/app/model"

type EnterServerReq struct {
	Session string `json:"session"`
}

type EnterServerResp struct {
	Role    model.Role     `json:"role"`
	RoleRes model.Resource `json:"role_res"`
	Time    int64          `json:"time"`
	Token   string         `json:"token"`
}
