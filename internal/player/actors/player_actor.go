package actors

import (
	"ThreeKingdoms/internal/player/dc"
	"ThreeKingdoms/internal/player/entity"
	"ThreeKingdoms/internal/player/service/port"
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"context"
	"errors"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

type State int

const (
	None State = iota
	Init
	LoadFailed
	Online
	Offline
	Stopping
)

const seqWindowSize = 1024

type PlayerID = entity.PlayerID
type WorldID = entity.WorldID

type PlayerActor struct {
	state    State
	PlayerId *PlayerID
	WorldId  *WorldID
	dc       *dc.PlayerDC

	worldPID   *actor.PID
	dispatcher *Dispatcher
	flushStop  chan struct{}

	seenSeq      map[int64]struct{}
	seenSeqOrder []int64
}

type flushTick struct{}

func (flushTick) NotInfluenceReceiveTimeout() {}

func NewPlayerActor(playerID PlayerID, worldId WorldID, repo port.PlayerRepository) *PlayerActor {
	return &PlayerActor{
		state:      None,
		PlayerId:   &playerID,
		WorldId:    &worldId,
		dc:         dc.NewPlayerDC(repo),
		dispatcher: NewDispatcher(),
		seenSeq:    make(map[int64]struct{}, seqWindowSize),
	}
}

func (p *PlayerActor) Receive(actorCtx actor.Context) {
	switch msg := actorCtx.Message().(type) {
	case *actor.Started:
		if err := p.init(context.TODO(), actorCtx, false); err != nil && !errors.Is(err, entity.ErrPlayerNotFound) {
			actorCtx.Logger().Error("player init failed", "player_id", p.PlayerId, "err", err)
		}
		return
	case *actor.Stopping:
		p.stopFlushLoop()
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := p.dc.Close(closeCtx); err != nil {
			actorCtx.Logger().Error("player dc close failed", "player_id", p.PlayerId, "err", err)
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
		if _, err := p.dc.Tick(); err != nil {
			actorCtx.Logger().Error("player periodic flush failed", "player_id", p.PlayerId, "err", err)
		}
		return
	case *playerpb.PlayerRequest:
		if msg == nil {
			actorCtx.Respond(fail("nil request"))
			return
		}
		if err := p.acceptSeq(msg.GetSeq()); err != nil {
			actorCtx.Respond(fail(err.Error()))
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
			if err := p.init(context.TODO(), actorCtx, true); err != nil {
				return
			}
		}

		if p.state != Online || p.Entity() == nil {
			actorCtx.Respond(fail("player state invalid"))
			return
		}

		p.dispatcher.Dispatch(actorCtx, p, msg)
	default:
		return
	}
}

func (p *PlayerActor) init(ctx context.Context, actorCtx actor.Context, respondOnErr bool) error {
	if p.state == Init {
		// todo 如果能 stash
		if respondOnErr {
			actorCtx.Respond(fail("player loading"))
		}
		return nil
	}

	p.state = Init

	_, err := p.dc.Load(ctx, *p.PlayerId)
	if err != nil {
		p.state = LoadFailed
		if respondOnErr {
			actorCtx.Respond(fail("load player failed"))
		}
		return err
	}

	err = p.initPlayer(ctx, actorCtx)

	if err != nil {
		return err
	}

	p.state = Online
	p.startFlushLoop(actorCtx)

	// 重放 stash
	//stashed := p.stash
	//p.stash = nil
	//
	//for _, _ = range stashed {
	//	//p.dispatcher.Dispatch("", p, "", "")
	//}
	return nil
}

func (p *PlayerActor) startFlushLoop(actorCtx actor.Context) {
	if p.flushStop != nil {
		return
	}
	interval := p.dc.FlushEvery()
	if interval <= 0 {
		return
	}
	p.flushStop = make(chan struct{})
	self := actorCtx.Self()
	root := actorCtx.ActorSystem().Root

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

func (p *PlayerActor) Entity() *entity.PlayerEntity {
	return p.dc.Entity()
}

func (p *PlayerActor) initPlayer(ctx context.Context, actorCtx actor.Context) error {
	if p == nil {
		return errors.New("player is nil")
	}
	player := p.Entity()

	var needFlush bool
	if player.Profile() == nil {
		needFlush = player.SetProfile(p.buildInitialProfile())
	}

	if player.Resource() == nil {
		needFlush = player.SetResource(p.buildInitialResource())
	}

	if player.Attribute() == nil {
		needFlush = player.SetAttribute(p.buildInitialAttribute())
	}

	if needFlush {
		_ = p.dc.FlushSync(context.TODO())
	}

	// todo 可以在第一次打开地图时获取位置
	future := actorCtx.RequestFuture(
		p.worldPID,
		messages.HWCreateCity{
			WorldBaseMessage: messages.WorldBaseMessage{
				PlayerId: int(*p.PlayerId),
				WorldId:  0,
			},
			NickName: player.Profile().NickName(),
		},
		5*time.Second,
	)
	result, err := future.Result()
	if err != nil {
		return err
	}

	if WHPosition, ok := result.(messages.WHCreateCity); ok {
		actorCtx.Logger().Info("position", "x", WHPosition.X, "y", WHPosition.Y)
	} else {
		return entity.ErrCreateCity
	}

	return nil
}

func (p *PlayerActor) acceptSeq(seq int64) error {
	if seq <= 0 {
		return errors.New("invalid seq")
	}
	if _, ok := p.seenSeq[seq]; ok {
		return errors.New("duplicate seq")
	}
	p.seenSeq[seq] = struct{}{}
	p.seenSeqOrder = append(p.seenSeqOrder, seq)

	if len(p.seenSeqOrder) > seqWindowSize {
		evict := p.seenSeqOrder[0]
		p.seenSeqOrder = p.seenSeqOrder[1:]
		delete(p.seenSeq, evict)
	}
	return nil
}

func (p *PlayerActor) buildInitialProfile() entity.RoleState {
	return entity.RoleState{
		Headid:    0,
		Sex:       0,
		NickName:  "momo",
		CreatedAt: time.Now(),
	}
}

func (p *PlayerActor) buildInitialResource() entity.ResourceState {
	config := basic.BasicConf.Role

	return entity.ResourceState{
		Wood:   config.Wood,
		Iron:   config.Iron,
		Stone:  config.Stone,
		Grain:  config.Grain,
		Gold:   config.Gold,
		Decree: config.Decree,
	}
}

func (p *PlayerActor) buildInitialAttribute() entity.RoleAttributeState {
	return entity.RoleAttributeState{
		ParentId: 0,
	}
}
