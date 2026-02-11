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
	// 业务拒绝 reason（服务内枚举），由 gate 统一映射为客户端 client_code。
	ReasonLoginInvalidCredentials = NewReason(reasoncode.AccountLoginInvalidCredentials, "用户名或密码错误")
	ReasonRegisterUserExist       = NewReason(reasoncode.AccountRegisterUserExist, "用户已存在")
	ReasonRoleNotExist            = NewReason(reasoncode.AccountRoleNotExist, "角色不存在")
)

var (
	// 技术错误 reason（服务内枚举），用于日志与排障。
	ReasonUserRepoUnavailable   = NewReason("USER_REPO_UNAVAILABLE", "用户存储库不可用")
	ReasonRoleRepoUnavailable   = NewReason("ROLE_REPO_UNAVAILABLE", "角色存储库不可用")
	ReasonTokenIssue            = NewReason("TOKEN_ISSUE", "令牌签发失败")
	ReasonLoginHistoryWriteFail = NewReason("LOGIN_HISTORY_WRITE_FAIL", "登录历史写入失败")
	ReasonLoginLastReadFail     = NewReason("LOGIN_LAST_READ_FAIL", "最后登录读取失败")
	ReasonLoginLastWriteFail    = NewReason("LOGIN_LAST_WRITE_FAIL", "最后登录写入失败")
	ReasonUserCreateFail        = NewReason("USER_CREATE_FAIL", "用户创建失败")
)
