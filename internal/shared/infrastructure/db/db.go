package db

import (
	"ThreeKingdoms/internal/shared/serverconfig"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// todo 替换成其他的，不一定是 mysql
func Open(cfg serverconfig.MySQLConfig, l *zap.Logger) (*gorm.DB, error) {
	if l == nil {
		l = zap.NewNop()
	}
	gcfg := &gorm.Config{
		// SQL 错误由业务层统一记录，这里关闭 gorm 内部 SQL 日志避免重复刷屏。
		Logger: logger.Default.LogMode(logger.Silent),
	}

	// username:password@protocol(address)/dbname?charset=utf8&parseTime=True&loc=Local
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
	)
	db, err := gorm.Open(mysql.Open(dsn), gcfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(cfg.MaxConn)
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)

	l.Info("open db success",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("db", cfg.DBName),
		zap.String("user", cfg.User),
	)
	return db, nil
}
