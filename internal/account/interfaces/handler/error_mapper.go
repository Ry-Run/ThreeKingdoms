package handler

import (
	"ThreeKingdoms/internal/account/app"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func toRPCError(err error) error {
	reason := app.GetErrorReasonCode(err)
	if reason == app.ReasonTokenIssue.Code {
		return status.Error(codes.Internal, err.Error())
	}

	switch {
	case errors.Is(err, app.ErrReqParamERR):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, app.ErrRateLimited):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, app.ErrTimeout):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case errors.Is(err, app.ErrUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
