package db

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ThreeKingdoms/internal/shared/config"
	"ThreeKingdoms/internal/shared/logs"
)

// todo 替换成其他的，不一定是 mysql
func Open(cfg config.MySQLConfig) (*gorm.DB, error) {

	gcfg := &gorm.Config{
		Logger: logs.NewGormLogger(logger.Info, 200*time.Millisecond),
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

	logs.Info("open db success", zap.String("url", dsn))
	return db, nil
}
