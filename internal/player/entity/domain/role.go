package domain

import "time"

type RoleStatus int

// entity
type Role struct {
	headid     int16     // 头像Id
	sex        int8      // 性别，0:女 1男
	nickName   string    // nick_name
	balance    int       // 余额
	loginTime  time.Time // 登录时间
	logoutTime time.Time // 登出时间
	profile    string    // 个人简介
	createdAt  time.Time
}
