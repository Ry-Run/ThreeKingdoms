package domain

import "time"

type Role struct {
	RId        int       `gorm:"rid pk autoincr"`
	UId        int       `gorm:"uid"`
	NickName   string    `gorm:"nick_name" validate:"min=4,max=20,regexp=^[a-zA-Z0-9_]*$"`
	Balance    int       `gorm:"balance"`
	HeadId     int16     `gorm:"headId"`
	Sex        int8      `gorm:"sex"`
	Profile    string    `gorm:"profile"`
	LoginTime  time.Time `gorm:"login_time"`
	LogoutTime time.Time `gorm:"logout_time"`
	CreatedAt  time.Time `gorm:"created_at"`
}

func (r *Role) TableName() string {
	return "role_1"
}
