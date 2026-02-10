package serverconfig

import "ThreeKingdoms/internal/shared/config"

const defaultConfigRelPath = "configs/conf.yml"

var Conf Config

func Load() {
	config.Load(defaultConfigRelPath, &Conf)
}
