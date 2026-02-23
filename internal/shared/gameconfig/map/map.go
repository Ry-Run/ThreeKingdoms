package _map

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

type mapBuildConf struct {
	Title  string `json:"title"`
	Cfg    []cfg  `json:"cfg"`
	cfgMap map[int8][]cfg
}

var MapConf = mapBuildConf{}

func Load() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("load map config failed: runtime.Caller(0) error")
	}
	configPath := filepath.Join(filepath.Dir(file), "map_build.json")
	config.Load(configPath, &MapConf)
}
