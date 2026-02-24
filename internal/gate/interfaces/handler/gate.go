package handler

import (
	"ThreeKingdoms/internal/gate/app"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"ThreeKingdoms/internal/shared/session"
)

type Gate struct {
	Session     session.Manager
	GateService *app.GateService
}

func NewGate(s session.Manager, accountServiceClient app.AccountServiceClient, playerServiceClient playerpb.PlayerServiceClient) *Gate {
	gate := Gate{
		Session: s,
	}
	gate.GateService = app.NewGateService(accountServiceClient, playerServiceClient)
	return &gate
}
