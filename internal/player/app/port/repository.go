package port

import (
	"ThreeKingdoms/internal/player/entity"
	"context"
)

type PlayerRepository interface {
	LoadPlayer(ctx context.Context, id *entity.PlayerID) (*entity.Player, error)
	Snapshot(ctx context.Context, s *entity.PlayerPersistSnapshot) error
}
