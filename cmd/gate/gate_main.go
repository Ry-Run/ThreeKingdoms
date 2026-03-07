package main

import (
	"ThreeKingdoms/internal/gate/interfaces"
	gategrpc "ThreeKingdoms/internal/gate/interfaces/grpc"
	gatepb "ThreeKingdoms/internal/shared/gen/gate"
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/shared/serverconfig"
	"ThreeKingdoms/internal/shared/session"
	"ThreeKingdoms/internal/shared/transport/grpc"
	transporthttp "ThreeKingdoms/internal/shared/transport/http"
	"ThreeKingdoms/internal/shared/transport/ws"
	"ThreeKingdoms/modules/kit/logx"
	"context"
	"errors"
	"fmt"
	"net"
	nethttp "net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	googlegrpc "google.golang.org/grpc"
)

func main() {
	serverconfig.Load()
	if err := logs.Init("gate", serverconfig.Conf.Log); err != nil {
		panic(err)
	}
	logs.Info("conf", zap.Any("conf", serverconfig.Conf))

	serverConfig := serverconfig.Conf.GateServer
	gateHost := serverConfig.Host
	if gateHost == "" {
		gateHost = "0.0.0.0"
	}
	gateServerAddr := fmt.Sprintf("%s:%d", gateHost, serverConfig.Port)

	sessMgr := session.NewSessMgr()
	baseLogger := logx.NewZapLogger(logs.Logger())
	wsRouter := ws.NewRouter(baseLogger)

	loginServerHost := serverconfig.Conf.LoginServer.Host
	if loginServerHost == "" {
		loginServerHost = "0.0.0.0"
	}
	loginServerAddr := fmt.Sprintf("%s:%d", loginServerHost, serverconfig.Conf.LoginServer.Port)

	accountConn, accountClient, err := grpc.DialAccountService(loginServerAddr)
	if err != nil {
		logs.Fatal("dial account service failed", zap.Error(err))
	}
	defer func() {
		_ = accountConn.Close()
	}()

	playerServerHost := serverconfig.Conf.PlayerServer.Host
	if playerServerHost == "" {
		playerServerHost = "0.0.0.0"
	}
	playerServerAddr := fmt.Sprintf("%s:%d", playerServerHost, serverconfig.Conf.PlayerServer.Port)

	playerConn, playerClient, err := grpc.DialPlayerService(playerServerAddr)
	if err != nil {
		logs.Fatal("dial player service failed", zap.Error(err))
	}
	defer func() {
		_ = playerConn.Close()
	}()

	accountModule := interfaces.New(sessMgr, accountClient, playerClient)
	wsModules := []ws.Registrar{
		accountModule,
	}
	for _, m := range wsModules {
		m.WsRegister(wsRouter)
	}

	httpServer := transporthttp.NewHttpServer(gateServerAddr, nil, baseLogger)
	httpModules := []transporthttp.Registrar{
		accountModule,
	}
	for _, m := range httpModules {
		m.HttpRegister(httpServer.Group())
	}

	wsServer := ws.NewServer(wsRouter, baseLogger)
	httpServer.Engine().Any("/ws", gin.WrapH(wsServer))
	httpServer.Engine().Any("/ws/*any", gin.WrapH(wsServer))

	gateGRPCHost := serverConfig.Host
	if gateGRPCHost == "" || gateGRPCHost == "0.0.0.0" {
		gateGRPCHost = "127.0.0.1"
	}
	gateGRPCPort := serverConfig.GRPCPort
	if gateGRPCPort <= 0 {
		gateGRPCPort = serverConfig.Port + 10000
	}
	gateGRPCAddr := fmt.Sprintf("%s:%d", gateGRPCHost, gateGRPCPort)
	grpcLis, err := net.Listen("tcp", gateGRPCAddr)
	if err != nil {
		logs.Fatal("listen gate grpc failed", zap.Error(err))
	}
	defer func() {
		_ = grpcLis.Close()
	}()
	grpcServer := googlegrpc.NewServer(
		googlegrpc.ChainUnaryInterceptor(grpc.UnaryServerTraceInterceptor()),
		googlegrpc.ChainStreamInterceptor(grpc.StreamServerTraceInterceptor()),
	)
	gatepb.RegisterGatePushServiceServer(grpcServer, gategrpc.NewPushServer(sessMgr))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Start(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			errCh <- fmt.Errorf("gate server start failed: %w", err)
			return
		}
	}()
	go func() {
		logs.Info("gate grpc server started", zap.String("addr", gateGRPCAddr))
		if err := grpcServer.Serve(grpcLis); err != nil {
			errCh <- fmt.Errorf("gate grpc serve failed: %w", err)
			return
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcStopCh := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopCh)
	}()
	select {
	case <-grpcStopCh:
	case <-time.After(10 * time.Second):
		grpcServer.Stop()
	}
	_ = httpServer.Shutdown(shutdownCtx)
}
