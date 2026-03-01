package facility

import (
	"ThreeKingdoms/internal/shared/config"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	TypeDurable        = 1 // 耐久
	TypeCost           = 2
	TypeArmyTeams      = 3 // 队伍数量
	TypeSpeed          = 4 // 速度
	TypeDefense        = 5 // 防御
	TypeStrategy       = 6 // 谋略
	TypeForce          = 7 // 攻击武力
	TypeConscriptTime  = 8 // 征兵时间
	TypeReserveLimit   = 9 // 预备役上限
	TypeUnknow         = 10
	TypeHanAddition    = 11
	TypeQunAddition    = 12
	TypeWeiAddition    = 13
	TypeShuAddition    = 14
	TypeWuAddition     = 15
	TypeDealTaxRate    = 16 // 交易税率
	TypeWood           = 17
	TypeIron           = 18
	TypeGrain          = 19
	TypeStone          = 20
	TypeTax            = 21 // 税收
	TypeExtendTimes    = 22 // 扩建次数
	TypeWarehouseLimit = 23 // 仓库容量
	TypeSoldierLimit   = 24 // 带兵数量
	TypeVanguardLimit  = 25 // 前锋数量
)

const (
	facilityIndexFile    = "Facility.json"
	facilityAdditionFile = "facility_addition.json"
)

type conditions struct {
	Type  int `json:"type"`
	Level int `json:"level"`
}

type Facility struct {
	Title      string       `json:"title"`
	Des        string       `json:"des"`
	Name       string       `json:"name"`
	Type       int8         `json:"type"`
	Additions  []int8       `json:"additions"`
	Conditions []conditions `json:"conditions"`
	Levels     []Level      `json:"levels"`
	LevelMap   map[int]Level
}

type NeedResource struct {
	Decree int `json:"decree"`
	Grain  int `json:"grain"`
	Wood   int `json:"wood"`
	Iron   int `json:"iron"`
	Stone  int `json:"stone"`
	Gold   int `json:"gold"`
}

type Level struct {
	Level  int          `json:"level"`
	Values []int        `json:"values"`
	Need   NeedResource `json:"need"`
	Time   int          `json:"time"` // 升级需要的时间
}

type Cfg struct {
	Name string `json:"name"`
	Type int8   `json:"type"`
}

type facilityConf struct {
	Title      string `json:"title"`
	List       []Cfg  `json:"list"`
	Facilities map[int8]*Facility
}

var FacilityConf = &facilityConf{}

// Load 保持与 basic/map 配置模块一致的调用方式。
func Load() {
	FacilityConf.Load()
}

func (f *facilityConf) Load() {
	if f == nil {
		panic("load Facility config failed: FacilityConf is nil")
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("load Facility config failed: runtime.Caller(0) error")
	}

	baseDir := filepath.Dir(file)
	indexPath := filepath.Join(baseDir, facilityIndexFile)
	config.Load(indexPath, f)
	f.loadFacilityFiles(baseDir)
}

func (f *facilityConf) loadFacilityFiles(baseDir string) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		panic(fmt.Errorf("load Facility config failed: read dir %q: %w", baseDir, err))
	}

	f.Facilities = make(map[int8]*Facility, len(f.List))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		if name == facilityIndexFile || name == facilityAdditionFile {
			continue
		}

		path := filepath.Join(baseDir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Errorf("load Facility detail failed: read %q: %w", path, err))
		}

		var item Facility
		if err := json.Unmarshal(raw, &item); err != nil {
			panic(fmt.Errorf("load Facility detail failed: unmarshal %q: %w", path, err))
		}

		// 跳过非设施明细类 JSON（例如分类/附加配置表）。
		if item.Name == "" || len(item.Levels) == 0 {
			continue
		}
		item.LevelMap = make(map[int]Level, len(item.Levels))
		for _, lv := range item.Levels {
			item.LevelMap[lv.Level] = lv
		}

		if _, exists := f.Facilities[item.Type]; exists {
			panic(fmt.Errorf("load Facility detail failed: duplicate Facility type=%d file=%q", item.Type, path))
		}

		copyItem := item
		f.Facilities[item.Type] = &copyItem
	}

	// 主索引中列出的设施都应该有对应明细。
	for _, c := range f.List {
		if _, ok := f.Facilities[c.Type]; !ok {
			panic(fmt.Errorf("load Facility detail failed: missing Facility detail for type=%d name=%q", c.Type, c.Name))
		}
	}
}

func (f *facilityConf) GetFacility(t int8) (*Facility, bool) {
	if f == nil || f.Facilities == nil {
		return nil, false
	}
	v, ok := f.Facilities[t]
	return v, ok
}

type FacilityYield struct {
	Wood  int
	Grain int
	Iron  int
	Stone int
	Gold  int
}

func (f *Facility) GetFacilityYield(level int) FacilityYield {
	var y FacilityYield
	if f == nil || f.LevelMap == nil {
		return y
	}
	l, ok := f.LevelMap[level]
	if !ok {
		return y
	}
	additions := f.Additions
	values := l.Values
	for i, aType := range additions {
		if i >= len(values) {
			break
		}
		if aType == TypeWood {
			y.Wood += values[i]
		} else if aType == TypeGrain {
			y.Grain += values[i]
		} else if aType == TypeIron {
			y.Iron += values[i]
		} else if aType == TypeStone {
			y.Stone += values[i]
		} else if aType == TypeTax {
			y.Gold += values[i]
		}
	}
	return y
}

func (f *facilityConf) MaxLevel(t int8) int {
	facility, b := f.GetFacility(t)

	if !b {
		return 0
	}
	return len(facility.Levels)
}

func (f *facilityConf) Cost(t int8, l int) NeedResource {
	facility, b := f.GetFacility(t)

	if !b {
		return NeedResource{}
	}

	level, ok := facility.LevelMap[l]

	if !ok {
		return NeedResource{}
	}

	return level.Need
}
