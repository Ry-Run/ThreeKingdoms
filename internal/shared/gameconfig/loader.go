package gameconfig

import (
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	"ThreeKingdoms/internal/shared/gameconfig/building"
	"ThreeKingdoms/internal/shared/gameconfig/facility"
	"ThreeKingdoms/internal/shared/gameconfig/general"
	_map "ThreeKingdoms/internal/shared/gameconfig/map"
	"ThreeKingdoms/internal/shared/gameconfig/skill"

	"go.uber.org/zap"
)

func LoadGameConfig(logger *zap.Logger) {
	basic.Load()
	building.Load()
	_map.Load()
	facility.Load()
	general.Load()
	skill.Load()
	//logger.Info()
}
