package serverconfig

import (
	"ThreeKingdoms/internal/shared/config"
	"os"
)

const defaultConfigRelPath = "configs/conf.yml"

var Conf Config

func Load() {
	config.Load(defaultConfigRelPath, &Conf)
	// 环境变量优先；若未设置则回填配置中的 jwt_secret，兼容本地开发场景。
	if os.Getenv("JWT_SECRET") == "" && Conf.JWTSecret != "" {
		_ = os.Setenv("JWT_SECRET", Conf.JWTSecret)
	}
}
