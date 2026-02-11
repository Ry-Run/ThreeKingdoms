package handler

import (
	"ThreeKingdoms/internal/gate/app"
	"ThreeKingdoms/internal/shared/transport"
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapBizReasonToClientCode(reason string) int {
	switch reason {
	case "":
		return transport.OK
	case app.ReasonAccountLoginInvalidCredentials.Code:
		return transport.PwdIncorrect
	case app.ReasonAccountRegisterUserExist.Code:
		return transport.UserExist
	case app.ReasonAccountRoleNotExist.Code:
		return transport.RoleNotExist
	default:
		return transport.SystemError
	}
}

func mapTechErrToClientCode(err error) int {
	if err == nil {
		return transport.OK
	}
	switch app.GetErrorReasonCode(err) {
	case app.ReasonUpstreamUnavailable.Code:
		return transport.UpstreamUnavailable
	case app.ReasonUpstreamTimeout.Code:
		return transport.UpstreamTimeout
	case app.ReasonUpstreamInternal.Code, app.ReasonUpstreamBadResponse.Code:
		return transport.UpstreamInternal
	}
	code, ok := grpcCodeFromErrorChain(err)
	if !ok {
		return transport.UpstreamInternal
	}
	switch code {
	case codes.Unavailable:
		return transport.UpstreamUnavailable
	case codes.DeadlineExceeded:
		return transport.UpstreamTimeout
	default:
		return transport.UpstreamInternal
	}
}

func grpcCodeFromErrorChain(err error) (codes.Code, bool) {
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		s, ok := status.FromError(cur)
		if !ok {
			continue
		}
		return s.Code(), true
	}
	return codes.Unknown, false
}

func HandleError(ctx context.Context, err error) (int, string) {
	reason := app.GetErrorReasonCode(err)
	if reason != "" {
		transport.SetErrorReason(ctx, reason)
	}

	if app.IsBizRejectedError(err) {
		bizCode := mapBizReasonToClientCode(reason)
		return bizCode, app.GetErrorMessage(err)
	}

	bizCode := mapTechErrToClientCode(err)
	return bizCode, "系统繁忙，请稍后重试"
}
