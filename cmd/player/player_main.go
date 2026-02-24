package main

import (
	playeractor "ThreeKingdoms/internal/player/actor"
	playermongo "ThreeKingdoms/internal/player/infra/persistence/mongodb"
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	_map "ThreeKingdoms/internal/shared/gameconfig/map"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	sharedmongo "ThreeKingdoms/internal/shared/infrastructure/mongo"
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/shared/serverconfig"
	transportgrpc "ThreeKingdoms/internal/shared/transport/grpc"
	worldactor "ThreeKingdoms/internal/world/actor"
	worldmongo "ThreeKingdoms/internal/world/infra/persistence/mongodb"
	"context"
	"fmt"
	"net"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type playerGRPCServer struct {
	rt *playeractor.Runtime
	playerpb.UnimplementedPlayerServiceServer
}

func (s *playerGRPCServer) Handle(ctx context.Context, req *playerpb.PlayerRequest) (*playerpb.PlayerResponse, error) {
	if s == nil || s.rt == nil {
		return nil, fmt.Errorf("player runtime is nil")
	}
	return s.rt.Handle(ctx, req)
}

func LoadGameConfig(logger *zap.Logger) {
	basic.Load()
	_map.LoadMapBuilding()
	_map.LoadMap()
	//logger.Info()
}

func main() {
	serverconfig.Load()
	if err := logs.Init("player", serverconfig.Conf.Log); err != nil {
		panic(err)
	}
	logs.Info("conf", zap.Any("conf", serverconfig.Conf))

	logger := logs.Logger()
	LoadGameConfig(logger)

	playerHost := serverconfig.Conf.PlayerServer.Host
	if playerHost == "" {
		playerHost = "0.0.0.0"
	}
	playerAddr := fmt.Sprintf("%s:%d", playerHost, serverconfig.Conf.PlayerServer.Port)

	mongoClient, err := sharedmongo.Open(serverconfig.Conf.MongoDB, logger)
	if err != nil {
		logs.Fatal("open mongodb failed", zap.Error(err))
	}
	defer func() {
		_ = mongoClient.Disconnect(context.Background())
	}()
	db := mongoClient.Database(serverconfig.Conf.MongoDB.Database)

	worldRepo := worldmongo.NewWorldRepository(db)
	worldRT := worldactor.NewRuntime(worldRepo, 0)
	defer worldRT.Shutdown()

	repo := playermongo.NewPlayerRepo(db)
	rt := playeractor.NewRuntimeWithWorldPID(logger, worldRT.ActorSystem(), repo, worldRT.WorldActorID(), 0)
	defer rt.Shutdown()

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(transportgrpc.UnaryServerTraceInterceptor()),
		grpc.ChainStreamInterceptor(transportgrpc.StreamServerTraceInterceptor()),
	)
	playerpb.RegisterPlayerServiceServer(server, &playerGRPCServer{rt: rt})

	lis, err := net.Listen("tcp", playerAddr)
	if err != nil {
		logs.Fatal("listen player grpc failed", zap.Error(err))
	}
	defer func() {
		_ = lis.Close()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logs.Info("player grpc server started", zap.String("addr", playerAddr))
		if err := server.Serve(lis); err != nil {
			errCh <- fmt.Errorf("player grpc serve failed: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		logs.Info("收到退出信号，准备优雅退出")
	case err := <-errCh:
		if err != nil {
			logs.Error("服务异常退出", zap.Error(err))
		}
	}

	stopCh := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopCh)
	}()
	select {
	case <-stopCh:
	case <-time.After(10 * time.Second):
		server.Stop()
	}
}
