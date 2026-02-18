package actors

import (
	worldpb "ThreeKingdoms/internal/shared/gen/world"
	"ThreeKingdoms/internal/world/app/port"
	"ThreeKingdoms/internal/world/dc"
	"ThreeKingdoms/internal/world/entity"
	"context"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

type State int

const (
	None State = iota
	Init
	Online
	Offline
	Stopping
)

type WorldActor struct {
	state      State
	worldID    *WorldID
	dc         *dc.WorldDC
	entity     *entity.World
	dispatcher *Dispatcher
	flushStop  chan struct{}
}

type flushTick struct{}

func (flushTick) NotInfluenceReceiveTimeout() {}

func NewWorldActor(worldID WorldID, repo port.WorldRepository) *WorldActor {
	return &WorldActor{
		state:      None,
		worldID:    &worldID,
		dc:         dc.NewWorldDC(repo),
		dispatcher: NewDispatcher(),
	}
}

func (p *WorldActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		p.state = Init
		p.init(ctx)
		return
	case *actor.Stopping:
		p.stopFlushLoop()
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := p.dc.Close(closeCtx); err != nil {
			ctx.Logger().Error("world dc close failed", "world_id", p.worldID, "err", err)
		}
		p.state = Stopping
		return
	case *actor.Stopped:
		p.stopFlushLoop()
		p.state = Offline
		return
	case *actor.Restarting:
		p.stopFlushLoop()
		p.state = Init
		return
	case flushTick:
		if p.state != Online {
			return
		}
		if err := p.dc.Flush(context.TODO()); err != nil {
			ctx.Logger().Error("world periodic flush failed", "world_id", p.worldID, "err", err)
		}
		return
	case *worldpb.EmptyRequest:
		if msg == nil {
			ctx.Respond(fail("nil request"))
			return
		}

		if p.state != Online {
			ctx.Respond(fail("world not online"))
			return
		}

		p.dispatcher.Dispatch(ctx, p, msg)
	default:
		return
	}
}

func (p *WorldActor) init(ctx actor.Context) {
	e, err := p.dc.Load(context.TODO(), p.worldID)
	if err != nil {
		p.state = Stopping
		ctx.Stop(ctx.Self())
		return
	}
	p.state = Online
	p.entity = e
	p.startFlushLoop(ctx)
}

func (p *WorldActor) WorldID() *WorldID {
	return p.worldID
}

func (p *WorldActor) Entity() *entity.World {
	return p.entity
}

func (p *WorldActor) DC() *dc.WorldDC {
	return p.dc
}

func (p *WorldActor) startFlushLoop(ctx actor.Context) {
	if p.flushStop != nil {
		return
	}
	interval := p.dc.FlushEvery()
	if interval <= 0 {
		return
	}
	p.flushStop = make(chan struct{})
	self := ctx.Self()
	root := ctx.ActorSystem().Root

	go func(stop <-chan struct{}, every time.Duration) {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				root.Send(self, flushTick{})
			case <-stop:
				return
			}
		}
	}(p.flushStop, interval)
}

func (p *WorldActor) stopFlushLoop() {
	if p.flushStop == nil {
		return
	}
	close(p.flushStop)
	p.flushStop = nil
}
