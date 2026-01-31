package main

import (
	"ThreeKingdoms/internal/account/interfaces"
	"ThreeKingdoms/internal/shared/config"
	"ThreeKingdoms/internal/shared/infrastructure/db"
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/shared/session"
	transporthttp "ThreeKingdoms/internal/shared/transport/http"
	"ThreeKingdoms/internal/shared/transport/ws"
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
	if err := logs.Init("login", config.Conf.Log); err != nil {
		panic(err)
	}
	logs.Info("conf", zap.Any("conf", config.Conf))

	httpHost := config.Conf.HTTPServer.Host
	if httpHost == "" {
		httpHost = "0.0.0.0"
	}
	httpServerAddr := fmt.Sprintf("%s:%d", httpHost, config.Conf.HTTPServer.Port)

	wsHost := config.Conf.LoginServer.Host
	if wsHost == "" {
		wsHost = "0.0.0.0"
	}
	wsServerAddr := fmt.Sprintf("%s:%d", wsHost, config.Conf.LoginServer.Port)

	gormDB, err := db.Open(config.Conf.MySQL)
	if err != nil {
		logs.Fatal("open db failed", zap.Error(err))
	}
	sessMgr := session.NewSessMgr()
	r := ws.NewRouter()
	accountModule := interfaces.New(gormDB, logs.Logger(), sessMgr)
	wsModules := []ws.Registrar{
		accountModule,
	}
	for _, m := range wsModules {
		m.WsRegister(r)
	}

	httpServer := transporthttp.NewHttpServer(httpServerAddr, gin.Default())
	httpModules := []transporthttp.Registrar{
		accountModule,
	}
	for _, m := range httpModules {
		m.HttpRegister(httpServer.Group())
	}

	wsServer := ws.NewServer(wsServerAddr, r)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go func() {
		if err := httpServer.Start(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			errCh <- fmt.Errorf("http server start failed: %w", err)
			return
		}
		errCh <- nil
	}()
	go func() {
		if err := wsServer.Start(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			errCh <- fmt.Errorf("ws server start failed: %w", err)
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
	_ = wsServer.Shutdown(shutdownCtx)
}
