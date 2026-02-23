package model

import (
	"time"
)

// model
type City struct {
	CityId     int       `gorm:"column:cityId;type:int UNSIGNED;comment:cityId;primaryKey;not null;" json:"cityId"`     // cityId
	Rid        int       `gorm:"column:rid;type:int UNSIGNED;comment:roleId;not null;" json:"rid"`                      // roleId
	X          int       `gorm:"column:x;type:int UNSIGNED;comment:x坐标;not null;" json:"x"`                             // x坐标
	Y          int       `gorm:"column:y;type:int UNSIGNED;comment:y坐标;not null;" json:"y"`                             // y坐标
	Name       string    `gorm:"column:name;type:varchar(100);comment:城池名称;not null;default:城池;" json:"name"`           // 城池名称
	IsMain     int8      `gorm:"column:is_main;type:tinyint UNSIGNED;comment:是否是主城;not null;default:0;" json:"is_main"` // 是否是主城
	CurDurable int       `gorm:"column:cur_durable;type:int UNSIGNED;comment:当前耐久;not null;" json:"cur_durable"`        // 当前耐久
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamp;not null;default:CURRENT_TIMESTAMP;" json:"created_at"`
	OccupyTime time.Time `gorm:"column:occupy_time;type:timestamp;comment:占领时间;default:2013-03-15 14:38:09;" json:"occupy_time"` // 占领时间
}

func (m *City) TableName() string {
	return "map_role_city"
}
