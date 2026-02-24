package domain

// entity
type Facility struct {
	id           int
	name         string
	privateLevel int8 //等级，外部读的时候不能直接读，要用GetLevel
	fType        int8
	upTime       int64 //升级的时间戳，0表示该等级已经升级完成了
}
