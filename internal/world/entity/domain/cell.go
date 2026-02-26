package domain

import "time"

// entity
type Cell struct {
	id       int // 数组中的下标
	pos      Pos
	cellType int8
	name     string
	level    int8

	opLevel    int8
	wood       int
	iron       int
	stone      int
	grain      int
	defender   int
	curDurable int
	maxDurable int
	occupyTime time.Time
	endTime    time.Time
	giveUpTime time.Time

	occupancy *Occupancy
}

// entity
type Occupancy struct {
	kind      int8 // 玩家城市, 系统城市, 系统要塞
	refId     int  // 关联对象ID（如城市ID/要塞ID等）
	owner     int  // 所属玩家ID（PlayerID），用于判断玩家动态占据态；不是 rid
	roleNick  string
	unionId   int
	unionName string
	parentId  int       // 上级联盟 ID
	garrison  *Garrison // 持久化驻军信息
}

// entity
type Garrison struct {
	owner  int // CityID / BuildingID / FortressID
	armyId int // 军队 id
}
