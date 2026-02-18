package model

import "time"

// model
type Role struct {
	Rid        uint32    `gorm:"column:rid;type:int UNSIGNED;comment:roleId;primaryKey;not null;" json:"rid"`       // roleId
	Uid        uint32    `gorm:"column:uid;type:int UNSIGNED;comment:用户UID;not null;" json:"uid"`                   // 用户UID
	Headid     uint32    `gorm:"column:headId;type:int UNSIGNED;comment:头像Id;not null;default:0;" json:"headId"`    // 头像Id
	Sex        uint8     `gorm:"column:sex;type:tinyint UNSIGNED;comment:性别，0:女 1男;not null;default:0;" json:"sex"` // 性别，0:女 1男
	NickName   string    `gorm:"column:nick_name;type:varchar(100);comment:nick_name;" json:"nick_name"`            // nick_name
	Balance    uint32    `gorm:"column:balance;type:int UNSIGNED;comment:余额;not null;default:0;" json:"balance"`    // 余额
	LoginTime  time.Time `gorm:"column:login_time;type:TIMESTAMP;comment:登录时间;default:NULL;" json:"login_time"`     // 登录时间
	LogoutTime time.Time `gorm:"column:logout_time;type:TIMESTAMP;comment:登出时间;default:NULL;" json:"logout_time"`   // 登出时间
	Profile    string    `gorm:"column:profile;type:varchar(500);comment:个人简介;" json:"profile"`                     // 个人简介
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamp;not null;default:CURRENT_TIMESTAMP;" json:"created_at"`
}

func (r *Role) TableName() string {
	return "role_1"
}
