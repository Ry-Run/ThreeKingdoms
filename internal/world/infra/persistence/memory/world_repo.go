package memory

import (
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	"ThreeKingdoms/internal/shared/serverconfig"
	"ThreeKingdoms/internal/world/entity"
	"context"
	"encoding/json"
	"os"
)

type WorldRepository struct{}

func NewWorldRepository() *WorldRepository {
	return &WorldRepository{}
}

func (r *WorldRepository) LoadWorld(ctx context.Context, id *entity.WorldID) (*entity.World, error) {
	_ = ctx
	worldID := id
	if worldID == nil {
		defaultID := entity.WorldID(1)
		worldID = &defaultID
	}
	return entity.NewWorld(worldID, loadNationMapPayload()), nil
}

func (r *WorldRepository) Save(ctx context.Context, s *entity.WorldPersistSnapshot) error {
	_ = ctx
	_ = s
	return nil
}

func loadNationMapPayload() string {
	mapPath := serverconfig.Conf.Logic.MapData
	if mapPath != "" {
		if raw, err := os.ReadFile(mapPath); err == nil && len(raw) > 0 {
			return string(raw)
		}
	}

	if raw, err := json.Marshal(basic.BasicConf); err == nil && len(raw) > 0 {
		return string(raw)
	}
	return "{}"
}
