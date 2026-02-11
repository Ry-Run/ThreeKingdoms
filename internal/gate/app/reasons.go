package app

import "ThreeKingdoms/internal/shared/reasoncode"

type Reason struct {
	Code    string
	Message string
}

func (r Reason) ReasonCode() string {
	return r.Code
}

func NewReason(c, m string) Reason {
	return Reason{
		Code:    c,
		Message: m,
	}
}

var (
	// 上游技术错误 reason（全局约定）。
	ReasonUpstreamUnavailable = NewReason("UPSTREAM_UNAVAILABLE", "下游服务不可用")
	ReasonUpstreamTimeout     = NewReason("UPSTREAM_TIMEOUT", "下游服务超时")
	ReasonUpstreamInternal    = NewReason("UPSTREAM_INTERNAL", "下游服务内部错误")
	ReasonUpstreamBadResponse = NewReason("UPSTREAM_BAD_RESPONSE", "下游返回异常")
)

var (
	// account 服务业务 reason（协议常量，避免依赖 account 包）。
	ReasonAccountLoginInvalidCredentials = NewReason(reasoncode.AccountLoginInvalidCredentials, "用户名或密码错误")
	ReasonAccountRegisterUserExist       = NewReason(reasoncode.AccountRegisterUserExist, "用户已存在")
	ReasonAccountRoleNotExist            = NewReason(reasoncode.AccountRoleNotExist, "角色不存在")
)
