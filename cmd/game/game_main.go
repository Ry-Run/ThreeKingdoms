package main

import (
	"ThreeKingdoms/internal/account/interfaces"
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	"ThreeKingdoms/internal/shared/infrastructure/db"
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/shared/serverconfig"
	transportgrpc "ThreeKingdoms/internal/shared/transport/grpc"
	"context"
	"fmt"
	"net"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	serverconfig.Load()
	if err := logs.Init("player", serverconfig.Conf.Log); err != nil {
		panic(err)
	}
	logs.Info("conf", zap.Any("conf", serverconfig.Conf))

	// 加载游戏配置
	basic.Load()
	logs.Info("player config ", zap.Any("basic", basic.BasicConf))

	slgServerHost := serverconfig.Conf.SLGServer.Host
	if slgServerHost == "" {
		slgServerHost = "0.0.0.0"
	}
	SLGServerAddr := fmt.Sprintf("%s:%d", slgServerHost, serverconfig.Conf.SLGServer.Port)
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(transportgrpc.UnaryServerTraceInterceptor()),
		grpc.ChainStreamInterceptor(transportgrpc.StreamServerTraceInterceptor()),
	)

	gormDB, err := db.Open(serverconfig.Conf.MySQL, logs.Logger())
	if err != nil {
		logs.Fatal("open db failed", zap.Error(err))
	}
	account := interfaces.New(gormDB, logs.Logger())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	lis, err := net.Listen("tcp", SLGServerAddr)
	if err != nil {
		logs.Fatal("listen player grpc failed", zap.Error(err))
	}
	errCh := make(chan error, 1)
	accountpb.RegisterAccountServiceServer(server, account.Account)
	go func() {
		logs.Info("player grpc server started", zap.String("addr", SLGServerAddr))
		if err := server.Serve(lis); err != nil {
			errCh <- fmt.Errorf("player grpc serve failed: %w", err)
		}
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
	_ = lis.Close()
}
