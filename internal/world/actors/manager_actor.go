package actors

import (
	sharedactor "ThreeKingdoms/internal/shared/actor"
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/world/entity"
	"ThreeKingdoms/internal/world/service/port"
	"context"

	"github.com/asynkron/protoactor-go/actor"
	"go.uber.org/zap"
)

type WorldID = entity.WorldID

const defaultWorldID = WorldID(1)

type ManagerActor struct {
	repo        port.WorldRepository
	worldActors map[WorldID]*actor.PID
	resolver    sharedactor.ManagerPIDResolver
	pusher      WorldPushBatchPusher
}

func NewManagerActor(repo port.WorldRepository, resolver sharedactor.ManagerPIDResolver, pusher WorldPushBatchPusher) *ManagerActor {
	return &ManagerActor{
		worldActors: make(map[WorldID]*actor.PID),
		repo:        repo,
		resolver:    resolver,
		pusher:      pusher,
	}
}

func (m *ManagerActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *messages.WorldPushBatch:
		m.handleWorldPushBatch(msg)
		return
	case messages.WorldMessage:
		if msg == nil {
			ctx.Respond("nil request")
			return
		}
		ctx.Forward(m.getOrSpawn(ctx, defaultWorldID))
		return
	default:
		return
	}
}

func (m *ManagerActor) handleWorldPushBatch(msg *messages.WorldPushBatch) {
	if msg == nil || len(msg.Items) == 0 || m.pusher == nil {
		return
	}
	if err := m.pusher.PushWorldPushBatch(context.Background(), msg); err != nil {
		logs.Error("push world batch failed", zap.Error(err), zap.Int("items", len(msg.Items)), zap.String("msg_type", string(msg.MsgType)))
	}
}

func (m *ManagerActor) getOrSpawn(ctx actor.Context, worldID WorldID) *actor.PID {
	if pid, ok := m.worldActors[worldID]; ok && pid != nil {
		return pid
	}

	props := actor.PropsFromProducer(func() actor.Actor {
		return NewWorldActor(worldID, m.repo, m.resolver)
	})
	pid := ctx.Spawn(props)
	m.worldActors[worldID] = pid
	return pid
}
