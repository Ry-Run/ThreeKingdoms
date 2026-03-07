package actors

import (
	"ThreeKingdoms/internal/shared/gameconfig/general"
	"ThreeKingdoms/internal/shared/gameconfig/skill"
	"ThreeKingdoms/internal/world/entity"
	"math/rand"
)

type BattleContext struct {
	Attacker            *entity.ArmyState
	Defender            *entity.ArmyState
	AttackerBattleUnits []*BattleUnit
	DefenderBattleUnits []*BattleUnit
}

// 战斗单元，一个武将就是一个单元
type BattleUnit struct {
	General  *entity.GeneralState
	Soldiers int //兵力
	Force    int //武力
	Strategy int //策略
	Defense  int //防御
	Speed    int //速度
	Destroy  int //破坏
	Arms     int //兵种
	Position int //位置

	skills []*TriggeredSkill
}

type Hit struct {
	AId          int               //本回合发起攻击的武将id
	DId          int               //本回合防御方的武将id
	ALoss        int               //本回合攻击方损失的兵力
	DLoss        int               //本回合防守方损失的兵力
	ABeforeSkill []*TriggeredSkill //攻击方攻击前技能
	AAfterSkill  []*TriggeredSkill //攻击方攻击后技能
	BAfterSkill  []*TriggeredSkill //防守方被攻击后触发技能
}

type Round struct {
	Battle []Hit
}

type TriggeredSkill struct {
	Cfg      skill.Conf
	Id       int
	Lv       int   //技能等级
	Duration int   //剩余轮数
	IsEnemy  bool  // 是不是攻击敌人
	FromId   int   //发起的id
	ToId     []int //作用目标id
	IEffect  []int //技能包括的效果
	EValue   []int //效果值
	ERound   []int //效果持续回合数
	Kill     []int //技能杀死数量
}

// 基础属性 + 当前技能修正
type realBattleAttr struct {
	force    int //武力
	strategy int //策略
	defense  int //防御
	speed    int //速度
	destroy  int //破坏
}

func NewTriggeredSkill(cfg skill.Conf, s entity.GSkillState, a *BattleUnit, our []*BattleUnit, enemy []*BattleUnit) *TriggeredSkill {
	l := cfg.Levels[s.Lv-1]

	ts := &TriggeredSkill{
		Cfg:      cfg,
		Id:       s.Id,
		Lv:       s.Lv,
		Duration: cfg.Duration,
		FromId:   a.General.Id,
		IEffect:  cfg.IncludeEffect,
		EValue:   l.EffectValue,
		ERound:   l.EffectRound,
	}

	switch skill.TargetType(cfg.Target) {
	case skill.MySelf:
		applySkillTargets(a, ts, []*BattleUnit{a}, false)
	case skill.OurSingle:
		targets := randNArmyPosAttribute(our, 1)
		applySkillTargets(a, ts, targets, false)
	case skill.OurMostTwo:
		targets := randNArmyPosAttribute(our, 2)
		applySkillTargets(a, ts, targets, false)
	case skill.OurMostThree:
		targets := randNArmyPosAttribute(our, 3)
		applySkillTargets(a, ts, targets, false)
	case skill.OurAll:
		targets := randNArmyPosAttribute(our, len(our))
		applySkillTargets(a, ts, targets, false)
	case skill.EnemySingle:
		targets := randNArmyPosAttribute(enemy, 1)
		applySkillTargets(a, ts, targets, true)
	case skill.EnemyMostTwo:
		targets := randNArmyPosAttribute(enemy, 2)
		applySkillTargets(a, ts, targets, true)
	case skill.EnemyMostThree:
		targets := randNArmyPosAttribute(enemy, 3)
		applySkillTargets(a, ts, targets, true)
	case skill.EnemyAll:
		targets := randNArmyPosAttribute(enemy, len(enemy))
		applySkillTargets(a, ts, targets, true)
	}

	return ts
}

// canTrigger 攻击前或攻击后触发技能
func (a *BattleUnit) triggerSkills(
	our []*BattleUnit,
	enemy []*BattleUnit,
	canTrigger func(cfg *skill.Conf) bool,
) []*TriggeredSkill {
	ret := make([]*TriggeredSkill, 0)

	for _, s := range a.General.Skills {
		if s.Id == 0 {
			continue
		}

		skillCfg, ok := skill.SkillConf.GetCfg(s.CfgId)
		if !ok {
			continue
		}
		if !canTrigger(&skillCfg) {
			continue
		}
		if s.Lv <= 0 || s.Lv > len(skillCfg.Levels) {
			continue
		}

		l := skillCfg.Levels[s.Lv-1]
		if rand.Intn(100) >= 100-l.Probability {
			ret = append(ret, NewTriggeredSkill(skillCfg, s, a, our, enemy))
		}
	}

	return ret
}

