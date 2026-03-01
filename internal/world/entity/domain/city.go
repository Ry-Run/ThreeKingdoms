package domain

import "time"

// entity
type City struct {
	cityId     CityID
	name       string    // 城池名称
	unionId    int       //联盟id
	unionName  string    //联盟名字
	parentId   int       //上级id
	pos        *Pos      // 坐标
	isMain     bool      // 是否是主城
	level      int8      // 等级
	curDurable int       // 当前耐久
	maxDurable int       // 最大耐久
	occupyTime time.Time // 占领时间
}
