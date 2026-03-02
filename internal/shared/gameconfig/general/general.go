package general

import (
	"ThreeKingdoms/internal/shared/config"
	"math/rand"
	"path/filepath"
	"runtime"
)

type general struct {
	Title            string          `json:"title"`
	GList            []generalDetail `json:"list"`
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
	Name         string `json:"name"`
	CfgId        int    `json:"cfgId"`
	Force        int    `json:"force"`    //武力
	Strategy     int    `json:"strategy"` //策略
	Defense      int    `json:"defense"`  //防御
	Speed        int    `json:"speed"`    //速度
	Destroy      int    `json:"destroy"`  //破坏力
	ForceGrow    int    `json:"force_grow"`
	StrategyGrow int    `json:"strategy_grow"`
	DefenseGrow  int    `json:"defense_grow"`
	SpeedGrow    int    `json:"speed_grow"`
	DestroyGrow  int    `json:"destroy_grow"`
	Cost         int8   `json:"cost"`
	Probability  int    `json:"probability"`
	Star         int8   `json:"star"`
	Arms         []int  `json:"arms"`
	Camp         int8   `json:"camp"`
}

type generalBasic struct {
	Title  string   `json:"title"`
	Levels []gLevel `json:"levels"`
}

type gLevel struct {
	Level    int8 `json:"level"`
	Exp      int  `json:"exp"`
	Soldiers int  `json:"soldiers"`
}

const (
	GeneralNormal      = 0 //正常
	GeneralComposeStar = 1 //星级合成
	GeneralConvert     = 2 //转换
)

const SkillLimit = 3

var General = &general{}

var GeneralBasic = &generalBasic{}

func Load() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("load basic config failed: runtime.Caller(0) error")
	}
	configDir := filepath.Dir(file)
	configPath := filepath.Join(configDir, "general.json")
	config.Load(configPath, &General)
	General.GMap = make(map[int]generalDetail)
	General.totalProbability = 0
	for _, v := range General.GList {
		General.GMap[v.CfgId] = v
		General.totalProbability += v.Probability
	}

	basicPath := filepath.Join(configDir, "general_basic.json")
	config.Load(basicPath, &GeneralBasic)
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
