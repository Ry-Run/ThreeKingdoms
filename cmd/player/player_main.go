package main

import (
	allianceactor "ThreeKingdoms/internal/alliance/actor"
	alliancemongo "ThreeKingdoms/internal/alliance/infra/persistence/mongodb"
	playeractor "ThreeKingdoms/internal/player/actor"
	playermongo "ThreeKingdoms/internal/player/infra/persistence/mongodb"
	sharedactor "ThreeKingdoms/internal/shared/actor"
	"ThreeKingdoms/internal/shared/gameconfig"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	sharedmongo "ThreeKingdoms/internal/shared/infrastructure/mongo"
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/shared/serverconfig"
	transportgrpc "ThreeKingdoms/internal/shared/transport/grpc"
	worldactor "ThreeKingdoms/internal/world/actor"
	worldactors "ThreeKingdoms/internal/world/actors"
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

func main() {
	serverconfig.Load()
	if err := logs.Init("player", serverconfig.Conf.Log); err != nil {
		panic(err)
	}
	logs.Info("conf", zap.Any("conf", serverconfig.Conf))

	logger := logs.Logger()
	gameconfig.LoadGameConfig(logger)

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
	managerPIDRegistry := sharedactor.NewPIDRegistry()
	pusherAddr := pusherServiceAddr(serverconfig.Conf.GateServer)
	pusherConn, pusher, err := transportgrpc.DialGatePushService(pusherAddr)
	if err != nil {
		logs.Fatal("dial gate push service failed", zap.Error(err), zap.String("addr", pusherAddr))
	}
	defer func() {
		_ = pusherConn.Close()
	}()

	worldRepo := worldmongo.NewWorldRepository(db)
	worldRT := worldactor.NewRuntime(worldRepo, managerPIDRegistry, worldactors.NewGRPCWorldPushBatchPusher(pusher), 0)
	defer worldRT.Shutdown()
	managerPIDRegistry.RegisterManagerPID(sharedactor.ManagerPIDWorld, worldRT.WorldActorID())

	worldID := serverconfig.Conf.Logic.WorldID
	if worldID <= 0 {
		logs.Fatal("logic.world_id must be greater than 0", zap.Int("world_id", worldID))
	}

	allianceRepo := alliancemongo.NewAllianceRepository(db)
	allianceRT := allianceactor.NewRuntime(worldRT.ActorSystem(), allianceRepo, worldID, 0)
	defer allianceRT.Shutdown()
	managerPIDRegistry.RegisterManagerPID(sharedactor.ManagerPIDAlliance, allianceRT.AllianceActorID())

	repo := playermongo.NewPlayerRepo(db)
	rt := playeractor.NewRuntime(logger, worldRT.ActorSystem(), repo, managerPIDRegistry, pusher, 0)
	defer rt.Shutdown()
	managerPIDRegistry.RegisterManagerPID(sharedactor.ManagerPIDPlayer, rt.PlayerMangerPID())

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

func pusherServiceAddr(cfg serverconfig.GateServerConfig) string {
	host := cfg.Host
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	port := cfg.GRPCPort
	if port <= 0 {
		port = cfg.Port + 10000
	}
	return fmt.Sprintf("%s:%d", host, port)
}
