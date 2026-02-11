package handler

import (
	"ThreeKingdoms/internal/gate/app"
	"ThreeKingdoms/internal/shared/session"
)

type Gate struct {
	Session     session.Manager
	GateService *app.GateService
}

func NewGate(s session.Manager, accountServiceClient app.AccountServiceClient) *Gate {
	gate := Gate{
		Session: s,
	}
	gate.GateService = app.NewGateService(accountServiceClient)
	return &gate
}
