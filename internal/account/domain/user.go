package domain

import "time"

type User struct {
	UId      int       `gorm:"column:uid;primaryKey;autoIncrement;comment:用户ID" json:"uid"`
	Username string    `gorm:"column:username;type:varchar(20);uniqueIndex;not null;comment:用户名" json:"username" validate:"min=4,max=20,regexp=^[a-zA-Z0-9_]*$"`
	Passcode string    `gorm:"column:passcode;type:varchar(255);comment:安全码;" json:"passcode"`
	Passwd   string    `gorm:"column:passwd;type:varchar(255);comment:密码;" json:"passwd"`
	Hardware string    `gorm:"column:hardware;type:varchar(100);comment:硬件指纹" json:"hardware"`
	Status   int       `gorm:"column:status;default:1;comment:状态 1正常 0禁用" json:"status"`
	Ctime    time.Time `gorm:"column:ctime;autoCreateTime;comment:创建时间" json:"ctime"`
	Mtime    time.Time `gorm:"column:mtime;autoUpdateTime;comment:更新时间" json:"mtime"`
	IsOnline bool      `gorm:"is_online" json:"is_online"`
}

func (User) TableName() string {
	return "user_info" // 指定表名
}

func (u User) CheckPassword(pwd string, encrypt func(plaintext, passcode string) string) bool {
	if pwd == "" {
		return false
	}

	s := encrypt(pwd, u.Passcode)
	if s != u.Passwd {
		return false
	}
	return true
}
