package actors

import (
	"ThreeKingdoms/internal/player/app/port"
	"ThreeKingdoms/internal/player/dc"
	"ThreeKingdoms/internal/player/entity"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
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

type PlayerActor struct {
	state      State
	playerID   *PlayerID
	dc         *dc.PlayerDC
	entity     *entity.Player
	worldPID   *actor.PID
	dispatcher *Dispatcher
	flushStop  chan struct{}
}

type flushTick struct{}

func (flushTick) NotInfluenceReceiveTimeout() {}

func NewPlayerActor(playerID PlayerID, repo port.PlayerRepository) *PlayerActor {
	return &PlayerActor{
		state:      None,
		playerID:   &playerID,
		dc:         dc.NewPlayerDC(repo),
		dispatcher: NewDispatcher(),
	}
}

func (p *PlayerActor) Receive(ctx actor.Context) {
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
			ctx.Logger().Error("player dc close failed", "player_id", p.playerID, "err", err)
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
		p.dc.Flush(context.TODO())
		return
	case *playerpb.PlayerRequest:
		if msg == nil {
			ctx.Respond(fail("nil request"))
			return
		}
		// stash
		//if p.state == Init {
		//	if len(p.stash) >= p.stashLimit {
		//		// 这里回 ErrPlayerLoading
		//		return
		//	}
		//	p.stash = append(p.stash, msg)
		//}

		if p.state != Online {
			ctx.Respond(fail("player not online"))
			return
		}

		/**
		需要异步回消息的时候，就要用到 sender := ctx.Sender() 保留本次消息的响应 actor
		handler 要一个 playerActor、req、sender 即可
		*/
		//worldPID := actor.NewPID("world-host:12000", "world")
		//ctx.Send(worldPID, struct{}{})
		p.dispatcher.Dispatch(ctx, p, msg)
	default:
		return
	}
}

func (p *PlayerActor) init(ctx actor.Context) {
	e, err := p.dc.Load(context.TODO(), p.playerID)
	if err != nil {
		p.state = Stopping
		ctx.Stop(ctx.Self())
		return
	}
	p.state = Online
	p.entity = e
	p.startFlushLoop(ctx)

	// 重放 stash
	//stashed := p.stash
	//p.stash = nil
	//
	//for _, _ = range stashed {
	//	//p.dispatcher.Dispatch("", p, "", "")
	//}
}

func (p *PlayerActor) PlayerID() *PlayerID {
	return p.playerID
}

func (p *PlayerActor) WorldPID() *actor.PID {
	return p.worldPID
}

func (p *PlayerActor) Entity() *entity.Player {
	return p.entity
}

func (p *PlayerActor) DC() *dc.PlayerDC {
	return p.dc
}

func (p *PlayerActor) startFlushLoop(ctx actor.Context) {
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

func (p *PlayerActor) stopFlushLoop() {
	if p.flushStop == nil {
		return
	}
	close(p.flushStop)
	p.flushStop = nil
}