func (a *BattleUnit) checkHit() {
	skills := make([]*TriggeredSkill, 0)
	for _, s := range a.skills {
		if s.Duration > 0 {
			//瞬时技能，当前攻击完成后移除
			skills = append(skills, s)
		}
	}
	a.skills = skills
}

func (a *BattleUnit) skillKill(defenders []*BattleUnit, skills []*TriggeredSkill) {
	for _, s := range skills {
		s.Kill = make([]int, len(s.ToId))
		for i, e := range s.IEffect {
			if skill.EffectType(e) == skill.HurtRate {
				v := s.EValue[i]
				for j, to := range s.ToId {
					for _, defender := range defenders {
						if defender == nil || defender.General.Id != to || defender.Soldiers <= 0 {
							continue
						}
						hitTarget := defender
						realA := a.calRealBattleAttr()
						realB := hitTarget.calRealBattleAttr()
						// 提升伤害的百分比
						force := realA.force * v / 100
						attKill := a.kill(hitTarget, force, realB.defense)
						s.Kill[j] += attKill
					}
				}
			}
		}
	}
}

func (a *BattleUnit) kill(hitB *BattleUnit, force int, defense int) int {
	// a兵种对b兵种的伤害率
	ratio := general.GArmsConf.GetHarmRatio(a.Arms, hitB.Arms)

	// 整体战斗力 / (防御力 + 系数)
	damage := float64(force*a.Soldiers) * ratio / float64(defense+100)

	kill := int(damage)

	if kill > hitB.Soldiers {
		kill = hitB.Soldiers
	}

	hitB.Soldiers -= kill
	a.General.Exp += kill * 5

	return kill
}

// 计算真正的战斗属性，包含了技能。因为是回合制，有的技能可能有 buff，所以在释放技能时再计算一次战斗属性
func (a *BattleUnit) calRealBattleAttr() realBattleAttr {
	attr := realBattleAttr{}
	// 开始战斗时的属性
	attr.defense = a.Defense
	attr.force = a.Force
	attr.destroy = a.Destroy
	attr.speed = a.Speed
	attr.strategy = a.Strategy

	for _, s := range a.skills {
		lvData := s.Cfg.Levels[s.Lv-1]
		effects := s.Cfg.IncludeEffect

		for i, effect := range effects {
			v := lvData.EffectValue[i]
			switch skill.EffectType(effect) {
			case skill.HurtRate:
				break
			case skill.Force:
				attr.force += v
				break
			case skill.Defense:
				attr.defense += v
				break
			case skill.Strategy:
				attr.strategy += v
				break
			case skill.Speed:
				attr.speed += v
				break
			case skill.Destroy:
				attr.destroy += v
				break
			}
		}
	}
	return attr
}

func (a *BattleUnit) checkNextRound() {
	skills := make([]*TriggeredSkill, 0)
	for _, s := range a.skills {
		s.Duration -= 1
		if s.Duration > 0 {
			//持续技能，当前回合结束后持续到期移除
			skills = append(skills, s)
		}
	}
	a.skills = skills
}

// 随机 n 个目标位置
func randNArmyPosAttribute(a []*BattleUnit, n int) []*BattleUnit {
	armies := make([]*BattleUnit, 0, n)

	// 收集可用目标
	valid := make([]int, 0, len(a))
	for i, v := range a {
		if v != nil && v.Soldiers > 0 {
			valid = append(valid, i)
		}
	}

	if len(valid) == 0 {
		return armies
	}

	// 最多取 n 个
	n = min(n, len(valid))

	// 随机打乱
	rand.Shuffle(len(valid), func(i, j int) {
		valid[i], valid[j] = valid[j], valid[i]
	})

	// 取前 n 个
	for i := 0; i < n; i++ {
		idx := valid[i]
		armies = append(armies, a[idx])
	}

	return armies
}

func applySkillTargets(a *BattleUnit, ts *TriggeredSkill, targets []*BattleUnit, isEnemy bool) {
	// 目标不为敌人 todo 这个可以删除
	ts.IsEnemy = isEnemy

	// 技能施法目标
	for _, target := range targets {
		ts.ToId = append(ts.ToId, target.General.Id)
	}

	// 应用上触发的技能
	a.skills = append(a.skills, ts)
}
