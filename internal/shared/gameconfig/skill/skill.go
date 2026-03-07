package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type TriggerType int

const (
	positive  TriggerType = iota + 1 //主动
	passive                          //被动
	addAttack                        //追击
	command                          //指挥
)

type TargetType int

const (
	MySelf         TargetType = iota + 1 //自己
	OurSingle                            //我军单体
	OurMostTwo                           //我军1-2个目标
	OurMostThree                         //我军1-3个目标
	OurAll                               //我军全体
	EnemySingle                          //敌军单体
	EnemyMostTwo                         //敌军1-2个目标
	EnemyMostThree                       //我军1-3个目标
	EnemyAll                             //敌军全体
)

type EffectType int

const (
	HurtRate EffectType = iota + 1 //伤害率
	Force
	Defense
	Strategy
	Speed
	Destroy
)

type skill struct {
	skills       []Conf
	skillConfMap map[int]Conf
	outline      outline
}

type trigger struct {
	Type int    `json:"type"`
	Des  string `json:"des"`
}

type triggerType struct {
	Des  string    `json:"des"`
	List []trigger `json:"list"`
}

type effect struct {
	Type   int    `json:"type"`
	Des    string `json:"des"`
	IsRate bool   `json:"isRate"`
}

type effectType struct {
	Des  string   `json:"des"`
	List []effect `json:"list"`
}

type target struct {
	Type int    `json:"type"`
	Des  string `json:"des"`
}

type targetType struct {
	Des  string   `json:"des"`
	List []target `json:"list"`
}

type outline struct {
	TriggerType triggerType `json:"trigger_type"` //触发类型
	EffectType  effectType  `json:"effect_type"`  //效果类型
	TargetType  targetType  `json:"target_type"`  //目标类型
}

type level struct {
	Probability int   `json:"probability"`  //发动概率
	EffectValue []int `json:"effect_value"` //效果值
	EffectRound []int `json:"effect_round"` //效果持续回合数
}

type Conf struct {
	CfgId         int     `json:"cfgId"`
	Name          string  `json:"name"`
	Trigger       int     `json:"trigger"` //发起类型
	Target        int     `json:"target"`  //目标类型
	Des           string  `json:"des"`
	Limit         int     `json:"limit"`          //可以被武将装备上限
	Duration      int     `json:"duration"`       //技能持续时间，0为瞬时
	Arms          []int   `json:"arms"`           //可以装备的兵种
	IncludeEffect []int   `json:"include_effect"` //技能包括的效果
	Levels        []level `json:"levels"`
}

const skillOutlineFile = "skill_outline.json"

var SkillConf = skill{}

func Load() {
	SkillConf.Load()
}

func (s *skill) Load() {
	if s == nil {
		panic("load skill config failed: SkillConf is nil")
	}

	s.skills = make([]Conf, 0)
	s.skillConfMap = make(map[int]Conf)

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("load skill config failed: runtime.Caller(0) error")
	}

	baseDir := filepath.Dir(file)
	outlinePath := filepath.Join(baseDir, skillOutlineFile)
	raw, err := os.ReadFile(outlinePath)
	if err != nil {
		panic(fmt.Errorf("load skill outline failed: read %q: %w", outlinePath, err))
	}
	if err := json.Unmarshal(raw, &s.outline); err != nil {
		panic(fmt.Errorf("load skill outline failed: unmarshal %q: %w", outlinePath, err))
	}
	s.loadSkillFiles(baseDir)
}

func (s *skill) loadSkillFiles(baseDir string) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		panic(fmt.Errorf("load skill config failed: read dir %q: %w", baseDir, err))
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subDir := filepath.Join(baseDir, entry.Name())
		files, err := os.ReadDir(subDir)
		if err != nil {
			panic(fmt.Errorf("load skill config failed: read skill subdir %q: %w", subDir, err))
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
				continue
			}

			path := filepath.Join(subDir, file.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				panic(fmt.Errorf("load skill config failed: read %q: %w", path, err))
			}

			var conf Conf
			if err := json.Unmarshal(raw, &conf); err != nil {
				panic(fmt.Errorf("load skill config failed: unmarshal %q: %w", path, err))
			}
			if conf.CfgId == 0 {
				panic(fmt.Errorf("load skill config failed: invalid cfgId=0 file=%q", path))
			}
			if _, exists := s.skillConfMap[conf.CfgId]; exists {
				panic(fmt.Errorf("load skill config failed: duplicate cfgId=%d file=%q", conf.CfgId, path))
			}

			s.skills = append(s.skills, conf)
			s.skillConfMap[conf.CfgId] = conf
		}
	}

	if len(s.skills) == 0 {
		panic("load skill config failed: no skill detail loaded")
	}
}

func (s *skill) GetCfg(cfgId int) (Conf, bool) {
	cfg, ok := s.skillConfMap[cfgId]
	return cfg, ok
}

func (c *Conf) IsHitBefore() bool {
	return c.Trigger == int(positive) || c.Trigger == int(command)
}

func (c *Conf) IsHitAfter() bool {
	return c.Trigger == int(passive) || c.Trigger == int(addAttack)
}
