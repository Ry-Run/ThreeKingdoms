package building

import (
	"ThreeKingdoms/internal/shared/config"
	"path/filepath"
	"runtime"
)

type cfg struct {
	Type     int8   `json:"type"`
	Name     string `json:"name"`
	Level    int8   `json:"level"`
	Grain    int    `json:"grain"`
	Wood     int    `json:"wood"`
	Iron     int    `json:"iron"`
	Stone    int    `json:"stone"`
	Durable  int    `json:"durable"`
	Defender int    `json:"defender"`
}

type buildConf struct {
	Title  string `json:"title"`
	Cfgs   []cfg  `json:"cfgs"`
	CfgMap map[int8]map[int8]cfg
}

var BuildingConf = buildConf{}

func Load() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("load map build config failed: runtime.Caller(0) error")
	}
	configPath := filepath.Join(filepath.Dir(file), "building.json")
	config.Load(configPath, &BuildingConf)
	cfgMap := make(map[int8]map[int8]cfg)
	for _, c := range BuildingConf.Cfgs {
		if _, ok := cfgMap[c.Type]; !ok {
			cfgMap[c.Type] = make(map[int8]cfg)
		}
		cfgMap[c.Type][c.Level] = c
	}
	BuildingConf.CfgMap = cfgMap
}

func (b *buildConf) GetCfg(cellType, level int8) *cfg {
	if b.CfgMap[cellType] == nil {
		return nil
	}
	cfg := b.CfgMap[cellType][level]
	return &cfg
}
