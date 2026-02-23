package port

import (
	"ThreeKingdoms/internal/player/entity"
	"context"
)

type PlayerRepository interface {
	LoadPlayer(ctx context.Context, id entity.PlayerID) (*entity.PlayerEntity, error)
	Save(ctx context.Context, s *entity.PlayerEntitySnap) error
}
