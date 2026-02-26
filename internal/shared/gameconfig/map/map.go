package _map

import (
	"ThreeKingdoms/internal/shared/config"
	"path/filepath"
	"runtime"
)

type MapCell struct {
	Cid   int  `json:"cid"` // cell 的 index
	X     int  `json:"x"`
	Y     int  `json:"y"`
	Type  int8 `json:"type"`
	Level int8 `json:"level"`
}

type mapConf struct {
	Confs       map[int]MapCell
	SysBuilding map[int]MapCell
}

type mapData struct {
	Width  int     `json:"w"`
	Height int     `json:"h"`
	List   [][]int `json:"list"`
}

const (
	MapBuildEmpty       = 0  //空地
	MapBuildSysFortress = 50 //系统要塞
	MapBuildSysCity     = 51 //系统城市
	MapWOOD             = 52
	MapIRON             = 53
	MapSTONE            = 54
	MapGRAIN            = 55
	MapBuildFortress    = 56 //玩家要塞
	MapPlayerCity       = 70 // 玩家城市
)

var MapConf = &mapConf{
	Confs:       make(map[int]MapCell),
	SysBuilding: make(map[int]MapCell),
}

var MapWidth = 200
var MapHeight = 200

func ToPosition(x, y int) int {
	return x + MapHeight*y
}

func Load() {
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
			Cid:   index,
			Type:  t,
			Level: l,
		}
		MapConf.Confs[index] = nm
		if t == MapBuildSysFortress || t == MapBuildSysCity {
			MapConf.SysBuilding[index] = nm
		}
	}
}

func (m *MapCell) IsSysCity() bool {
	return m.Type == MapBuildSysCity
}
func (m *MapCell) IsSysFortress() bool {
	return m.Type == MapBuildSysFortress
}
func (m *MapCell) IsRoleFortress() bool {
	return m.Type == MapBuildFortress
}
