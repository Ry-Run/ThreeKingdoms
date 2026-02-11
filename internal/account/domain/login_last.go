package domain

import "time"

type LoginLast struct {
	Id         int        `gorm:"column:id;primaryKey;autoIncrement;comment:主键ID" json:"id"`
	UId        int        `gorm:"column:uid;uniqueIndex;not null;comment:用户ID" json:"uid"` // 一个用户只有一条最后登录记录
	LoginTime  time.Time  `gorm:"column:login_time;comment:登录时间" json:"login_time"`
	LogoutTime *time.Time `gorm:"column:logout_time;comment:登出时间" json:"logout_time"`
	Ip         string     `gorm:"column:ip;type:varchar(50);comment:IP地址" json:"ip"`
	Session    string     `gorm:"column:session;type:varchar(255);index;comment:会话标识" json:"session"`
	IsLogout   int8       `gorm:"column:is_logout;default:0;comment:是否已登出 0否 1是" json:"is_logout"`
	Hardware   string     `gorm:"column:hardware;type:varchar(255);comment:硬件信息" json:"hardware"`
}

func (LoginLast) TableName() string {
	return "login_last"
}
