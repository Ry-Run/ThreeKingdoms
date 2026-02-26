package actors

import (
	commonpb "ThreeKingdoms/internal/shared/gen/common"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"ThreeKingdoms/internal/shared/reasoncode"
)

func ok() *playerpb.PlayerResponse {
	return OK()
}

func fail(reason string) *playerpb.PlayerResponse {
	return Fail(reason)
}

func failBiz(reason, message string) *playerpb.PlayerResponse {
	if message == "" {
		message = reason
	}
	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{
			Ok:      false,
			Reason:  reason,
			Message: message,
		},
	}
}

func failRoleNotExist() *playerpb.PlayerResponse {
	return failBiz(reasoncode.AccountRoleNotExist, "角色不存在")
}
