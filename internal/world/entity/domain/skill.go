package domain

// entity
type GSkill struct {
	id    int
	lv    int
	cfgId int
}

// entity
type Skill struct {
	id             int
	cfgId          int
	belongGenerals string
	generals       []int
}
