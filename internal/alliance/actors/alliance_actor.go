package actors

import (
	"ThreeKingdoms/internal/alliance/dc"
	"ThreeKingdoms/internal/alliance/entity"
	"ThreeKingdoms/internal/alliance/service/port"
	"ThreeKingdoms/internal/shared/actor/messages"
	"context"
	"sort"
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

type AllianceActor struct {
	state                 State
	allianceID            *AllianceID
	worldID               entity.WorldID
	managerPID            *actor.PID
	dc                    *dc.AllianceDC
	entity                *entity.AllianceEntity
	dispatcher            *Dispatcher
	flushStop             chan struct{}
	pendingSummaryVersion uint64
	pendingSummary        messages.Alliance
	hasPendingSummary     bool
}

type flushTick struct{}

func (flushTick) NotInfluenceReceiveTimeout() {}

type persistedSummaryReady struct {
	Version uint64
}

func NewAllianceActor(allianceID AllianceID, worldID entity.WorldID, managerPID *actor.PID, repo port.AllianceRepository) *AllianceActor {
	return &AllianceActor{
		state:      None,
		allianceID: &allianceID,
		worldID:    worldID,
		managerPID: managerPID,
		dc:         dc.NewAllianceDC(repo),
		dispatcher: NewDispatcher(),
	}
}

func (a *AllianceActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.init(ctx)
		return
	case *actor.Stopping:
		a.stopFlushLoop()
		a.hasPendingSummary = false
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := a.dc.Close(closeCtx); err != nil {
			ctx.Logger().Error("alliance dc close failed", "alliance_id", a.allianceID, "err", err)
		}
		a.state = Stopping
		return
	case *actor.Stopped:
		a.stopFlushLoop()
		a.hasPendingSummary = false
		a.state = Offline
		return
	case *actor.Restarting:
		a.stopFlushLoop()
		a.hasPendingSummary = false
		a.state = Init
		return
	case flushTick:
		if a.state != Online {
			return
		}
		summary := a.summaryFromEntity()
		version, err := a.dc.Tick()
		if err != nil {
			ctx.Logger().Error("alliance periodic flush failed", "alliance_id", a.allianceID, "err", err)
			return
		}
		if version > 0 {
			a.pendingSummaryVersion = version
			a.pendingSummary = summary
			a.hasPendingSummary = true
			self := ctx.Self()
			root := ctx.ActorSystem().Root
			go func(ver uint64) {
				if err := a.dc.WaitPersisted(context.Background(), ver); err != nil {
					return
				}
				root.Send(self, &persistedSummaryReady{Version: ver})
			}(version)
		}
		return
	case *persistedSummaryReady:
		if !a.hasPendingSummary || msg.Version != a.pendingSummaryVersion {
			return
		}
		summary := a.pendingSummary
		a.hasPendingSummary = false
		if a.managerPID != nil {
			ctx.Send(a.managerPID, &messages.AllianceSummaryUpsert{
				WorldId: int(a.worldID),
				Version: msg.Version,
				Summary: summary,
			})
		}
		return
	case messages.AllianceMessage:
		if msg == nil {
			ctx.Respond("nil request")
			return
		}
		if a.state != Online {
			ctx.Respond("alliance not online")
			return
		}
		a.dispatcher.Dispatch(ctx, a, msg)
	default:
		return
	}
}

func (a *AllianceActor) init(actorCtx actor.Context) {
	if a.state == Init {
		return
	}
	a.state = Init

	e, err := a.dc.Load(context.TODO(), *a.allianceID)
	if err != nil {
		a.state = Stopping
		actorCtx.Stop(actorCtx.Self())
		return
	}

	var needFlush bool
	state := e.Save()
	if state.Id == 0 {
		needFlush = e.SetId(*a.allianceID) || needFlush
	}
	if state.WorldId == 0 && a.worldID > 0 {
		needFlush = e.SetWorldId(a.worldID) || needFlush
	}
	if state.Majors == nil {
		needFlush = e.ReplaceMajors(make(map[entity.PlayerID]entity.MajorState)) || needFlush
	}
	if state.Members == nil {
		needFlush = e.ReplaceMembers(make(map[entity.PlayerID]entity.MemberState)) || needFlush
	}

	if needFlush {
		_ = a.dc.FlushSync(context.TODO())
	}

	a.state = Online
	a.entity = e
	a.startFlushLoop(actorCtx)
	a.publishBootstrapSummary(actorCtx)
}

func (a *AllianceActor) AllianceID() *AllianceID {
	return a.allianceID
}

func (a *AllianceActor) Entity() *entity.AllianceEntity {
	return a.entity
}

func (a *AllianceActor) DC() *dc.AllianceDC {
	return a.dc
}

func (a *AllianceActor) startFlushLoop(ctx actor.Context) {
	if a.flushStop != nil {
		return
	}
	interval := a.dc.FlushEvery()
	if interval <= 0 {
		return
	}
	a.flushStop = make(chan struct{})
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
	}(a.flushStop, interval)
}

func (a *AllianceActor) stopFlushLoop() {
	if a.flushStop == nil {
		return
	}
	close(a.flushStop)
	a.flushStop = nil
}

func (a *AllianceActor) publishBootstrapSummary(ctx actor.Context) {
	if a == nil || a.managerPID == nil {
		return
	}
	ctx.Send(a.managerPID, &messages.AllianceSummaryUpsert{
		WorldId: int(a.worldID),
		Version: 0,
		Summary: a.summaryFromEntity(),
	})
}

func (a *AllianceActor) summaryFromEntity() messages.Alliance {
	if a == nil || a.entity == nil {
		return messages.Alliance{}
	}
	majors := make([]*messages.Major, 0, a.entity.LenMajors())
	majorIDs := make([]int, 0, a.entity.LenMajors())
	a.entity.ForEachMajors(func(rid entity.PlayerID, _ entity.MajorState) {
		majorIDs = append(majorIDs, int(rid))
	})
	sort.Ints(majorIDs)
	for _, rid := range majorIDs {
		major, ok := a.entity.GetMajors(entity.PlayerID(rid))
		if !ok {
			continue
		}
		majors = append(majors, &messages.Major{
			Rid:   int32(major.Id),
			Name:  major.Name,
			Title: toAllianceSummaryTitle(major.Title),
		})
	}
	return messages.Alliance{
		Id:     int32(a.entity.Id()),
		Name:   a.entity.Name(),
		Cnt:    int32(a.entity.LenMembers()),
		Notice: a.entity.Notice(),
		Major:  majors,
	}
}

func toAllianceSummaryTitle(v int8) messages.AllianceTitle {
	switch messages.AllianceTitle(v) {
	case messages.ALLIANCE_CHAIRMAN:
		return messages.ALLIANCE_CHAIRMAN
	case messages.ALLIANCE_VICE_CHAIRMAN:
		return messages.ALLIANCE_VICE_CHAIRMAN
	case messages.ALLIANCE_COMMON:
		return messages.ALLIANCE_COMMON
	default:
		return messages.ALLIANCE_COMMON
	}
}
