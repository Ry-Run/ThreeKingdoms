package actors

import (
	"ThreeKingdoms/internal/alliance/entity"
	"ThreeKingdoms/internal/alliance/service/port"
	"ThreeKingdoms/internal/shared/actor/messages"
	"context"
	"sort"

	"github.com/asynkron/protoactor-go/actor"
)

type AllianceID = entity.AllianceID

const defaultAllianceID = AllianceID(1)

type summaryEntry struct {
	summary messages.Alliance
	version uint64
}

type ManagerActor struct {
	repo           port.AllianceRepository
	worldID        int
	allianceActors map[AllianceID]*actor.PID
	summaries      map[AllianceID]summaryEntry
	dbLoaded       bool
}

func NewManagerActor(repo port.AllianceRepository, worldID int) *ManagerActor {
	if worldID < 0 {
		worldID = 0
	}
	return &ManagerActor{
		allianceActors: make(map[AllianceID]*actor.PID),
		repo:           repo,
		worldID:        worldID,
		summaries:      make(map[AllianceID]summaryEntry),
	}
}

func (m *ManagerActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *messages.AllianceSummaryUpsert:
		m.applySummary(msg)
		return
	case *messages.HAAllianceList:
		if msg == nil {
			ctx.Respond(&messages.AHAllianceList{List: make([]messages.Alliance, 0)})
			return
		}
		m.handleAllianceList(ctx, msg)
		return
	case messages.AllianceMessage:
		m.forwardAllianceMessage(ctx, msg)
		return
	default:
		return
	}
}

func (m *ManagerActor) forwardAllianceMessage(ctx actor.Context, req messages.AllianceMessage) {
	if req == nil {
		ctx.Respond("nil request")
		return
	}

	allianceID := defaultAllianceID
	if req.AllianceID() > 0 {
		allianceID = AllianceID(req.AllianceID())
	}

	ctx.Forward(m.getOrSpawn(ctx, allianceID))
}

func (m *ManagerActor) handleAllianceList(ctx actor.Context, req *messages.HAAllianceList) {
	if req == nil {
		ctx.Respond(&messages.AHAllianceList{List: make([]messages.Alliance, 0)})
		return
	}
	worldID := req.WorldID()
	if worldID <= 0 || worldID != m.worldID {
		ctx.Respond(&messages.AHAllianceList{List: make([]messages.Alliance, 0)})
		return
	}
	if !m.dbLoaded {
		if err := m.reloadSummariesFromDB(context.Background()); err != nil {
			ctx.Logger().Error("load alliance summaries from db failed", "world_id", worldID, "err", err)
		}
	}
	ctx.Respond(&messages.AHAllianceList{List: m.snapshotSummaries()})
}

func (m *ManagerActor) applySummary(upsert *messages.AllianceSummaryUpsert) {
	if upsert == nil {
		return
	}
	if upsert.WorldId <= 0 || upsert.WorldId != m.worldID {
		return
	}
	allianceID := AllianceID(upsert.Summary.Id)
	if allianceID <= 0 {
		return
	}
	if old, ok := m.summaries[allianceID]; ok && upsert.Version < old.version {
		return
	}
	m.summaries[allianceID] = summaryEntry{
		summary: upsert.Summary,
		version: upsert.Version,
	}
}

func (m *ManagerActor) reloadSummariesFromDB(ctx context.Context) error {
	if m == nil || m.repo == nil {
		return nil
	}
	states, err := m.repo.ListAllianceSummaryByWorld(ctx, entity.WorldID(m.worldID))
	if err != nil {
		return err
	}
	next := make(map[AllianceID]summaryEntry, len(states))
	for _, state := range states {
		if int(state.WorldId) != m.worldID {
			continue
		}
		allianceID := state.Id
		next[allianceID] = summaryEntry{
			summary: stateToSummary(state),
			version: 0,
		}
	}
	m.summaries = next
	m.dbLoaded = true
	return nil
}

func (m *ManagerActor) snapshotSummaries() []messages.Alliance {
	if len(m.summaries) == 0 {
		return make([]messages.Alliance, 0)
	}
	ids := make([]int, 0, len(m.summaries))
	for allianceID := range m.summaries {
		ids = append(ids, int(allianceID))
	}
	sort.Ints(ids)
	out := make([]messages.Alliance, 0, len(ids))
	for _, id := range ids {
		entry, ok := m.summaries[AllianceID(id)]
		if !ok {
			continue
		}
		out = append(out, entry.summary)
	}
	return out
}

func stateToSummary(state entity.AllianceState) messages.Alliance {
	majorIDs := make([]int, 0, len(state.Majors))
	for rid := range state.Majors {
		majorIDs = append(majorIDs, int(rid))
	}
	sort.Ints(majorIDs)
	majorList := make([]*messages.Major, 0, len(majorIDs))
	for _, rid := range majorIDs {
		majorState, ok := state.Majors[entity.PlayerID(rid)]
		if !ok {
			continue
		}
		majorList = append(majorList, &messages.Major{
			Rid:   int32(majorState.Id),
			Name:  majorState.Name,
			Title: toAllianceTitle(majorState.Title),
		})
	}
	return messages.Alliance{
		Id:     int32(state.Id),
		Name:   state.Name,
		Cnt:    int32(len(state.Members)),
		Notice: state.Notice,
		Major:  majorList,
	}
}

func toAllianceTitle(v int8) messages.AllianceTitle {
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

func (m *ManagerActor) getOrSpawn(ctx actor.Context, allianceID AllianceID) *actor.PID {
	if pid, ok := m.allianceActors[allianceID]; ok && pid != nil {
		return pid
	}

	props := actor.PropsFromProducer(func() actor.Actor {
		return NewAllianceActor(allianceID, entity.WorldID(m.worldID), ctx.Self(), m.repo)
	})
	pid := ctx.Spawn(props)
	m.allianceActors[allianceID] = pid
	return pid
}
