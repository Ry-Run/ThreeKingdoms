package actors

import (
	"ThreeKingdoms/internal/player/service/port"
	sharedactor "ThreeKingdoms/internal/shared/actor"
	"ThreeKingdoms/internal/shared/actor/messages"
	commonpb "ThreeKingdoms/internal/shared/gen/common"
	gatepb "ThreeKingdoms/internal/shared/gen/gate"
	playerpb "ThreeKingdoms/internal/shared/gen/player"

	"github.com/asynkron/protoactor-go/actor"
)

type ManagerActor struct {
	repo         port.PlayerRepository
	playerActors map[PlayerID]*actor.PID // player(uid) -> actor.pid
	resolver     sharedactor.ManagerPIDResolver
	pusher       gatepb.GatePushServiceClient
}

func NewManagerActor(repo port.PlayerRepository, resolver sharedactor.ManagerPIDResolver, pusher gatepb.GatePushServiceClient) *ManagerActor {
	return &ManagerActor{
		playerActors: make(map[PlayerID]*actor.PID),
		repo:         repo,
		resolver:     resolver,
		pusher:       pusher,
	}
}

func (m *ManagerActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *playerpb.PlayerRequest:
		if msg == nil {
			ctx.Respond(failResponse("nil request"))
			return
		}
		playerID, ok := toPlayerID(msg.GetPlayerId())
		if !ok {
			ctx.Respond(failResponse("invalid player_id"))
			return
		}
		worldID, ok := toWorldID(msg.GetWorldId())
		if !ok {
			ctx.Respond(failResponse("invalid world_id"))
			return
		}
		ctx.Forward(m.getOrSpawn(ctx, playerID, worldID))
		return
	case messages.PlayerMessage:
		if msg == nil {
			return
		}
		playerID, ok := toPlayerID(int64(msg.PlayerID()))
		if !ok {
			return
		}
		worldID := WorldID(1)
		if withWorld, hasWorldID := msg.(interface{ WorldID() int }); hasWorldID {
			if resolved, ok := toWorldID(int64(withWorld.WorldID())); ok {
				worldID = resolved
			}
		}
		ctx.Forward(m.getOrSpawn(ctx, playerID, worldID))
		return
	default:
		return
	}
}

func (m *ManagerActor) getOrSpawn(ctx actor.Context, playerId PlayerID, worldId WorldID) *actor.PID {
	if pid, ok := m.playerActors[playerId]; ok && pid != nil {
		return pid
	}

	props := actor.PropsFromProducer(func() actor.Actor {
		return NewPlayerActor(playerId, worldId, m.repo, m.resolver, m.pusher)
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
