package general

import (
	"ThreeKingdoms/internal/shared/config"
	"math/rand"
	"path/filepath"
	"runtime"
)

type general struct {
	Title            string          `json:"title" mapstructure:"title"`
	GList            []generalDetail `json:"list" mapstructure:"list"`
	GMap             map[int]generalDetail
	totalProbability int
}

func (g general) Cost(id int) int8 {
	curGeneral, ok := g.GMap[id]
	if !ok {
		return 0
	}
	return curGeneral.Cost
}

type generalDetail struct {
	Name         string `json:"name" mapstructure:"name"`
	CfgId        int    `json:"cfgId" mapstructure:"cfgId"`
	Force        int    `json:"force" mapstructure:"force"`       //武力
	Strategy     int    `json:"strategy" mapstructure:"strategy"` //策略
	Defense      int    `json:"defense" mapstructure:"defense"`   //防御
	Speed        int    `json:"speed" mapstructure:"speed"`       //速度
	Destroy      int    `json:"destroy" mapstructure:"destroy"`   //破坏力
	ForceGrow    int    `json:"force_grow" mapstructure:"force_grow"`
	StrategyGrow int    `json:"strategy_grow" mapstructure:"strategy_grow"`
	DefenseGrow  int    `json:"defense_grow" mapstructure:"defense_grow"`
	SpeedGrow    int    `json:"speed_grow" mapstructure:"speed_grow"`
	DestroyGrow  int    `json:"destroy_grow" mapstructure:"destroy_grow"`
	Cost         int8   `json:"cost" mapstructure:"cost"`
	Probability  int    `json:"probability" mapstructure:"probability"`
	Star         int8   `json:"star" mapstructure:"star"`
	Arms         []int  `json:"arms" mapstructure:"arms"`
	Camp         int8   `json:"camp" mapstructure:"camp"`
}

type generalBasic struct {
	Title  string   `json:"title" mapstructure:"title"`
	Levels []gLevel `json:"levels" mapstructure:"levels"`
}

type gLevel struct {
	Level    int8 `json:"level" mapstructure:"level"`
	Exp      int  `json:"exp" mapstructure:"exp"`
	Soldiers int  `json:"soldiers" mapstructure:"soldiers"`
}

type gArmsCondition struct {
	Level     int `json:"level" mapstructure:"level"`
	StarLevel int `json:"star_lv" mapstructure:"star_lv"`
}

type gArmsCost struct {
	Gold int `json:"gold" mapstructure:"gold"`
}

type gArms struct {
	Id         int            `json:"id" mapstructure:"id"` // 兵种 ID
	Name       string         `json:"name" mapstructure:"name"`
	Condition  gArmsCondition `json:"condition" mapstructure:"condition"`
	ChangeCost gArmsCost      `json:"change_cost" mapstructure:"change_cost"`
	HarmRatio  []int          `json:"harm_ratio" mapstructure:"harm_ratio"` // 该兵种对其他兵种的伤害率
}

type Arms struct {
	Title string        `json:"title" mapstructure:"title"`
	Arms  []gArms       `json:"arms" mapstructure:"arms"`
	AMap  map[int]gArms // 兵种 ID to gArms
}

const (
	GeneralNormal      = 0 //正常
	GeneralComposeStar = 1 //星级合成
	GeneralConvert     = 2 //转换
)

const SkillLimit = 3

var General = &general{}

var GeneralBasic = &generalBasic{}

var GArmsConf = &Arms{}

func Load() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("load basic config failed: runtime.Caller(0) error")
	}
	configDir := filepath.Dir(file)
	configPath := filepath.Join(configDir, "general.json")
	config.Load(configPath, General)
	General.GMap = make(map[int]generalDetail)
	General.totalProbability = 0
	for _, v := range General.GList {
		General.GMap[v.CfgId] = v
		General.totalProbability += v.Probability
	}

	basicPath := filepath.Join(configDir, "general_basic.json")
	config.Load(basicPath, GeneralBasic)

	armsPath := filepath.Join(configDir, "general_arms.json")
	config.Load(armsPath, GArmsConf)
	GArmsConf.AMap = make(map[int]gArms, len(GArmsConf.Arms))
	for _, v := range GArmsConf.Arms {
		GArmsConf.AMap[v.Id] = v
	}
}

// 随机武将
func Rand() int {
	if General == nil || General.GList == nil {
		return 0
	}
	rate := rand.Intn(General.totalProbability)
	current := 0
	for _, v := range General.GList {
		if rate >= current && rate <= current+v.Probability {
			return v.CfgId
		}
		current += v.Probability
	}
	return 0
}

func (g *generalBasic) GetLevel(level int8) *gLevel {
	for i := range g.Levels {
		if g.Levels[i].Level == level {
			return &g.Levels[i]
		}
	}
	return nil
}

func (a *Arms) GetHarmRatio(attId, defId int) float64 {
	attArm, ok1 := a.AMap[attId]
	_, ok2 := a.AMap[defId]
	if ok1 && ok2 && defId-1 >= 0 && defId-1 < len(attArm.HarmRatio) {
		return float64(attArm.HarmRatio[defId-1]) / 100.0
	} else {
		return 1.0
	}
}

func (g *generalBasic) ExpToLevel(exp int) int8 {
	var level int8 = 0

	for _, v := range g.Levels {
		if exp >= v.Exp {
			level = v.Level
		} else {
			break
		}
	}

	return level
}
