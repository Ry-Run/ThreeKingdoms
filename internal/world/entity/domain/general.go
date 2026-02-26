package domain

import "time"

// entity
type General struct {
	id             int
	cfgId          int       // 配置id
	power          int       // 体力
	level          int8      //
	exp            int       // 经验
	order          int8      // 第几队
	cityId         int       // 城市id
	createdAt      time.Time //
	curArms        int       // 兵种
	hasPrPoint     int       // 总属性点
	usePrPoint     int       // 已用属性点
	attackDistance int       // 攻击距离
	forceAdded     int       // 已加攻击属性
	strategyAdded  int       // 已加战略属性
	defenseAdded   int       // 已加防御属性
	speedAdded     int       // 已加速度属性
	destroyAdded   int       // 已加破坏属性
	starLv         int8      // 稀有度(星级)进阶等级级
	star           int8      // 稀有度(星级)
	parentId       int       // 已合成到武将的id
	skills         []*GSkill //携带的技能
	state          int8      // 0:正常，1:转换掉了
}
