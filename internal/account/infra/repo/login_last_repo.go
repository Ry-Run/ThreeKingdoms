package repo

import (
	"ThreeKingdoms/internal/account/domain"
	"context"
	"errors"

	"gorm.io/gorm"
)

type LoginLastRepo struct {
	db *gorm.DB
}

func NewLoginLastRepo(db *gorm.DB) *LoginLastRepo {
	return &LoginLastRepo{
		db: db,
	}
}

func (r *LoginLastRepo) GetLoginLast(ctx context.Context, uid int) (domain.LoginLast, error) {
	var ll domain.LoginLast
	err := r.db.WithContext(ctx).Where("uid = ?", uid).First(&ll).Error
	if err == nil {
		return ll, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 技术错误 → 业务错误
		return domain.LoginLast{}, domain.ErrLastLoginNotFound.WithData("uid", uid)
	}
	//  纯技术错误（连接超时等），是无法转换的技术错误，保持原样或包装返回给上级
	return domain.LoginLast{}, domain.ErrSystemUnavailable.WithData("uid", uid).WithCause(err)
}

func (r *LoginLastRepo) Save(ctx context.Context, ll domain.LoginLast) error {
	// 这里使用 Save 实现 upsert 语义：
	// - Id==0 时插入新记录
	// - Id!=0 时按主键更新记录
	err := r.db.WithContext(ctx).Save(&ll).Error
	if err != nil {
		//  纯技术错误（连接超时等），是无法转换的技术错误，保持原样或包装返回给上级
		return domain.ErrSystemUnavailable.WithData("uid", ll.UId).WithCause(err)
	}
	return nil
}
