package _map

import (
	"ThreeKingdoms/internal/shared/config"
	"path/filepath"
	"runtime"
)

type MapCell struct {
	MId   int  `json:"mid"`
	X     int  `json:"x"`
	Y     int  `json:"y"`
	Type  int8 `json:"type"`
	Level int8 `json:"level"`
}

type mapResource struct {
	Confs       map[int]MapCell
	SysBuilding map[int]MapCell
}

type mapData struct {
	Width  int     `json:"w"`
	Height int     `json:"h"`
	List   [][]int `json:"list"`
}

const (
	MapBuildSysFortress = 50 //系统要塞
	MapBuildSysCity     = 51 //系统城市
	MapBuildFortress    = 56 //玩家要塞
)

var MapResource = &mapResource{
	Confs:       make(map[int]MapCell),
	SysBuilding: make(map[int]MapCell),
}

var MapWidth = 200
var MapHeight = 200

func ToPosition(x, y int) int {
	return x + MapHeight*y
}

func LoadMap() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("load map config failed: runtime.Caller(0) error")
	}
	configPath := filepath.Join(filepath.Dir(file), "map.json")
	mapData := &mapData{}
	config.Load(configPath, &mapData)
	MapWidth = mapData.Width
	MapHeight = mapData.Height
	for index, v := range mapData.List {
		t := int8(v[0])
		l := int8(v[1])
		nm := MapCell{
			X:     index % MapWidth,
			Y:     index / MapWidth,
			MId:   index,
			Type:  t,
			Level: l,
		}
		MapResource.Confs[index] = nm
		if t == MapBuildSysFortress || t == MapBuildSysCity {
			MapResource.SysBuilding[index] = nm
		}
	}
}
