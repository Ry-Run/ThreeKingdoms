package actors

import (
	"ThreeKingdoms/internal/shared/actor/messages"

	"github.com/asynkron/protoactor-go/actor"
)

type AllianceHandler struct{}

func (h AllianceHandler) HandleHAAllianceInfo(ctx actor.Context, a *AllianceActor, req *messages.HAAllianceInfo) {
	resp := &messages.AHAllianceInfo{}
	if req == nil || !h.preCheck(a, req.WorldID(), req.AllianceID()) {
		ctx.Respond(resp)
		return
	}
	resp.Alliance = a.summaryFromEntity()
	ctx.Respond(resp)
}

func (h AllianceHandler) HandleHAAllianceApplyList(ctx actor.Context, a *AllianceActor, req *messages.HAAllianceApplyList) {
	resp := &messages.AHAllianceApplyList{
		ApplyItem: make([]messages.ApplyItem, 0),
	}
	if req == nil || !h.preCheck(a, req.WorldID(), req.AllianceID()) {
		ctx.Respond(resp)
		return
	}

	items := make([]messages.ApplyItem, 0, a.Entity().LenApplyList())
	for i := 0; i < a.Entity().LenApplyList(); i++ {
		state, ok := a.Entity().AtApplyList(i)
		if !ok {
			continue
		}
		items = append(items, messages.ApplyItem{
			PlayerId: state.PlayerId,
			NickName: state.NickName,
		})
	}
	resp.ApplyItem = items
	ctx.Respond(resp)
}

func (h AllianceHandler) preCheck(a *AllianceActor, worldID, allianceID int) bool {
	if a == nil || a.Entity() == nil || a.AllianceID() == nil {
		return false
	}
	if worldID <= 0 || worldID != int(a.worldID) {
		return false
	}
	if allianceID <= 0 || allianceID != int(*a.AllianceID()) {
		return false
	}
	return true
}

var AH = &AllianceHandler{}
