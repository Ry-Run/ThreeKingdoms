package main

import (
	net "ThreeKingdoms"
	"common/config"
	"common/logs"
	"db"
	"server/login"

	"go.uber.org/zap"
)

func main() {
	config.Load("../common/conf/conf.yml")
	config.Conf.Log.FileDir = ""
	logs.Init("TestReadConfig", config.Conf.Log)
	logs.Info("conf", zap.Any("conf", config.Conf))

	addr := "127.0.0.1:8080"
	r := net.NewRouter()
	modules := []net.Registrar{
		login.New(),
	}
	for _, m := range modules {
		m.Register(r)
	}
	db.Open(config.Conf.MySQL)
	s := net.NewServer(addr, r)
	s.Run()
}
