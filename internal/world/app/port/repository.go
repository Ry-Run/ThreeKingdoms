package port

import (
	"ThreeKingdoms/internal/world/entity"
	"context"
)

type WorldRepository interface {
	LoadWorld(ctx context.Context, id *entity.WorldID) (*entity.World, error)
	Save(ctx context.Context, s *entity.WorldPersistSnapshot) error
}
