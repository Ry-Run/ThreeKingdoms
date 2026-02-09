package app

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func wrapTechErr(err error) error {
	if err == nil {
		return nil
	}
	s, ok := status.FromError(err)
	if !ok {
		return ErrInternalServer.WithReason(ReasonUpstreamInternal).WithCause(err)
	}
	switch s.Code() {
	case codes.Unavailable:
		return ErrUnavailable.WithReason(ReasonUpstreamUnavailable).WithCause(err)
	case codes.DeadlineExceeded:
		return ErrUnavailable.WithReason(ReasonUpstreamTimeout).WithCause(err)
	default:
		return ErrInternalServer.WithReason(ReasonUpstreamInternal).WithCause(err)
	}
}
