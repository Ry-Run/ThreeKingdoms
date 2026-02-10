package main

import (
	"ThreeKingdoms/internal/gate/interfaces"
	"ThreeKingdoms/internal/shared/config"
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/shared/session"
	"ThreeKingdoms/internal/shared/transport/grpc"
	transporthttp "ThreeKingdoms/internal/shared/transport/http"
	"ThreeKingdoms/internal/shared/transport/ws"
	"ThreeKingdoms/modules/kit/logx"
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	config.Load("")
	if err := logs.Init("gate", config.Conf.Log); err != nil {
		panic(err)
	}
	logs.Info("conf", zap.Any("conf", config.Conf))

	serverConfig := config.Conf.GateServer
	gateHost := serverConfig.Host
	if gateHost == "" {
		gateHost = "0.0.0.0"
	}
	gateServerAddr := fmt.Sprintf("%s:%d", gateHost, serverConfig.Port)

	sessMgr := session.NewSessMgr()
	baseLogger := logx.NewZapLogger(logs.Logger())
	wsRouter := ws.NewRouter(baseLogger)

	loginServerHost := config.Conf.LoginServer.Host
	if loginServerHost == "" {
		loginServerHost = "0.0.0.0"
	}
	loginServerAddr := fmt.Sprintf("%s:%d", loginServerHost, config.Conf.LoginServer.Port)

	accountConn, accountClient, err := grpc.DialAccountService(loginServerAddr)
	if err != nil {
		logs.Fatal("dial account service failed", zap.Error(err))
	}
	defer func() {
		_ = accountConn.Close()
	}()

	accountModule := interfaces.New(sessMgr, accountClient)
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Start(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			errCh <- fmt.Errorf("gate server start failed: %w", err)
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
