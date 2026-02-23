package global

// todo 应该通过配置获取
var MapWith = 200
var MapHeight = 200

func ToPosition(x, y int) int {
	return x + MapHeight*y
}
