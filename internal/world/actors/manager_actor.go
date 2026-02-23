package actors

import (
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/world/entity"
	"ThreeKingdoms/internal/world/service/port"

	"github.com/asynkron/protoactor-go/actor"
)

type WorldID = entity.WorldID

const defaultWorldID = WorldID(1)

type ManagerActor struct {
	repo        port.WorldRepository
	worldActors map[WorldID]*actor.PID
}

func NewManagerActor(repo port.WorldRepository) *ManagerActor {
	return &ManagerActor{
		worldActors: make(map[WorldID]*actor.PID),
		repo:        repo,
	}
}

func (m *ManagerActor) Receive(ctx actor.Context) {
	req, ok := ctx.Message().(messages.WorldMessage)
	if !ok {
		return
	}
	if req == nil {
		ctx.Respond("nil request")
		return
	}

	ctx.Forward(m.getOrSpawn(ctx, defaultWorldID))
}

func (m *ManagerActor) getOrSpawn(ctx actor.Context, worldID WorldID) *actor.PID {
	if pid, ok := m.worldActors[worldID]; ok && pid != nil {
		return pid
	}

	props := actor.PropsFromProducer(func() actor.Actor {
		return NewWorldActor(worldID, m.repo)
	})
	pid := ctx.Spawn(props)
	m.worldActors[worldID] = pid
	return pid
}
