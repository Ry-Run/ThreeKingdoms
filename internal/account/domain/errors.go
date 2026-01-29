package domain

import "ThreeKingdoms/modules/kit/errx"

// Code 表示领域错误码（对外语义的唯一来源之一）。
//
// 约定：
// - 领域层只关心“是什么错”（code）以及“业务上下文”（data）
// - cause 仅用于溯源/日志，不参与对外语义
type Code = errx.Code

const (
	CodeUserNotFound    Code = "ACCOUNT_USER_NOT_FOUND"
	CodeInvalidPassword Code = "ACCOUNT_INVALID_PASSWORD"
	CodeUserDisabled    Code = "ACCOUNT_USER_DISABLED"
	// CodeSystemUnavailable 复用 kit 的统一系统码（跨服务一致，便于告警/排障）。
	CodeSystemUnavailable Code = errx.CodeUnavailable
)

// Error 复用通用错误模型：领域层通常不需要 msg，但可以使用 code/data/cause/stack。
type Error = errx.Error

func NewError(code Code, data map[string]any, cause error) *Error {
	base := newByCodeKind(code)
	if data != nil {
		base = base.WithDataMap(data)
	}
	if cause != nil {
		base = base.WithCause(cause)
	}
	return base
}

var (
	ErrUserNotFound      = errx.NewBiz(CodeUserNotFound, "")
	ErrInvalidPassword   = errx.NewBiz(CodeInvalidPassword, "")
	ErrUserDisabled      = errx.NewBiz(CodeUserDisabled, "")
	ErrSystemUnavailable = errx.ErrUnavailable
)

func newByCodeKind(code Code) *Error {
	switch code {
	case CodeSystemUnavailable:
		return errx.ErrUnavailable
	default:
		return errx.NewBiz(code, "")
	}
}
