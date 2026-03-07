package messages

import (
	"ThreeKingdoms/internal/shared/gameconfig/skill"
	"time"
)

type Pos struct {
	X int
	Y int
}

type Facility struct {
	Name         string
	PrivateLevel int
	FType        int8
	UpTime       int64
}

type WorldCity struct {
	PlayerId     int
	CityId       int64
	Name         string
	Pos          Pos
	IsMain       bool
	Level        int8
	CurDurable   int
	MaxDurable   int
	OccupyTime   int64 // 毫秒时间戳
	AllianceId   int
	AllianceName string
	ParentId     int
}

type Building struct {
	PlayerId     int
	Id           int // cell 的 index
	Type         int8
	Level        int8
	OPLevel      int8
	Pos          Pos
	Name         string
	Wood         int
	Iron         int
	Stone        int
	Grain        int
	Defender     int
	CurDurable   int
	MaxDurable   int
	OccupyTime   time.Time
	EndTime      time.Time
	GiveUpTime   time.Time
	RNick        string
	AllianceId   int
	AllianceName string
	ParentId     int
}

type GSkill struct {
	Id    int
	CfgId int
	Lv    int
}

type General struct {
	Id             int
	CfgId          int
	Power          int
	Level          int8
	Exp            int
	Order          int8
	CityId         int
	CreatedAt      time.Time
	CurArms        int
	HasPrPoint     int
	UsePrPoint     int
	AttackDistance int
	ForceAdded     int
	StrategyAdded  int
	DefenseAdded   int
	SpeedAdded     int
	DestroyAdded   int
	StarLv         int8
	Star           int8
	ParentId       int
	Skills         []GSkill
	State          int8
}

type Army struct {
	Id         int
	CityId     int
	PlayerId   int
	AllianceId int  //联盟id
	Order      int8 //第几队，1-5队
	Generals   []*General
	Soldiers   [3]int
	ConTimes   [3]int64
	ConCounts  [3]int
	Cmd        int8
	State      int8 //状态:1=行军中(running)，0=停留/驻守(stop)
	FromPos    Pos
	ToPos      Pos
	Start      int64 //出征开始时间（毫秒时间戳）
	End        int64 //出征结束时间（毫秒时间戳）
}

type BattleResult int

const (
	LOSS BattleResult = iota
	TIE
	WIN
)

type WarReport struct {
	Id                int
	Attacker          int // 进攻方 id
	Defender          int // 防守方 id
	BegAttackArmy     *Army
	BegDefenseArmy    *Army
	EndAttackArmy     *Army
	EndDefenseArmy    *Army
	BegAttackGeneral  []*General
	BegDefenseGeneral []*General
	EndAttackGeneral  []*General
	EndDefenseGeneral []*General
	Result            BattleResult // 0失败，1打平，2胜利
	Rounds            []*Round     // 回合
	AttackIsRead      bool
	DefenseIsRead     bool
	DestroyDurable    int
	Occupy            int
	X                 int
	Y                 int
	CTime             int
}

type Round struct {
	Battle []Hit
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
