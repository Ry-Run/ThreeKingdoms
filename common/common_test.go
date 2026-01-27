package common

import (
	"common/config"
	"common/logs"
	"testing"

	"go.uber.org/zap"
)

func TestReadConfig(t *testing.T) {
	config.Load("./conf/conf.yml")
	//log.Println(config.Conf)
	logs.Init("TestReadConfig", config.Conf.Log)
	logs.Info("conf", zap.Any("conf", config.Conf))
}
