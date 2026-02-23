package actors

import (
	"ThreeKingdoms/internal/player/service"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
)

func ok() *playerpb.PlayerResponse {
	return service.OK()
}

func fail(reason string) *playerpb.PlayerResponse {
	return service.Fail(reason)
}
