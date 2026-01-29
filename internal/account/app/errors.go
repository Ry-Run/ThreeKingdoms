package app

import "ThreeKingdoms/modules/kit/errx"

// Code 表示应用层错误码（通常更贴近“业务语义/对外协议”）。
type Code = errx.Code

const (
	CodeInvalidCredentials Code = "AUTH_INVALID_CREDENTIAL"
	// CodeInternalServer 复用 kit 的统一系统码（跨服务一致，便于告警/排障）。
	CodeInternalServer Code = errx.CodeInternal
	// CodeUnavailable 复用 kit 的统一系统码（跨服务一致，便于告警/排障）。
	CodeUnavailable Code = errx.CodeUnavailable
)

// Error 复用通用错误模型：对外语义(code/msg)、上下文(data)、溯源链(cause)、系统错误一次栈(stack)。
type Error = errx.Error

// NewError 创建业务类错误（不捕获栈）。
func NewError(code Code, msg string) *Error {
	return errx.NewBiz(code, msg)
}

// Wrap 创建系统类错误并挂载 cause（系统错误会在第一次 wrap/转换处捕获一次栈）。
func Wrap(code Code, msg string, cause error) *Error {
	return errx.NewSys(code, msg).WithCause(cause)
}

// 常用错误定义（哨兵错误）：禁止直接修改其 data/cause（通过 WithData/WithCause 派生新对象）。
var (
	ErrInvalidCredentials = errx.NewBiz(CodeInvalidCredentials, "用户名或密码错误")
	ErrInternalServer     = errx.ErrInternal
	ErrUnavailable        = errx.ErrUnavailable
)
