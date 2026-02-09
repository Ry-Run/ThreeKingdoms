package app

import (
	"ThreeKingdoms/modules/kit/errx"
	"errors"
	"fmt"
)

const codeBizRejected = "BIZ_REJECTED"

var (
	// ErrUnavailable 表示下游依赖不可用。
	ErrUnavailable = errx.ErrUnavailable
	// ErrInternalServer 表示网关内部技术错误。
	ErrInternalServer = errx.ErrInternal
)

type BizRejectedError struct {
	reason Reason
}

func newBizRejectedError(reasonCode, message string) error {
	return &BizRejectedError{
		reason: NewReason(reasonCode, message),
	}
}

func (e *BizRejectedError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.reason.Message == "" {
		return fmt.Sprintf("%s: %s", codeBizRejected, e.reason.Code)
	}
	return fmt.Sprintf("%s: %s: %s", codeBizRejected, e.reason.Code, e.reason.Message)
}

func (e *BizRejectedError) CodeText() string {
	if e == nil {
		return ""
	}
	return codeBizRejected
}

func (e *BizRejectedError) Reason() string {
	if e == nil {
		return ""
	}
	return e.reason.Code
}

func (e *BizRejectedError) Msg() string {
	if e == nil {
		return ""
	}
	return e.reason.Message
}

func IsBizRejectedError(err error) bool {
	var bizErr *BizRejectedError
	return errors.As(err, &bizErr)
}

func GetErrorReasonCode(err error) string {
	var rp interface{ Reason() string }
	if !errors.As(err, &rp) {
		return ""
	}
	return rp.Reason()
}

func GetErrorMessage(err error) string {
	var mp interface{ Msg() string }
	if !errors.As(err, &mp) {
		return ""
	}
	return mp.Msg()
}
