package main

import (
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	sharedmongo "ThreeKingdoms/internal/shared/infrastructure/mongo"
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/shared/serverconfig"
	worldactors "ThreeKingdoms/internal/world/actors"
	worldmongo "ThreeKingdoms/internal/world/infra/persistence/mongodb"
	"context"
	"fmt"
	"os/signal"
	"syscall"

	protoactor "github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"go.uber.org/zap"
)

const worldActorName = "world"

func main() {
	serverconfig.Load()
	if err := logs.Init("world", serverconfig.Conf.Log); err != nil {
		panic(err)
	}
	logs.Info("conf", zap.Any("conf", serverconfig.Conf))

	basic.Load()

	worldHost := serverconfig.Conf.WorldServer.Host
	if worldHost == "" {
		worldHost = "0.0.0.0"
	}
	worldPort := serverconfig.Conf.WorldServer.Port
	worldAddr := fmt.Sprintf("%s:%d", worldHost, worldPort)

	mongoClient, err := sharedmongo.Open(serverconfig.Conf.MongoDB, logs.Logger())
	if err != nil {
		logs.Fatal("open mongodb failed", zap.Error(err))
	}
	defer func() {
		_ = mongoClient.Disconnect(context.Background())
	}()
	db := mongoClient.Database(serverconfig.Conf.MongoDB.Database)

	repo := worldmongo.NewWorldRepository(db)

	system := protoactor.NewActorSystem()
	root := system.Root
	props := protoactor.PropsFromProducer(func() protoactor.Actor {
		return worldactors.NewManagerActor(repo)
	})
	worldPID, err := root.SpawnNamed(props, worldActorName)
	if err != nil {
		logs.Fatal("spawn world actor failed", zap.Error(err))
	}

	remoting := remote.NewRemote(system, remote.Configure(worldHost, worldPort))
	remoting.Start()
	logs.Info("world actor remote started",
		zap.String("addr", worldAddr),
		zap.String("actor", worldActorName),
		zap.String("pid", worldPID.String()),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	logs.Info("收到退出信号，准备优雅退出")
	remoting.Shutdown(true)
	system.Shutdown()
}
