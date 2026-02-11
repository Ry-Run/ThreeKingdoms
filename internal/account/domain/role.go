package domain

import "time"

type Role struct {
	RId        int       `gorm:"column:rid;primaryKey;autoIncrement"`
	UId        int       `gorm:"column:uid"`
	NickName   string    `gorm:"column:nick_name" validate:"min=4,max=20,regexp=^[a-zA-Z0-9_]*$"`
	Balance    int       `gorm:"column:balance"`
	HeadId     int16     `gorm:"column:headId"`
	Sex        int8      `gorm:"column:sex"`
	Profile    string    `gorm:"column:profile"`
	LoginTime  time.Time `gorm:"column:login_time"`
	LogoutTime time.Time `gorm:"column:logout_time"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (r *Role) TableName() string {
	return "role_1"
}
