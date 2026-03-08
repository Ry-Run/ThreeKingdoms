package actors

import (
	sharedactor "ThreeKingdoms/internal/shared/actor"
	"ThreeKingdoms/internal/shared/actor/messages"
	"ThreeKingdoms/internal/shared/gameconfig/basic"
	"ThreeKingdoms/internal/shared/gameconfig/facility"
	"ThreeKingdoms/internal/shared/gameconfig/general"
	"ThreeKingdoms/internal/shared/gameconfig/map"
	"ThreeKingdoms/internal/shared/gameconfig/skill"
	playerpb "ThreeKingdoms/internal/shared/gen/player"
	"ThreeKingdoms/internal/shared/logs"
	"ThreeKingdoms/internal/shared/utils"
	"ThreeKingdoms/internal/world/entity"
	"math/rand"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

type WorldService struct{}

var WS = &WorldService{}

type PlayerID = entity.PlayerID
type CityID = entity.CityID
type ArmyID = entity.ArmyID
type AllianceID = entity.AllianceID
type CityState = entity.CityState
type messageSender interface {
	Send(pid *actor.PID, message interface{})
}

func (s *WorldService) CreateCity(e *entity.WorldEntity, request *messages.HWCreateCity) *messages.WHCreateCity {
	if request == nil {
		return &messages.WHCreateCity{}
	}
	PlayerId := entity.PlayerID(request.PlayerId)
	cities, ok := e.GetCityByPlayer(PlayerId)

	if ok && cities != nil {
		return &messages.WHCreateCity{}
	}
	id, _ := utils.NextSnowflakeID()
	cityID := CityID(id)

	var city CityState
	var x, y int
	for {
		x = rand.Intn(_map.MapWidth)
		y = rand.Intn(_map.MapHeight)

		if !CanBuildCity(e, x, y) {
			continue
		}

		city = CityState{
			CityId: cityID,
			Pos: entity.PosState{
				X: x,
				Y: y,
			},
			Name:       request.NickName,
			CurDurable: basic.BasicConf.City.Durable,
			IsMain:     true,
			Level:      1,
		}
		break
	}
	cityMap := make(map[CityID]CityState)
	cityMap[cityID] = city
	e.PutCityByPlayer(PlayerId, cityMap)
	// 占据这个格子
	pos := _map.ToPosition(x, y)
	e.UpdateWorldMap(pos, func(v *entity.CellEntity) {
		v.SetMaxDurable(basic.BasicConf.City.Durable)
		v.SetCurDurable(basic.BasicConf.City.Durable)
		v.SetCellType(_map.MapPlayerCity)
		v.Occupancy().SetKind(int8(_map.MapPlayerCity))
		v.Occupancy().SetRefId(request.PlayerId)
		v.Occupancy().SetOwner(request.PlayerId)
		v.Occupancy().SetRoleNick(request.NickName)
		v.Occupancy().SetAllianceId(request.PlayerId)
		v.Occupancy().SetAllianceName(request.AllianceName)
		// garrison & parentId
	})
	return &messages.WHCreateCity{CityId: int(id), X: x, Y: y}
}

func (s *WorldService) SyncCityFacility(world *entity.WorldEntity, req *messages.HWSyncCityFacility) *messages.WHSyncCityFacility {
	resp := &messages.WHSyncCityFacility{OK: false}
	if world == nil || req == nil || req.PlayerId <= 0 || req.CityId <= 0 {
		return resp
	}

	playerID := PlayerID(req.PlayerId)
	cityID := CityID(req.CityId)
	cities, ok := world.GetCityByPlayer(playerID)
	if !ok {
		return resp
	}
	if _, exists := cities[cityID]; !exists {
		return resp
	}

	facilities := toWorldFacilityStates(req.Facilities)
	world.UpdateCityByPlayer(playerID, func(value map[CityID]*entity.CityEntity) {
		city, ok := value[cityID]
		if !ok || city == nil {
			return
		}
		city.ReplaceFacility(facilities)
	})
	resp.OK = true
	return resp
}

func (s *WorldService) ScanBlock(w *WorldActor, request *messages.HWScanBlock) *messages.WHScanBlock {
	if request == nil {
		return &messages.WHScanBlock{}
	}
	world := w.Entity()
	x, y, Length := request.X, request.Y, request.Length
	if x < 0 || x >= _map.MapWidth || y < 0 || y >= _map.MapHeight {
		return &messages.WHScanBlock{}
	}
	maxX := min(_map.MapWidth-1, x+Length-1)
	maxY := min(_map.MapHeight-1, y+Length-1)

	buildings := make([]messages.Building, 0)
	cities := make([]messages.WorldCity, 0)
	// 驻军 && 行军
	armies := make([]messages.Army, 0)

	for i := x; i <= maxX; i++ {
		for j := y; j <= maxY; j++ {
			pos := _map.ToPosition(i, j)
			cell, ok := world.GetWorldMap(pos)
			if !ok {
				continue
			}

			kind := cell.Occupancy.Kind
			if kind == _map.MapPlayerCity {
				// 返回玩家城市
				cityId := CityID(cell.Occupancy.RefId)
				playerId := PlayerID(cell.Occupancy.Owner)
				cityMap, ok := world.GetCityByPlayer(playerId)
				if !ok {
					continue
				}

				if city, ok := cityMap[cityId]; ok {
					cities = append(cities, ToMessagesCity(city, playerId))
				}
			} else if cell.Occupancy.Garrison.ArmyId != 0 && cell.Occupancy.Owner != 0 {
				// 返回驻军信息
				armyID := ArmyID(cell.Occupancy.Garrison.ArmyId)
				playerId := PlayerID(cell.Occupancy.Owner)
				armyMap, ok := world.GetArmies(playerId)
				if !ok {
					continue
				}

				if army, ok := armyMap[armyID]; ok {
					armies = append(armies, ToMessagesArmy(army))
				}
			} else if cell.Occupancy.Owner != 0 || kind == _map.MapBuildSysFortress || kind == _map.MapBuildSysCity {
				// 返回动态建筑/占领地（含系统战略点与玩家动态占据地块）
				buildings = append(buildings, ToMessagesBuilding(cell))
			}

			// 行军信息
			marches, ok := world.GetCellToMarch(cell.Id)
			if ok && len(marches) > 0 {
				for _, march := range marches {
					armyID := march.ArmyID
					playerId := march.PlayerId
					armyMap, ok := world.GetArmies(playerId)
					if !ok {
						continue
					}

					if army, ok := armyMap[armyID]; ok {
						armies = append(armies, ToMessagesArmy(army))
					}
				}
			}
		}
	}

	// 记录玩家当前的视野位置
	w.PlayerView[PlayerID(request.PlayerId)] = View{
		X: x, Y: y, Length: Length,
	}

	return &messages.WHScanBlock{
		Cities:    cities,
		Armies:    armies,
		Buildings: buildings,
	}
}

func (s *WorldService) Attack(ctx actor.Context, w *WorldActor, req *messages.HWAttack) *messages.WHAttack {
	now := time.Now()
	world := w.Entity()
	playerID := PlayerID(req.PlayerId)

	defenderPos := _map.ToPosition(req.DefenderPos.X, req.DefenderPos.Y)
	defenderCell, b := world.GetWorldMap(defenderPos)
	if !b || defenderCell.Occupancy.Owner == 0 {
		ctx.Logger().Error("request param invalid")
		return nil
	}

	attackerCities, b := world.GetCityByPlayer(playerID)
	if !b {
		ctx.Logger().Error("request param invalid")
		return nil
	}
	var attackerCity *entity.CityState
	for _, city := range attackerCities {
		if city.IsMain {
			attackerCity = &city
		}
	}
	if attackerCity == nil {
		ctx.Logger().Error("attacker city not found")
		return nil
	}

	// 自己的城池 和联盟的城池 都不能攻击
	if !s.CanAttack(PlayerID(req.PlayerID()), PlayerID(defenderCell.Occupancy.Owner), attackerCity.AllianceId, AllianceID(defenderCell.Occupancy.AllianceId)) {
		ctx.Logger().Error("can not attack")
		return nil
	}

	//是否免战 比如刚占领 不能被攻击
	if s.IsWarFree(now.UnixMilli(), defenderCell.OccupyTime.UnixMilli()) {
		ctx.Logger().Error("war free")
		return nil
	}

	// 先把 player 传来的消息模型转为 world 模型，再覆盖攻击态关键字段
	army := toWorldArmyState(req.Army)
	army.FromX = attackerCity.Pos.X
	army.FromY = attackerCity.Pos.Y
	army.ToX = defenderCell.Pos.X
	army.ToY = defenderCell.Pos.Y
	army.Cmd = entity.ArmyCmdAttack
	army.State = entity.ArmyRunning
	army.StartTime = now
	// 实际按照速度来
	army.EndTime = now.Add(time.Second * 10)

	// 加入军队池
	armyID := ArmyID(army.Id)
	armies, b := world.GetArmies(playerID)
	if !b || armies == nil {
		armies = make(map[entity.ArmyID]entity.ArmyState)
	}
	armies[armyID] = army
	world.PutArmies(playerID, armies)

	s.dispatchArmyMarch(world, army)

	return &messages.WHAttack{
		OK:        true,
		StartTime: now,
		EndTime:   army.EndTime,
	}
}

// 返回
func (s *WorldService) Back(ctx actor.Context, w *WorldActor, req *messages.HWBack) *messages.WHBack {
	now := time.Now()
	if req == nil || w == nil || w.Entity() == nil || req.PlayerId <= 0 || req.ArmyId <= 0 {
		if ctx != nil {
			ctx.Logger().Error("back request invalid")
		}
		return nil
	}

	world := w.Entity()
	playerID := PlayerID(req.PlayerId)
	armyID := ArmyID(req.ArmyId)

	army, ok := GetArmy(world, playerID, armyID)
	if !ok {
		if ctx != nil {
			ctx.Logger().Error("army not found")
		}
		return nil
	}
	if army.State != entity.ArmyRunning {
		if ctx != nil {
			ctx.Logger().Error("army is not marching")
		}
		return nil
	}

	marches, ok := world.GetMarches(playerID)
	if !ok {
		if ctx != nil {
			ctx.Logger().Error("player marches not found")
		}
		return nil
	}
	currentMarch, ok := marches[armyID]
	if !ok {
		if ctx != nil {
			ctx.Logger().Error("army march not found")
		}
		return nil
	}

	// 返程以前，先把旧的行军索引移除，再以当前位置重新派发行军。
	s.removeMarchFromIndex(world, currentMarch)
	homeX, homeY := army.FromX, army.FromY
	currentX, currentY := marchArmyPos(army)
	army.FromX = currentX
	army.FromY = currentY
	army.ToX = homeX
	army.ToY = homeY
	army.Cmd = entity.ArmyCmdBack
	army.State = entity.ArmyRunning
	army.StartTime = now
	army.EndTime = now.Add(time.Second * 10)

	if !s.replaceArmyState(world, army) {
		if ctx != nil {
			ctx.Logger().Error("replace army state failed")
		}
		return nil
	}
	s.dispatchArmyMarch(world, army)

	return &messages.WHBack{
		OK:   true,
		Army: ToMessagesArmy(army),
	}
}

// 检查行军
func (s *WorldService) march(ctx actor.Context, w *WorldActor) {
	now := time.Now()
	world := w.Entity()
	arrived := make([]entity.MarchState, 0)
	world.ForEachMarches(func(k entity.PlayerID, v map[entity.ArmyID]entity.MarchState) {
		del := false
		for id, state := range v {
			if !state.ArriveAt.After(now) {
				arrived = append(arrived, state)
				delete(v, id)
				s.removeMarchFromIndex(world, state)
				del = true
			}
			// cell 里面的pos更新，暂时没有，空间索引没怎么使用
			// 推送位置
			// 计算当前位置
			army, b := GetArmy(world, state.PlayerId, state.ArmyID)
			if !b {
				continue
			}
			x, y := marchArmyPos(army)
			// 这个位置是否需要推送
			msg := buildArmyPushBatch(w, x, y, &army)
			if msg != nil && len(msg.Items) > 0 {
				if worldPID := w.WorldPID(); worldPID != nil {
					ctx.Send(worldPID, msg)
				}
			}
		}
		if del {
			if len(v) == 0 {
				world.DelMarches(k)
			} else {
				world.PutMarches(k, v)
			}
		}
		// cell 里面的pos更新，暂时没有，空间索引没怎么使用
	})
	// 处理到达的行为
	for _, state := range arrived {
		army, b := GetArmy(world, state.PlayerId, state.ArmyID)
		if !b {
			continue
		}
		// 执行行军到达处理方法
		s.handleArrive(ctx, w, world, army, now)
	}
}

func (s *WorldService) handleArrive(ctx actor.Context, w *WorldActor, world *entity.WorldEntity, army entity.ArmyState, now time.Time) {
	switch army.Cmd {
	case entity.ArmyCmdAttack:
		defenderPos := _map.ToPosition(army.ToX, army.ToY)
		defenderCell, b := world.GetWorldMap(defenderPos)
		if !b || defenderCell.Occupancy.Owner == 0 {
			logs.Warn("not found the defender, can not attack")
			return
		}
		// 自己的城池 和联盟的城池 都不能攻击
		if !s.CanAttack(army.PlayerId, PlayerID(defenderCell.Occupancy.Owner), army.AllianceId, AllianceID(defenderCell.Occupancy.AllianceId)) {
			logs.Warn("can not attack")
			return
		}

		//是否免战 比如刚占领 不能被攻击
		if s.IsWarFree(now.UnixMilli(), defenderCell.OccupyTime.UnixMilli()) {
			logs.Warn("war free")
			return
		}
		s.startBattle(ctx, w, world, army, defenderCell)
	case entity.ArmyCmdBack:
		world.UpdateArmies(army.PlayerId, func(v map[entity.ArmyID]*entity.ArmyEntity) {
			armyEntity, ok := v[ArmyID(army.Id)]
			if ok {
				armyEntity.SetCmd(entity.ArmyCmdIdle)
				armyEntity.SetState(entity.ArmyStop)
				armyEntity.SetToX(armyEntity.FromX())
				armyEntity.SetToY(armyEntity.FromY())
			}
		})
		if updated, ok := GetArmy(world, army.PlayerId, ArmyID(army.Id)); ok {
			s.pushArmySync(ctx, w, updated)
		}
	}
}

func GetArmy(world *entity.WorldEntity, playerId PlayerID, armyId ArmyID) (entity.ArmyState, bool) {
	armies, b := world.GetArmies(playerId)
	if !b || armies == nil {
		return entity.ArmyState{}, false
	}
	army, b := armies[armyId]
	if !b {
		return entity.ArmyState{}, false
	}
	return army, true
}

// 返回行军中的军队的位置 线性插值
func marchArmyPos(army entity.ArmyState) (int, int) {
	if army.State != entity.ArmyRunning {
		return 0, 0
	}
	now := time.Now()

	passedTime := now.Sub(army.StartTime)
	totalTime := army.EndTime.Sub(army.StartTime)

	if totalTime <= 0 {
		return 0, 0
	}

	if passedTime <= 0 {
		return army.FromX, army.FromY
	}

	if passedTime >= totalTime {
		return army.ToX, army.ToY
	}

	progress := float64(passedTime) / float64(totalTime)

	x := int(float64(army.FromX) + (progress * float64(army.ToX-army.FromX)))
	y := int(float64(army.FromY) + progress*float64(army.ToY-army.FromY))

	return x, y
}

// 检查位置的行军是否需要推送
func buildArmyPushBatch(w *WorldActor, x, y int, army *entity.ArmyState) *messages.WorldPushBatch {
	if w == nil || army == nil {
		return nil
	}
	maxMapX := _map.MapWidth - 1
	maxMapY := _map.MapHeight - 1
	msg := make([]messages.WorldPushItem, 0)
	for id, view := range w.PlayerView {
		viewMaxX := min(maxMapX, view.X+view.Length-1)
		viewMaxY := min(maxMapY, view.Y+view.Length-1)

		if x >= view.X && x <= viewMaxX &&
			y >= view.Y && y <= viewMaxY {
			item := messages.WorldPushItem{
				PlayerID: int64(id),
				Army:     toPlayerPBArmy(*army),
			}
			msg = append(msg, item)
		}
	}
	return &messages.WorldPushBatch{
		WorldBaseMessage: messages.WorldBaseMessage{
			WorldId: int(*w.worldID),
		},
		MsgType: messages.ArmyPush,
		Items:   msg,
	}
}

func toPlayerPBArmy(army entity.ArmyState) *playerpb.Army {
	msg := ToMessagesArmy(army)
	pbGenerals := make([]int32, 0, len(msg.Generals))
	for _, general := range msg.Generals {
		if general == nil {
			pbGenerals = append(pbGenerals, 0)
			continue
		}
		pbGenerals = append(pbGenerals, int32(general.Id))
	}
	pbSoldiers := make([]int32, 0, len(msg.Soldiers))
	for _, soldier := range msg.Soldiers {
		pbSoldiers = append(pbSoldiers, int32(soldier))
	}
	pbConTimes := make([]int64, 0, len(msg.ConTimes))
	for _, endTime := range msg.ConTimes {
		pbConTimes = append(pbConTimes, endTime)
	}
	pbConCounts := make([]int32, 0, len(msg.ConCounts))
	for _, count := range msg.ConCounts {
		pbConCounts = append(pbConCounts, int32(count))
	}

	return &playerpb.Army{
		Id:       int32(msg.Id),
		CityId:   int32(msg.CityId),
		UnionId:  int32(msg.AllianceId),
		Order:    int32(msg.Order),
		Generals: pbGenerals,
		Soldiers: pbSoldiers,
		ConTimes: pbConTimes,
		ConCnts:  pbConCounts,
		Cmd:      int32(msg.Cmd),
		State:    int32(msg.State),
		FromX:    int32(msg.FromPos.X),
		FromY:    int32(msg.FromPos.Y),
		ToX:      int32(msg.ToPos.X),
		ToY:      int32(msg.ToPos.Y),
		Start:    msg.Start,
		End:      msg.End,
	}
}

func toWorldArmyState(army messages.Army) entity.ArmyState {
	generals := make([]entity.GeneralState, 0, len(army.Generals))
	for _, value := range army.Generals {
		if value == nil {
			continue
		}
		generals = append(generals, toWorldGeneralState(value))
	}
	soldiers := make([]int, 0, len(army.Soldiers))
	for _, value := range army.Soldiers {
		soldiers = append(soldiers, value)
	}
	conTimes := make([]int64, 0, len(army.ConTimes))
	for _, value := range army.ConTimes {
		conTimes = append(conTimes, value)
	}
	conCounts := make([]int, 0, len(army.ConCounts))
	for _, value := range army.ConCounts {
		conCounts = append(conCounts, value)
	}

	state := entity.ArmyState{
		Id:                army.Id,
		CityId:            CityID(army.CityId),
		PlayerId:          PlayerID(army.PlayerId),
		AllianceId:        AllianceID(army.AllianceId),
		Order:             army.Order,
		Cmd:               army.Cmd,
		State:             army.State,
		FromX:             army.FromPos.X,
		FromY:             army.FromPos.Y,
		ToX:               army.ToPos.X,
		ToY:               army.ToPos.Y,
		Generals:          generals,
		Soldiers:          soldiers,
		ConscriptEndTimes: conTimes,
		ConscriptCounts:   conCounts,
	}
	state.StartTime = millisToTime(army.Start)
	state.EndTime = millisToTime(army.End)
	return state
}

func toWorldFacilityStates(in []messages.Facility) []entity.FacilityState {
	if len(in) == 0 {
		return nil
	}
	out := make([]entity.FacilityState, 0, len(in))
	for _, value := range in {
		out = append(out, entity.FacilityState{
			Name:         value.Name,
			PrivateLevel: value.PrivateLevel,
			FType:        value.FType,
			UpTime:       value.UpTime,
		})
	}
	return out
}

func toWorldGeneralState(g *messages.General) entity.GeneralState {
	if g == nil {
		return entity.GeneralState{}
	}
	return entity.GeneralState{
		Id:             g.Id,
		CfgId:          g.CfgId,
		Power:          g.Power,
		Level:          g.Level,
		Exp:            g.Exp,
		Order:          g.Order,
		CityId:         g.CityId,
		CreatedAt:      g.CreatedAt,
		CurArms:        g.CurArms,
		HasPrPoint:     g.HasPrPoint,
		UsePrPoint:     g.UsePrPoint,
		AttackDistance: g.AttackDistance,
		ForceAdded:     g.ForceAdded,
		StrategyAdded:  g.StrategyAdded,
		DefenseAdded:   g.DefenseAdded,
		SpeedAdded:     g.SpeedAdded,
		DestroyAdded:   g.DestroyAdded,
		StarLv:         g.StarLv,
		Star:           g.Star,
		ParentId:       g.ParentId,
		Skills:         toWorldGSkillStates(g.Skills),
		State:          g.State,
	}
}

func toWorldGSkillStates(skills []messages.GSkill) []entity.GSkillState {
	if len(skills) == 0 {
		return nil
	}
	result := make([]entity.GSkillState, 0, len(skills))
	for _, skill := range skills {
		result = append(result, entity.GSkillState{
			Id:    skill.Id,
			CfgId: skill.CfgId,
			Lv:    skill.Lv,
		})
	}
	return result
}

func (s *WorldService) IsWarFree(now, occupyMills int64) bool {
	// 盟友、或者在保护期内，不可以被攻击
	if now-occupyMills < basic.BasicConf.Build.WarFree {
		return true
	}
	return false
}

func (s *WorldService) CanAttack(attackerId, defenderId PlayerID, attackerAlliance, defenderAlliance AllianceID) bool {
	// 盟友的城池不能攻击
	if attackerAlliance != 0 && attackerAlliance == defenderAlliance {
		return false
	}

	// 自己的城池不能攻击
	if attackerId == defenderId {
		return false
	}

	return true
}

// 和驻防军队进行战斗，需要玩家主动设置驻防军队
func (s *WorldService) startBattle(ctx actor.Context, w *WorldActor, world *entity.WorldEntity, attacker entity.ArmyState, defender entity.CellState) {
	// 略过打建筑，目前主流的slg游戏没有这种玩法
	defenderArmy := s.defenderArmies(world, defender)
	if hasArmyState(defenderArmy) {
		// 有驻防军时走完整战斗结算
		battleContext := initBattleContext(world, attacker, defenderArmy)
		report := s.battle(world, defender, battleContext)
		s.pushWarReport(ctx, w, report, attacker.PlayerId, PlayerID(defender.Occupancy.Owner))
		s.pushBattleResult(ctx, w, *battleContext.Attacker)
		s.pushBattleResult(ctx, w, *battleContext.Defender)
		return
	}

	// 没有驻防军，直接按破坏力扣减耐久并生成战报。
	begAttackArmy := cloneArmyState(attacker)
	destroy := s.Destroy(attacker)
	DurableChange(world, defender, -destroy)
	now := time.Now()
	attacker.FromX = defender.Pos.X
	attacker.FromY = defender.Pos.Y
	attacker.ToX = begAttackArmy.FromX
	attacker.ToY = begAttackArmy.FromY
	attacker.Cmd = entity.ArmyCmdBack
	attacker.State = entity.ArmyRunning
	attacker.StartTime = now
	attacker.EndTime = now.Add(time.Second * 10)
	s.replaceArmyState(world, attacker)
	s.dispatchArmyMarch(world, attacker)
	report := s.createWarReport(begAttackArmy, attacker, defenderArmy, defenderArmy, defender, messages.WIN, destroy, 0, nil)
	s.pushWarReport(ctx, w, report, attacker.PlayerId, PlayerID(defender.Occupancy.Owner))
	s.pushBattleResult(ctx, w, attacker)
}

// 初始化战斗数据  军队和武将属性、兵种、加成等
func initBattleContext(w *entity.WorldEntity, attacker entity.ArmyState, defender entity.ArmyState) *BattleContext {
	attackerBattleUnits := make([]*BattleUnit, 0)
	defenderBattleUnits := make([]*BattleUnit, 0)

	ctx := &BattleContext{
		Attacker:            &attacker,
		Defender:            &defender,
		AttackerBattleUnits: attackerBattleUnits,
		DefenderBattleUnits: defenderBattleUnits,
	}

	//城内设施加成
	attackerAdds := []int{0, 0, 0, 0}
	if attacker.CityId > 0 {
		attackerAdds = GetAdditions(
			w,
			attacker.PlayerId,
			attacker.CityId,
			facility.TypeForce,
			facility.TypeDefense,
			facility.TypeSpeed,
			facility.TypeStrategy)
	}

	defenderAdds := []int{0, 0, 0, 0}
	if defender.CityId > 0 {
		defenderAdds = GetAdditions(
			w,
			defender.PlayerId,
			defender.CityId,
			facility.TypeForce,
			facility.TypeDefense,
			facility.TypeSpeed,
			facility.TypeStrategy)
	}

	for i, g := range attacker.Generals {
		if g.Id == 0 {
			attackerBattleUnits = append(attackerBattleUnits, nil)
		} else {
			a := &BattleUnit{
				General:  &g,
				Soldiers: attacker.Soldiers[i],
				Force:    g.ForceAdded + attackerAdds[0],
				Defense:  g.DefenseAdded + attackerAdds[1],
				Speed:    g.SpeedAdded + attackerAdds[2],
				Strategy: g.StrategyAdded + attackerAdds[3],
				Destroy:  g.DestroyAdded,
				Arms:     g.CurArms,
				Position: i,
			}
			attackerBattleUnits = append(attackerBattleUnits, a)
		}
	}

	for i, g := range defender.Generals {
		if g.Id == 0 {
			defenderBattleUnits = append(defenderBattleUnits, nil)
		} else {
			a := &BattleUnit{
				General:  &g,
				Soldiers: defender.Soldiers[i],
				Force:    g.ForceAdded + defenderAdds[0],
				Defense:  g.DefenseAdded + defenderAdds[1],
				Speed:    g.SpeedAdded + defenderAdds[2],
				Strategy: g.StrategyAdded + defenderAdds[3],
				Destroy:  g.DestroyAdded,
				Arms:     g.CurArms,
				Position: i,
			}
			defenderBattleUnits = append(defenderBattleUnits, a)
		}
	}
	return ctx
}

func (s *WorldService) battle(world *entity.WorldEntity, defender entity.CellState, ctx *BattleContext) messages.WarReport {
	begAttackArmy := cloneArmyState(*ctx.Attacker)
	begDefenseArmy := cloneArmyState(*ctx.Defender)

	isEnd := false
	rounds := make([]*Round, 0)
	for i := 0; i < basic.MaxRound && !isEnd; i++ {
		r, end := round(ctx)
		rounds = append(rounds, r)
		isEnd = end
	}

	attackerBattleUnits, defenderBattleUnits := ctx.AttackerBattleUnits, ctx.DefenderBattleUnits
	for i := 0; i < basic.ArmyGCnt; i++ {
		// 更新军队数据
		if attackerBattleUnits[i] != nil {
			ctx.Attacker.Soldiers[i] = attackerBattleUnits[i].Soldiers
		}
		if defenderBattleUnits[i] != nil {
			ctx.Defender.Soldiers[i] = defenderBattleUnits[i].Soldiers
		}
	}

	// Position == 0 是军队的大本营
	result := 2
	if attackerBattleUnits[0].Soldiers == 0 {
		result = 0
	} else if defenderBattleUnits[0] != nil && defenderBattleUnits[0].Soldiers != 0 {
		result = 1
	} else {
		result = 2
	}

	//武将战斗后
	for i := range ctx.Attacker.Generals {
		if ctx.Attacker.Generals[i].Id != 0 {
			// 更新将领等级
			ctx.Attacker.Generals[i].Level = general.GeneralBasic.ExpToLevel(ctx.Attacker.Generals[i].Exp)
		}
	}

	for i := range ctx.Defender.Generals {
		if ctx.Defender.Generals[i].Id != 0 {
			// 更新将领等级
			ctx.Defender.Generals[i].Level = general.GeneralBasic.ExpToLevel(ctx.Defender.Generals[i].Exp)
		}
	}

	now := time.Now()
	ctx.Attacker.FromX = defender.Pos.X
	ctx.Attacker.FromY = defender.Pos.Y
	ctx.Attacker.ToX = begAttackArmy.FromX
	ctx.Attacker.ToY = begAttackArmy.FromY
	ctx.Attacker.Cmd = entity.ArmyCmdBack
	ctx.Attacker.State = entity.ArmyRunning
	ctx.Attacker.StartTime = now
	ctx.Attacker.EndTime = now.Add(time.Second * 10)

	s.replaceArmyState(world, *ctx.Attacker)
	s.replaceArmyState(world, *ctx.Defender)
	s.dispatchArmyMarch(world, *ctx.Attacker)

	return s.createWarReport(
		begAttackArmy,
		*ctx.Attacker,
		begDefenseArmy,
		*ctx.Defender,
		defender,
		messages.BattleResult(result),
		0,
		0,
		rounds,
	)
}

func round(data *BattleContext) (*Round, bool) {
	curRound := &Round{}
	attacker := data.AttackerBattleUnits
	defender := data.DefenderBattleUnits

	//随机先手
	n := rand.Intn(10)
	if n%2 == 0 {
		attacker = data.DefenderBattleUnits
		defender = data.AttackerBattleUnits
	}

	for i, hitA := range attacker {
		////////攻击方begin//////////
		if hitA == nil || hitA.Soldiers == 0 {
			continue
		}
		hitB := defender[i]
		if hitB == nil {
			return nil, true
		}
		if hit(hitA, hitB, attacker, defender, curRound) {
			return curRound, true
		}
		////////攻击方end//////////

		////////防守方begin//////////
		if hitB.Soldiers == 0 || hitA.Soldiers == 0 {
			continue
		}
		if hit(hitB, hitA, defender, attacker, curRound) {
			return curRound, true
		}
		////////防守方end//////////
	}
	//清理过期的技能功能效果
	for _, attack := range attacker {
		attack.checkNextRound()
	}

	for _, defense := range defender {
		defense.checkNextRound()
	}

	return curRound, false
}

// A 攻击 B，但是 A 可能有群体治疗或者群体伤害
func hit(hitA *BattleUnit, hitB *BattleUnit, attackers []*BattleUnit, defenders []*BattleUnit, curRound *Round) bool {
	//释放技能
	h := Hit{}
	// 获取攻击前技能
	h.ABeforeSkill = hitA.triggerSkills(attackers, defenders, func(cfg *skill.Conf) bool {
		return cfg.IsHitBefore()
	})

	//释放伤害技能
	hitA.skillKill(defenders, h.ABeforeSkill)

	//普通攻击
	if hitB.Soldiers > 0 {
		realA := hitA.calRealBattleAttr()
		realB := hitB.calRealBattleAttr()
		attKill := hitA.kill(hitB, realA.force, realB.defense)
		h.AId = hitA.General.Id
		h.DId = hitB.General.Id
		h.DLoss = attKill
	}

	//清理瞬时技能
	hitA.checkHit()
	hitB.checkHit()

	if hitB.Position == 0 && hitB.Soldiers == 0 {
		//大营干死了，直接结束
		curRound.Battle = append(curRound.Battle, h)
		return true
	}

	//被动触发技能
	h.AAfterSkill = hitA.triggerSkills(attackers, defenders, func(cfg *skill.Conf) bool {
		return cfg.IsHitAfter()
	})
	hitA.skillKill(defenders, h.AAfterSkill)

	h.BAfterSkill = hitB.triggerSkills(attackers, defenders, func(cfg *skill.Conf) bool {
		return cfg.IsHitAfter()
	})
	hitB.skillKill(attackers, h.BAfterSkill)

	curRound.Battle = append(curRound.Battle, h)
	return false
}

func GetAdditions(w *entity.WorldEntity, playerId PlayerID, id CityID, ft ...int8) []int {
	adds := make([]int, len(ft))
	playerCities, b := w.GetCityByPlayer(playerId)
	if !b || playerCities == nil {
		return adds
	}

	state, ok := playerCities[id]
	if !ok {
		return adds
	}

	for _, v := range state.Facility {
		if v.PrivateLevel <= 0 {
			continue
		}

		f, ok := facility.FacilityConf.GetFacility(v.FType)
		if !ok {
			continue
		}

		level, ok := f.LevelMap[v.PrivateLevel]
		if !ok {
			continue
		}

		for i, fType := range f.Additions {
			for j, t := range ft {
				if fType == t {
					adds[j] += level.Values[i]
				}
			}
		}
	}

	return adds
}

func (s *WorldService) createWarReport(
	begAttacker entity.ArmyState,
	endAttacker entity.ArmyState,
	begDefender entity.ArmyState,
	endDefender entity.ArmyState,
	defender entity.CellState,
	result messages.BattleResult,
	destroyDurable int,
	occupy int,
	rounds []*Round,
) messages.WarReport {
	reportId, _ := utils.NextSnowflakeID()

	begAttackArmy := ToMessagesArmy(begAttacker)
	endAttackArmy := ToMessagesArmy(endAttacker)
	begAttackGeneral := s.Generals(begAttacker.Generals)
	endAttackGeneral := s.Generals(endAttacker.Generals)

	var (
		begDefenseArmy    *messages.Army
		endDefenseArmy    *messages.Army
		begDefenseGeneral []*messages.General
		endDefenseGeneral []*messages.General
		defenderPlayer    = defender.Occupancy.Owner
		reportCTimeMills  = int(time.Now().UnixMilli())
	)
	if hasArmyState(begDefender) {
		beg := ToMessagesArmy(begDefender)
		end := ToMessagesArmy(endDefender)
		begDefenseArmy = &beg
		endDefenseArmy = &end
		begDefenseGeneral = s.Generals(begDefender.Generals)
		endDefenseGeneral = s.Generals(endDefender.Generals)
	}

	return messages.WarReport{
		Id:                int(reportId),
		Attacker:          int(begAttacker.PlayerId),
		Defender:          defenderPlayer,
		BegAttackArmy:     &begAttackArmy,
		BegDefenseArmy:    begDefenseArmy,
		EndAttackArmy:     &endAttackArmy,
		EndDefenseArmy:    endDefenseArmy,
		BegAttackGeneral:  begAttackGeneral,
		BegDefenseGeneral: begDefenseGeneral,
		EndAttackGeneral:  endAttackGeneral,
		EndDefenseGeneral: endDefenseGeneral,
		Result:            result,
		Rounds:            toMessageRounds(rounds),
		AttackIsRead:      false,
		DefenseIsRead:     false,
		DestroyDurable:    destroyDurable,
		Occupy:            occupy,
		X:                 defender.Pos.X,
		Y:                 defender.Pos.Y,
		CTime:             reportCTimeMills,
	}
}

func hasArmyState(v entity.ArmyState) bool {
	return v.Id != 0 || v.PlayerId != 0 || len(v.Generals) > 0 || len(v.Soldiers) > 0
}

func cloneArmyState(v entity.ArmyState) entity.ArmyState {
	out := v
	out.Generals = append([]entity.GeneralState(nil), v.Generals...)
	out.Soldiers = append([]int(nil), v.Soldiers...)
	out.ConscriptEndTimes = append([]int64(nil), v.ConscriptEndTimes...)
	out.ConscriptCounts = append([]int(nil), v.ConscriptCounts...)
	return out
}

func (s *WorldService) replaceArmyState(world *entity.WorldEntity, army entity.ArmyState) bool {
	if world == nil || !hasArmyState(army) {
		return false
	}
	return world.UpdateArmies(army.PlayerId, func(v map[entity.ArmyID]*entity.ArmyEntity) {
		if v == nil {
			return
		}
		v[entity.ArmyID(army.Id)] = entity.HydrateArmyEntity(army)
	})
}

func (s *WorldService) dispatchArmyMarch(world *entity.WorldEntity, army entity.ArmyState) bool {
	if world == nil || !hasArmyState(army) {
		return false
	}
	march := entity.MarchState{
		PlayerId:   army.PlayerId,
		AllianceId: army.AllianceId,
		ArmyID:     entity.ArmyID(army.Id),
		From: entity.PosState{
			X: army.FromX,
			Y: army.FromY,
		},
		To: entity.PosState{
			X: army.ToX,
			Y: army.ToY,
		},
		StartAt:  army.StartTime,
		ArriveAt: army.EndTime,
	}

	marches, ok := world.GetMarches(army.PlayerId)
	if !ok || marches == nil {
		marches = make(map[entity.ArmyID]entity.MarchState)
	}
	marches[march.ArmyID] = march
	world.PutMarches(army.PlayerId, marches)

	cellID := _map.ToPosition(army.FromX, army.FromY)
	cellMarches, ok := world.GetCellToMarch(cellID)
	if !ok {
		cellMarches = nil
	}
	world.PutCellToMarch(cellID, upsertMarch(cellMarches, march))
	return true
}

func (s *WorldService) removeMarchFromIndex(world *entity.WorldEntity, march entity.MarchState) bool {
	if world == nil {
		return false
	}
	cellID := _map.ToPosition(march.From.X, march.From.Y)
	cellMarches, ok := world.GetCellToMarch(cellID)
	if !ok || len(cellMarches) == 0 {
		return false
	}
	filtered := removeMarch(cellMarches, march)
	if len(filtered) == 0 {
		return world.DelCellToMarch(cellID)
	}
	return world.PutCellToMarch(cellID, filtered)
}

func upsertMarch(items []entity.MarchState, target entity.MarchState) []entity.MarchState {
	filtered := removeMarch(items, target)
	return append(filtered, target)
}

func removeMarch(items []entity.MarchState, target entity.MarchState) []entity.MarchState {
	if len(items) == 0 {
		return nil
	}
	filtered := make([]entity.MarchState, 0, len(items))
	for _, item := range items {
		if item.PlayerId == target.PlayerId && item.ArmyID == target.ArmyID {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func toMessageRounds(rounds []*Round) []*messages.Round {
	if len(rounds) == 0 {
		return nil
	}
	result := make([]*messages.Round, 0, len(rounds))
	for _, round := range rounds {
		if round == nil {
			continue
		}
		result = append(result, &messages.Round{Battle: toMessageHits(round.Battle)})
	}
	return result
}

func toMessageHits(hits []Hit) []messages.Hit {
	if len(hits) == 0 {
		return nil
	}
	result := make([]messages.Hit, 0, len(hits))
	for _, hit := range hits {
		result = append(result, messages.Hit{
			AId:          hit.AId,
			DId:          hit.DId,
			ALoss:        hit.ALoss,
			DLoss:        hit.DLoss,
			ABeforeSkill: toMessageTriggeredSkills(hit.ABeforeSkill),
			AAfterSkill:  toMessageTriggeredSkills(hit.AAfterSkill),
			BAfterSkill:  toMessageTriggeredSkills(hit.BAfterSkill),
		})
	}
	return result
}

func toMessageTriggeredSkills(skills []*TriggeredSkill) []*messages.TriggeredSkill {
	if len(skills) == 0 {
		return nil
	}
	result := make([]*messages.TriggeredSkill, 0, len(skills))
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		result = append(result, &messages.TriggeredSkill{
			Cfg:      skill.Cfg,
			Id:       skill.Id,
			Lv:       skill.Lv,
			Duration: skill.Duration,
			IsEnemy:  skill.IsEnemy,
			FromId:   skill.FromId,
			ToId:     append([]int(nil), skill.ToId...),
			IEffect:  append([]int(nil), skill.IEffect...),
			EValue:   append([]int(nil), skill.EValue...),
			ERound:   append([]int(nil), skill.ERound...),
			Kill:     append([]int(nil), skill.Kill...),
		})
	}
	return result
}

func (s *WorldService) Generals(v []entity.GeneralState) []*messages.General {
	generals := make([]*messages.General, 0, len(v))
	for _, g := range v {
		generals = append(generals, toMessageGeneral(g))
	}
	return generals
}

func (s *WorldService) pushWarReport(ctx actor.Context, w *WorldActor, report messages.WarReport, playerIDs ...PlayerID) {
	if ctx == nil || w == nil {
		return
	}
	playerManagerPID, ok := w.ResolveManagerPID(sharedactor.ManagerPIDPlayer)
	if !ok || playerManagerPID == nil {
		logs.Warn("player manager actor pid is nil, skip war report push")
		return
	}
	worldID := 0
	if wid := w.WorldID(); wid != nil {
		worldID = int(*wid)
	}

	sent := make(map[PlayerID]struct{}, len(playerIDs))
	for _, playerID := range playerIDs {
		if playerID <= 0 {
			continue
		}
		if _, exists := sent[playerID]; exists {
			continue
		}
		ctx.Send(playerManagerPID, &messages.WHWarReport{
			PlayerBaseMessage: messages.PlayerBaseMessage{
				WorldId:  worldID,
				PlayerId: int(playerID),
			},
			WarReport: report,
		})
		sent[playerID] = struct{}{}
	}
}

func (s *WorldService) pushBattleResult(ctx actor.Context, w *WorldActor, army entity.ArmyState) {
	if ctx == nil || w == nil || !hasArmyState(army) || army.PlayerId <= 0 {
		return
	}
	playerManagerPID, ok := w.ResolveManagerPID(sharedactor.ManagerPIDPlayer)
	if !ok || playerManagerPID == nil {
		logs.Warn("player manager actor pid is nil, skip battle result push")
		return
	}
	worldID := 0
	if wid := w.WorldID(); wid != nil {
		worldID = int(*wid)
	}
	resultArmy := ToMessagesArmy(army)
	ctx.Send(playerManagerPID, &messages.WHBattleResult{
		PlayerBaseMessage: messages.PlayerBaseMessage{
			WorldId:  worldID,
			PlayerId: int(army.PlayerId),
		},
		Army: &resultArmy,
	})
}

func (s *WorldService) pushArmySync(sender messageSender, w *WorldActor, army entity.ArmyState) {
	if sender == nil || w == nil || !hasArmyState(army) || army.PlayerId <= 0 {
		return
	}
	playerManagerPID, ok := w.ResolveManagerPID(sharedactor.ManagerPIDPlayer)
	if !ok || playerManagerPID == nil {
		logs.Warn("player manager actor pid is nil, skip army sync push")
		return
	}
	worldID := 0
	if wid := w.WorldID(); wid != nil {
		worldID = int(*wid)
	}
	resultArmy := ToMessagesArmy(army)
	sender.Send(playerManagerPID, &messages.WHArmySync{
		PlayerBaseMessage: messages.PlayerBaseMessage{
			WorldId:  worldID,
			PlayerId: int(army.PlayerId),
		},
		Army: &resultArmy,
	})
}

func (s *WorldService) defenderArmies(world *entity.WorldEntity, defender entity.CellState) entity.ArmyState {
	armyId := defender.Occupancy.Garrison.ArmyId
	garrisons, b := world.GetArmies(PlayerID(defender.Occupancy.Owner))
	if !b || garrisons == nil {
		return entity.ArmyState{}
	}
	if army, ok := garrisons[armyId]; ok {
		return army
	}

	return entity.ArmyState{}
}

func (s *WorldService) Destroy(attacker entity.ArmyState) int {
	//所有武将的破坏力
	destroy := 0
	for _, g := range attacker.Generals {
		if g.Id == 0 {
			continue
		}
		destroy += s.GeneralDestroy(g)
	}
	return destroy
}

func (s *WorldService) GeneralDestroy(g entity.GeneralState) int {
	cfg, ok := general.General.GMap[g.CfgId]
	if ok {
		return cfg.Destroy + cfg.DestroyGrow*int(g.Level) + g.DestroyAdded
	}
	return 0
}

func DurableChange(world *entity.WorldEntity, c entity.CellState, add int) {
	world.UpdateWorldMap(c.Id, func(v *entity.CellEntity) {
		curDurable := v.CurDurable()
		durable := curDurable + add
		if durable < 0 {
			durable = 0
		} else {
			durable = min(durable, v.MaxDurable())
		}

		v.SetCurDurable(durable)
	})
}

func ToMessagesBuilding(cell entity.CellState) messages.Building {
	b := messages.Building{}
	b.PlayerId = cell.Occupancy.Owner
	b.RNick = cell.Occupancy.RoleNick
	b.AllianceId = cell.Occupancy.AllianceId
	b.AllianceName = cell.Occupancy.AllianceName
	b.ParentId = cell.Occupancy.ParentId
	b.Pos = messages.Pos{X: cell.Pos.X, Y: cell.Pos.Y}
	b.Type = cell.Occupancy.Kind
	b.Name = cell.Name

	b.OccupyTime = cell.OccupyTime
	b.GiveUpTime = cell.GiveUpTime
	b.EndTime = cell.EndTime

	//if cell.EndTime.IsZero() == false {
	//	if IsHasTransferAuth(cell) {
	//		if time.Now().Before(cell.EndTime) == false {
	//			if cell.OPLevel == 0 {
	//				cell.ConvertToRes()
	//			} else {
	//				cell.Level = cell.OPLevel
	//				cell.EndTime = time.Time{}
	//				Cfg, ok := static_conf.MapBCConf.BuildConfig(cell.Type, cell.Level)
	//				if ok {
	//					cell.MaxDurable = Cfg.Durable
	//					cell.CurDurable = min(cell.MaxDurable, cell.CurDurable)
	//					cell.Defender = Cfg.Defender
	//				}
	//			}
	//		}
	//	}
	//}

	b.CurDurable = cell.CurDurable
	b.MaxDurable = cell.MaxDurable
	b.Defender = cell.Defender
	b.Level = cell.Level
	b.OPLevel = cell.OpLevel
	return b
}

func ToMessagesCity(city entity.CityState, playerId PlayerID) messages.WorldCity {
	return messages.WorldCity{
		PlayerId:     int(playerId),
		CityId:       int64(city.CityId),
		Name:         city.Name,
		Pos:          messages.Pos{X: city.Pos.X, Y: city.Pos.Y},
		IsMain:       city.IsMain,
		Level:        city.Level,
		CurDurable:   city.CurDurable,
		MaxDurable:   city.MaxDurable,
		OccupyTime:   city.OccupyTime.UnixNano() / 1e6,
		AllianceId:   int(city.AllianceId),
		AllianceName: city.AllianceName,
		ParentId:     city.ParentId,
	}
}

func ToMessagesArmy(army entity.ArmyState) messages.Army {
	generals := make([]*messages.General, 0, len(army.Generals))
	for _, value := range army.Generals {
		generals = append(generals, toMessageGeneral(value))
	}
	var soldiers [3]int
	for i := 0; i < len(soldiers) && i < len(army.Soldiers); i++ {
		soldiers[i] = army.Soldiers[i]
	}
	var conTimes [3]int64
	for i := 0; i < len(conTimes) && i < len(army.ConscriptEndTimes); i++ {
		conTimes[i] = army.ConscriptEndTimes[i]
	}
	var conCounts [3]int
	for i := 0; i < len(conCounts) && i < len(army.ConscriptCounts); i++ {
		conCounts[i] = army.ConscriptCounts[i]
	}

	return messages.Army{
		Id:         army.Id,
		CityId:     int(army.CityId),
		PlayerId:   int(army.PlayerId),
		AllianceId: int(army.AllianceId),
		Order:      army.Order,
		Generals:   generals,
		Soldiers:   soldiers,
		ConTimes:   conTimes,
		ConCounts:  conCounts,
		Cmd:        army.Cmd,
		State:      army.State,
		FromPos:    messages.Pos{X: army.FromX, Y: army.FromY},
		ToPos:      messages.Pos{X: army.ToX, Y: army.ToY},
		Start:      timeToMillis(army.StartTime),
		End:        timeToMillis(army.EndTime),
	}
}

func timeToMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func millisToTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func toMessageGeneral(g entity.GeneralState) *messages.General {
	return &messages.General{
		Id:             g.Id,
		CfgId:          g.CfgId,
		Power:          g.Power,
		Level:          g.Level,
		Exp:            g.Exp,
		Order:          g.Order,
		CityId:         g.CityId,
		CreatedAt:      g.CreatedAt,
		CurArms:        g.CurArms,
		HasPrPoint:     g.HasPrPoint,
		UsePrPoint:     g.UsePrPoint,
		AttackDistance: g.AttackDistance,
		ForceAdded:     g.ForceAdded,
		StrategyAdded:  g.StrategyAdded,
		DefenseAdded:   g.DefenseAdded,
		SpeedAdded:     g.SpeedAdded,
		DestroyAdded:   g.DestroyAdded,
		StarLv:         g.StarLv,
		Star:           g.Star,
		ParentId:       g.ParentId,
		Skills:         toMessageGSkills(g.Skills),
		State:          g.State,
	}
}

func toMessageGSkills(skills []entity.GSkillState) []messages.GSkill {
	if len(skills) == 0 {
		return nil
	}
	result := make([]messages.GSkill, 0, len(skills))
	for _, skill := range skills {
		result = append(result, messages.GSkill{
			Id:    skill.Id,
			CfgId: skill.CfgId,
			Lv:    skill.Lv,
		})
	}
	return result
}

// todo 根据 world 的 map 数据来确定是否可以建立城市
func CanBuildCity(w *entity.WorldEntity, x int, y int) bool {
	confs := _map.MapConf.Confs
	index := _map.ToPosition(x, y)

	_, ok := confs[index]
	// 超出地图范围
	if !ok {
		return false
	}

	//城池 1范围内 不能超过边界
	if x+1 >= _map.MapWidth || y+1 >= _map.MapHeight || y-1 < 0 || x-1 < 0 {
		return false
	}

	// 系统城池 5 格内不能有其他城池（玩家的和系统的）
	for i := x - 5; i <= x+5; i++ {
		for j := y - 5; j <= y+5; j++ {
			pos := _map.ToPosition(i, j)
			cell, ok := w.GetWorldMap(pos)
			if !ok {
				continue
			}

			// 玩家城市、玩家占据、系统城市
			if cell.Occupancy.Owner != 0 || IsSysBuilding(cell.Occupancy.Kind) || cell.Occupancy.Kind == _map.MapPlayerCity {
				return false
			}

		}
	}
	return true
}

func IsSysBuilding(kind int8) bool {
	return kind == _map.MapBuildSysFortress || kind == _map.MapBuildSysCity
}
