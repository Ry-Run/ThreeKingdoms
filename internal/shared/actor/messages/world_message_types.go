package messages

type WorldCell struct {
	Type     int8   `json:"type"`
	Name     string `json:"name"`
	Level    int8   `json:"level"`
	Grain    int    `json:"grain"`
	Wood     int    `json:"wood"`
	Iron     int    `json:"iron"`
	Stone    int    `json:"stone"`
	Durable  int    `json:"durable"`
	Defender int    `json:"defender"`
}

type WorldCity struct {
	CityId     int64  `json:"city_id"`
	Name       string `json:"name"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	IsMain     bool   `json:"is_main"`
	Level      int8   `json:"level"`
	CurDurable int    `json:"cur_durable"`
	MaxDurable int    `json:"max_durable"`
	OccupyTime int64  `json:"occupy_time"` // 毫秒时间戳
	UnionId    int    `json:"union_id"`
	UnionName  string `json:"union_name"`
	ParentId   int    `json:"parent_id"`
}
