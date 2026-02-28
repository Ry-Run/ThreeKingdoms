package actors

import (
	"ThreeKingdoms/internal/shared/actor/messages"

	"github.com/asynkron/protoactor-go/actor"
)

type AllianceHandler struct{}

func (h AllianceHandler) HandleHAAllianceInfo(ctx actor.Context, a *AllianceActor, req *messages.HAAllianceInfo) {
	resp := &messages.AHAllianceInfo{}
	if req == nil || a == nil || a.Entity() == nil || a.AllianceID() == nil {
		ctx.Respond(resp)
		return
	}
	if req.WorldID() <= 0 || req.WorldID() != int(a.worldID) {
		ctx.Respond(resp)
		return
	}
	if req.AllianceID() <= 0 || req.AllianceID() != int(*a.AllianceID()) {
		ctx.Respond(resp)
		return
	}
	resp.Alliance = a.summaryFromEntity()
	ctx.Respond(resp)
}

var AH = &AllianceHandler{}
