package cmd

import (
	"ThreeKingdoms/internal/shared/serverconfig"
	"testing"

	"ThreeKingdoms/internal/shared/logs"

	"go.uber.org/zap"
)

func TestReadConfig(t *testing.T) {
	serverconfig.Load()
	//log.Println(config.Conf)
	logs.Init("TestReadConfig", serverconfig.Conf.Log)
	logs.Info("conf", zap.Any("conf", serverconfig.Conf))
}
