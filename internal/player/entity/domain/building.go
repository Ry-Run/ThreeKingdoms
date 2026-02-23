package domain

import "time"

const (
	BuildingSysFortress = 50 //系统要塞
	BuildingSysCity     = 51 //系统城市
	BuildingFortress    = 56 //玩家要塞
)

// entity
type Building struct {
	id           int
	buildingType int8   // 建筑类型
	level        int8   // 建筑等级
	oPLevel      int8   //建筑操作等级
	x            int    // x坐标
	y            int    // y坐标
	name         string //名称
	wood         int
	iron         int
	stone        int
	grain        int
	defender     int
	curDurable   int       // 当前耐久
	maxDurable   int       // 最大耐久
	occupyTime   time.Time // 占领时间
	endTime      time.Time // 建造、升级、拆除结束时间
	giveUpTime   int64     // 放弃时间
}
