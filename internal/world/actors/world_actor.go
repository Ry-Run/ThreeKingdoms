package actors

import (
	sharedactor "ThreeKingdoms/internal/shared/actor"
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/shared/gameconfig/building"
	"ThreeKingdoms/internal/shared/gameconfig/map"
	"ThreeKingdoms/internal/world/dc"
	"ThreeKingdoms/internal/world/entity"
	"ThreeKingdoms/internal/world/service/port"
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
	entity     *entity.WorldEntity
	resolver   sharedactor.ManagerPIDResolver
	dispatcher *Dispatcher
	flushStop  chan struct{}
	// 玩家的视野
	PlayerView map[PlayerID]View
}

func NewWorldActor(worldID WorldID, repo port.WorldRepository, resolver sharedactor.ManagerPIDResolver) *WorldActor {
	return &WorldActor{
		state:      None,
		worldID:    &worldID,
		dc:         dc.NewWorldDC(repo),
		resolver:   resolver,
		dispatcher: NewDispatcher(),
	}
}

func (w *WorldActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		w.init(ctx)
		return
	case *actor.Stopping:
		w.stopFlushLoop()
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := w.dc.Close(closeCtx); err != nil {
			ctx.Logger().Error("world dc close failed", "world_id", w.worldID, "err", err)
		}
		w.state = Stopping
		return
	case *actor.Stopped:
		w.stopFlushLoop()
		w.state = Offline
		return
	case *actor.Restarting:
		w.stopFlushLoop()
		w.state = Init
		return
	case *messages.DCTick:
		if w.state != Online {
			return
		}
		if _, err := w.dc.Tick(); err != nil {
			ctx.Logger().Error("world periodic flush failed", "world_id", w.worldID, "err", err)
		}
		return
	case *messages.Tick:
		if w.state != Online {
			return
		}
		// 检查
		WS.march(ctx, w)
		return
	case messages.WorldMessage:
		if msg == nil {
			ctx.Respond("nil request")
			return
		}

		if w.state != Online {
			ctx.Respond("world not online")
			return
		}

		w.dispatcher.Dispatch(ctx, w, msg)
	default:
		return
	}
}

func (w *WorldActor) init(actorCtx actor.Context) {
	if w.state == Init {
		return
	}
	w.state = Init

	e, err := w.dc.Load(context.TODO(), *w.worldID)
	if err != nil {
		w.state = Stopping
		actorCtx.Stop(actorCtx.Self())
		return
	}

	var needFlush bool
	if e.LenWorldMap() == 0 {
		// todo 可以去掉这个字段直接使用 config
		needFlush = e.ReplaceWorldMap(w.buildInitialMap()) || needFlush
	}

	if needFlush {
		_ = w.dc.FlushSync(context.TODO())
	}

	w.state = Online
	w.entity = e
	w.startFlushLoop(actorCtx)
}

func (w *WorldActor) WorldID() *WorldID {
	return w.worldID
}

func (w *WorldActor) Entity() *entity.WorldEntity {
	return w.entity
}

func (w *WorldActor) DC() *dc.WorldDC {
	return w.dc
}

func (w *WorldActor) WorldPID() *actor.PID {
	if w == nil || w.resolver == nil {
		return nil
	}
	pid, ok := w.resolver.ResolveManagerPID(sharedactor.ManagerPIDWorld)
	if !ok {
		return nil
	}
	return pid
}

func (w *WorldActor) ResolveManagerPID(key sharedactor.ManagerPIDKey) (*actor.PID, bool) {
	if w == nil {
		return nil, false
	}
	if w.resolver == nil {
		return nil, false
	}
	return w.resolver.ResolveManagerPID(key)
}

func (w *WorldActor) startFlushLoop(ctx actor.Context) {
	if w.flushStop != nil {
		return
	}
	interval := w.dc.FlushEvery()
	if interval <= 0 {
		return
	}
	w.flushStop = make(chan struct{})
	self := ctx.Self()
	root := ctx.ActorSystem().Root

	go func(stop <-chan struct{}, every time.Duration) {
		dcTicker := time.NewTicker(every)
		ticker := time.NewTicker(1 * time.Second)
		defer dcTicker.Stop()
		for {
			select {
			case <-dcTicker.C:
				root.Send(self, &messages.DCTick{})
			case <-ticker.C:
				root.Send(self, &messages.Tick{})
			case <-stop:
				dcTicker.Stop()
				return
			}
		}
	}(w.flushStop, interval)
}

func (w *WorldActor) stopFlushLoop() {
	if w.flushStop == nil {
		return
	}
	close(w.flushStop)
	w.flushStop = nil
}

func (w *WorldActor) buildInitialMap() map[int]entity.CellState {
	mapConf := _map.MapConf
	cells := make(map[int]entity.CellState)
	for _, v := range mapConf.Confs {
		// 获取此地块的配置
		cfg := building.BuildingConf.GetCfg(v.Type, v.Level)
		if cfg == nil {
			panic("build conf not found")
		}
		cell := entity.CellState{
			Id:         v.Cid,
			Pos:        entity.PosState{X: v.X, Y: v.Y},
			CellType:   v.Type,
			Level:      v.Level,
			Name:       cfg.Name,
			Wood:       cfg.Wood,
			Iron:       cfg.Iron,
			Stone:      cfg.Stone,
			Grain:      cfg.Grain,
			MaxDurable: cfg.Durable,
			CurDurable: cfg.Durable,
			Defender:   cfg.Defender,
		}
		cells[cell.Id] = cell
	}
	return cells
}
