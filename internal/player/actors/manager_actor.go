package actors

import (
	"ThreeKingdoms/internal/player/service/port"
	commonpb "ThreeKingdoms/internal/shared/gen/common"
	playerpb "ThreeKingdoms/internal/shared/gen/player"

	"github.com/asynkron/protoactor-go/actor"
)

type ManagerActor struct {
	repo         port.PlayerRepository
	playerActors map[PlayerID]*actor.PID // player(uid) -> actor.pid
}

func NewManagerActor(repo port.PlayerRepository) *ManagerActor {
	return &ManagerActor{
		playerActors: make(map[PlayerID]*actor.PID),
		repo:         repo,
	}
}

func (m *ManagerActor) Receive(ctx actor.Context) {
	req, ok := ctx.Message().(*playerpb.PlayerRequest)
	if !ok {
		return
	}
	if req == nil {
		ctx.Respond(failResponse("nil request"))
		return
	}
	playerID, ok := toPlayerID(req.GetPlayerId())
	worldID, ok := toWorldID(req.GetWorldId())
	if !ok {
		ctx.Respond(failResponse("invalid player_id"))
		return
	}

	ctx.Forward(m.getOrSpawn(ctx, playerID, worldID))
}

func (m *ManagerActor) getOrSpawn(ctx actor.Context, playerId PlayerID, worldId WorldID) *actor.PID {
	if pid, ok := m.playerActors[playerId]; ok && pid != nil {
		return pid
	}

	props := actor.PropsFromProducer(func() actor.Actor {
		return NewPlayerActor(playerId, worldId, m.repo)
	})
	// ManagerActor 创建 子 actor
	pid := ctx.Spawn(props)
	m.playerActors[playerId] = pid
	return pid
}

func toPlayerID(raw int64) (PlayerID, bool) {
	const maxInt = int64(^uint(0) >> 1)
	if raw <= 0 {
		return 0, false
	}
	if raw > maxInt {
		return 0, false
	}
	return PlayerID(raw), true
}

func toWorldID(raw int64) (WorldID, bool) {
	const maxInt = int64(^uint(0) >> 1)
	if raw <= 0 {
		return 0, false
	}
	if raw > maxInt {
		return 0, false
	}
	return WorldID(raw), true
}

func failResponse(reason string) *playerpb.PlayerResponse {
	return &playerpb.PlayerResponse{
		Result: &commonpb.BizResult{
			Ok:      false,
			Reason:  reason,
			Message: reason,
		},
	}
}
