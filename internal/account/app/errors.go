package app

import (
	"ThreeKingdoms/modules/kit/errx"
	"errors"
)

var (
	ErrInternalServer = errx.ErrInternal
	ErrUnavailable    = errx.ErrUnavailable
	ErrTimeout        = errx.ErrTimeout
	ErrRateLimited    = errx.ErrRateLimited
	ErrReqParamERR    = errx.ErrReqParamERR
)

func GetErrorReasonCode(err error) string {
	var rp interface{ Reason() string }
	if !errors.As(err, &rp) {
		return ""
	}
	return rp.Reason()
}
