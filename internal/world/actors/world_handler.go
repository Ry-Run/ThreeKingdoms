package actors

import (
	commonpb "ThreeKingdoms/internal/shared/gen/common"
	worldpb "ThreeKingdoms/internal/shared/gen/world"

	"github.com/asynkron/protoactor-go/actor"
)

type WorldHandler struct{}

var WH = &WorldHandler{}

func (h *WorldHandler) HandleNationMapConfigRequest(ctx actor.Context, p *WorldActor, request *worldpb.EmptyRequest) {
	if request == nil {
		ctx.Respond(fail("request parameter error"))
		return
	}
	if p == nil || p.entity == nil {
		ctx.Respond(fail("world entity not ready"))
		return
	}
	ctx.Respond(ok(p.entity.NationMapConfigJSON()))
}

func ok(payload string) *worldpb.JsonReply {
	if payload == "" {
		payload = "{}"
	}
	return &worldpb.JsonReply{
		Result: &commonpb.BizResult{
			Ok: true,
		},
		PayloadJson: payload,
	}
}

func fail(reason string) *worldpb.JsonReply {
	return &worldpb.JsonReply{
		Result: &commonpb.BizResult{
			Ok:      false,
			Reason:  reason,
			Message: reason,
		},
		BizMessage: reason,
	}
}
