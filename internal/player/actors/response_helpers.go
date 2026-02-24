package actors

import (
	playerpb "ThreeKingdoms/internal/shared/gen/player"
)

func ok() *playerpb.PlayerResponse {
	return OK()
}

func fail(reason string) *playerpb.PlayerResponse {
	return Fail(reason)
}
