package actors

import (
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/world/entity"
	"sort"

	"github.com/asynkron/protoactor-go/actor"
)

type WorldHandler struct{}

var WH = &WorldHandler{}

func (h *WorldHandler) HandleHWCreateCity(ctx actor.Context, w *WorldActor, request *messages.HWCreateCity) {
	if request == nil || request.PlayerId <= 0 {
		ctx.Respond(&messages.WHCreateCity{})
		return
	}

	ctx.Respond(WS.CreateCity(w.Entity(), request))
}

func (h *WorldHandler) HandleHWMyCities(ctx actor.Context, w *WorldActor, request *messages.HWMyCities) {
	out := &messages.WHMyCities{}
	if request == nil || request.PlayerId <= 0 || w == nil || w.Entity() == nil {
		ctx.Respond(out)
		return
	}

	cities, ok := w.Entity().GetCityByPlayer(entity.PlayerID(request.PlayerId))
	if !ok || len(cities) == 0 {
		ctx.Respond(out)
		return
	}

	type kv struct {
		id   entity.CityID
		city entity.CityState
	}
	rows := make([]kv, 0, len(cities))
	for cityID, city := range cities {
		rows = append(rows, kv{id: cityID, city: city})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })

	out.Cities = make([]messages.WorldCity, 0, len(rows))
	for _, row := range rows {
		out.Cities = append(out.Cities, ToMessagesCity(row.city, entity.PlayerID(request.PlayerId)))
	}
	ctx.Respond(out)
}

func (h *WorldHandler) HandleHWScanBlock(ctx actor.Context, w *WorldActor, request *messages.HWScanBlock) {
	ctx.Respond(WS.ScanBlock(w, request))
}

func (h *WorldHandler) HandleHWAttack(ctx actor.Context, w *WorldActor, req *messages.HWAttack) {
	attack := WS.Attack(ctx, w, req)
	if attack == nil {
		attack = &messages.WHAttack{
			OK: false,
		}
	}
	ctx.Respond(attack)
}

func (h *WorldHandler) HandleHWSyncCityFacility(ctx actor.Context, w *WorldActor, req *messages.HWSyncCityFacility) {
	ctx.Respond(WS.SyncCityFacility(w.Entity(), req))
}
