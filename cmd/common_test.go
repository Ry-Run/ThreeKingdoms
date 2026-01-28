package cmd

import (
	"testing"

	"ThreeKingdoms/internal/shared/config"
	"ThreeKingdoms/internal/shared/logs"

	"go.uber.org/zap"
)

func TestReadConfig(t *testing.T) {
	config.Load("")
	//log.Println(config.Conf)
	logs.Init("TestReadConfig", config.Conf.Log)
	logs.Info("conf", zap.Any("conf", config.Conf))
}
