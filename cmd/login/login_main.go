package main

import (
	"ThreeKingdoms/internal/account/interfaces"
	"ThreeKingdoms/internal/shared/config"
	accountpb "ThreeKingdoms/internal/shared/gen/account"
	"ThreeKingdoms/internal/shared/infrastructure/db"
	"ThreeKingdoms/internal/shared/logs"
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
	config.Load("")
	if err := logs.Init("login", config.Conf.Log); err != nil {
		panic(err)
	}
	logs.Info("conf", zap.Any("conf", config.Conf))

	loginServerHost := config.Conf.LoginServer.Host
	if loginServerHost == "" {
		loginServerHost = "0.0.0.0"
	}
	loginServerAddr := fmt.Sprintf("%s:%d", loginServerHost, config.Conf.LoginServer.Port)
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(transportgrpc.UnaryServerTraceInterceptor()),
		grpc.ChainStreamInterceptor(transportgrpc.StreamServerTraceInterceptor()),
	)

	gormDB, err := db.Open(config.Conf.MySQL)
	if err != nil {
		logs.Fatal("open db failed", zap.Error(err))
	}
	account := interfaces.New(gormDB, logs.Logger())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	lis, err := net.Listen("tcp", loginServerAddr)
	if err != nil {
		logs.Fatal("listen login grpc failed", zap.Error(err))
	}
	errCh := make(chan error, 1)
	accountpb.RegisterAccountServiceServer(server, account.Account)
	go func() {
		logs.Info("login grpc server started", zap.String("addr", loginServerAddr))
		if err := server.Serve(lis); err != nil {
			errCh <- fmt.Errorf("login grpc serve failed: %w", err)
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
