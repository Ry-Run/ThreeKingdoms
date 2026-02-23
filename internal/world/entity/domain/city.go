package domain

import "time"

// entity
type City struct {
	cityId     CityID
	name       string    // 城池名称
	unionId    int       //联盟id
	unionName  string    //联盟名字
	parentId   int       //上级id
	x          int       // x坐标
	y          int       // y坐标
	isMain     bool      // 是否是主城
	level      int8      // 最大耐久
	curDurable int       // 当前耐久
	maxDurable int       // 最大等级
	occupyTime time.Time // 占领时间
}
