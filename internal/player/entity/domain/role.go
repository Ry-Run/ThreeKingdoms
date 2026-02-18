package domain

import "time"

// entity
type Role struct {
	rid        uint32    // roleId
	uid        uint32    // 用户UID
	headid     uint32    // 头像Id
	sex        uint8     // 性别，0:女 1男
	nickName   string    // nick_name
	balance    uint32    // 余额
	loginTime  time.Time // 登录时间
	logoutTime time.Time // 登出时间
	profile    string    // 个人简介
	createdAt  time.Time
}
