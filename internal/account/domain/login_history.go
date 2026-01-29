package domain

import "time"

const (
	LoginFail    int8 = 0
	LoginSuccess int8 = 1
)

type LoginHistory struct {
	Id       int       `gorm:"column:id;primaryKey;autoIncrement;comment:主键ID" json:"id"`
	UId      int       `gorm:"column:uid;index:idx_uid_time;not null;comment:用户ID" json:"uid"`
	CTime    time.Time `gorm:"column:ctime;autoCreateTime;index:idx_uid_time;comment:登录时间" json:"ctime"`
	Ip       string    `gorm:"column:ip;type:varchar(50);comment:IP地址" json:"ip"`
	State    int8      `gorm:"column:state;default:1;comment:登录状态 1成功 0失败" json:"state"`
	Hardware string    `gorm:"column:hardware;type:varchar(255);comment:硬件信息" json:"hardware"`
}

func (LoginHistory) TableName() string {
	return "login_history"
}
