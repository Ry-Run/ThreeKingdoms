package port

import (
	"ThreeKingdoms/internal/world/entity"
	"context"
)

type WorldRepository interface {
	LoadWorld(ctx context.Context, id entity.WorldID) (*entity.WorldEntity, error)
	Save(ctx context.Context, s *entity.WorldEntitySnap) error
}
