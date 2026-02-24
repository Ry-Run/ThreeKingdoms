package domain

// entity
type WarReport struct {
	id                int
	attacker          int // 进攻方 id
	defender          int // 防守方 id
	begAttackArmy     *Army
	begDefenseArmy    *Army
	endAttackArmy     *Army
	endDefenseArmy    *Army
	begAttackGeneral  *General
	begDefenseGeneral *General
	endAttackGeneral  *General
	endDefenseGeneral *General
	result            int    // 0失败，1打平，2胜利
	rounds            string // 回合
	attackIsRead      bool
	defenseIsRead     bool
	destroyDurable    int
	occupy            int
	x                 int
	y                 int
	cTime             int
}
