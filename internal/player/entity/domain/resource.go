package domain

// entity
type Resource struct {
	wood      int   // 木
	iron      int   // 铁
	stone     int   // 石头
	grain     int   // 粮食
	gold      int   // 金币
	decree    int   // 令牌
	lastClaim int64 // 上次领取产出的时间
}

func (r *Resource) IsEnoughGold(cost int) bool {
	return r.gold >= cost
}
