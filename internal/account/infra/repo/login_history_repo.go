package repo

import (
	"ThreeKingdoms/internal/account/domain"
	"context"

	"gorm.io/gorm"
)

type LoginHistoryRepo struct {
	db *gorm.DB
}

func NewLoginHistoryRepo(db *gorm.DB) *LoginHistoryRepo {
	return &LoginHistoryRepo{
		db: db,
	}
}

func (r *LoginHistoryRepo) Save(ctx context.Context, history domain.LoginHistory) error {
	err := r.db.WithContext(ctx).Create(&history).Error
	if err != nil {
		//  纯技术错误（连接超时等），是无法转换的技术错误，保持原样或包装返回给上级
		return domain.ErrSystemUnavailable.WithData("uid", history.UId).WithCause(err)
	}
	return nil
}
