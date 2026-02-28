package port

import (
	"ThreeKingdoms/internal/alliance/entity"
	"context"
)

type AllianceRepository interface {
	LoadAlliance(ctx context.Context, allianceID entity.AllianceID) (*entity.AllianceEntity, error)
	ListAllianceSummaryByWorld(ctx context.Context, worldID entity.WorldID) ([]entity.AllianceState, error)
	Save(ctx context.Context, s *entity.AllianceEntitySnap) error
}
