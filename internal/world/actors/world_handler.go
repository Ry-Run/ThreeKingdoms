package actors

import (
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/world/entity"
	"ThreeKingdoms/internal/world/service"
	"sort"

	"github.com/asynkron/protoactor-go/actor"
)

type WorldHandler struct{}

var WH = &WorldHandler{}

func (h *WorldHandler) HandleHWCreateCity(ctx actor.Context, w *WorldActor, request messages.HWCreateCity) {
	if request.PlayerId <= 0 {
		ctx.Respond(messages.WHCreateCity{})
		return
	}

	cityID := service.WS.CreateCity(w.Entity(), request)
	ctx.Respond(messages.WHCreateCity{CityId: int(cityID)})
}

func (h *WorldHandler) HandleHWWorldMap(ctx actor.Context, w *WorldActor, request messages.HWWorldMap) {
	var worldMap messages.WHWorldMap
	w.Entity().ForEachWorldMap(func(i int, v entity.CellState) {
		cell := messages.WorldCell{
			Type:     v.CellType,
			Name:     v.Name,
			Level:    v.Level,
			Defender: v.Defender,
			Durable:  v.Durable,
			Grain:    v.Grain,
			Iron:     v.Iron,
			Stone:    v.Stone,
			Wood:     v.Wood,
		}
		worldMap.WorldMap = append(worldMap.WorldMap, cell)
	})
	ctx.Respond(worldMap)
}

func (h *WorldHandler) HandleHWMyCities(ctx actor.Context, w *WorldActor, request messages.HWMyCities) {
	var out messages.WHMyCities
	if request.PlayerId <= 0 || w == nil || w.Entity() == nil {
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
		c := row.city
		out.Cities = append(out.Cities, messages.WorldCity{
			CityId:     int64(row.id),
			Name:       c.Name,
			X:          c.X,
			Y:          c.Y,
			IsMain:     c.IsMain,
			Level:      c.Level,
			CurDurable: c.CurDurable,
			MaxDurable: c.MaxDurable,
			OccupyTime: c.OccupyTime.UnixNano() / 1e6,
			UnionId:    c.UnionId,
			UnionName:  c.UnionName,
			ParentId:   c.ParentId,
		})
	}
	ctx.Respond(out)
}
