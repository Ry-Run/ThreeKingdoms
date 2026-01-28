package main

import (
	"ThreeKingdoms/internal/account/interfaces"
	"ThreeKingdoms/internal/shared/config"
	"ThreeKingdoms/internal/shared/infrastructure/db"
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/shared/transport/ws"

	"go.uber.org/zap"
)

func main() {
	config.Load("")
	config.Conf.Log.FileDir = ""
	logs.Init("TestReadConfig", config.Conf.Log)
	logs.Info("conf", zap.Any("conf", config.Conf))

	addr := "127.0.0.1:8080"
	r := ws.NewRouter()
	modules := []ws.Registrar{
		interfaces.New(),
	}
	for _, m := range modules {
		m.Register(r)
	}
	db.Open(config.Conf.MySQL)
	s := ws.NewServer(addr, r)
	s.Run()
}
