package model

// model
type Resource struct {
	Id     uint32 `gorm:"column:id;type:int UNSIGNED;comment:id;primaryKey;not null;" json:"id"` // id
	Rid    uint32 `gorm:"column:rid;type:int UNSIGNED;comment:rid;not null;" json:"rid"`         // rid
	Wood   uint32 `gorm:"column:wood;type:int UNSIGNED;comment:木;not null;" json:"wood"`         // 木
	Iron   uint32 `gorm:"column:iron;type:int UNSIGNED;comment:铁;not null;" json:"iron"`         // 铁
	Stone  uint32 `gorm:"column:stone;type:int UNSIGNED;comment:石头;not null;" json:"stone"`      // 石头
	Grain  uint32 `gorm:"column:grain;type:int UNSIGNED;comment:粮食;not null;" json:"grain"`      // 粮食
	Gold   uint32 `gorm:"column:gold;type:int UNSIGNED;comment:金币;not null;" json:"gold"`        // 金币
	Decree uint32 `gorm:"column:decree;type:int UNSIGNED;comment:令牌;not null;" json:"decree"`    // 令牌
}

func (r *Resource) TableName() string {
	return "role_res_1"
}
