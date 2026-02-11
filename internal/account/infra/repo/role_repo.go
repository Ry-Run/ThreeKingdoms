package repo

import (
	"ThreeKingdoms/internal/account/domain"
	"context"
	"errors"

	"gorm.io/gorm"
)

type RoleRepo struct {
	db *gorm.DB
}

func NewRoleRepo(db *gorm.DB) *RoleRepo {
	return &RoleRepo{
		db: db,
	}
}

func (r *RoleRepo) GetRoleByUid(ctx context.Context, uid int) (*domain.Role, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).Where("uid = ?", uid).First(&role).Error
	if err == nil {
		return &role, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 技术错误 → 业务错误
		return nil, domain.ErrRoleNotFound.WithData("uid", uid)
	}
	//  纯技术错误（连接超时等），是无法转换的技术错误，保持原样或包装返回给上级
	return nil, domain.ErrSystemUnavailable.WithData("uid", uid).WithCause(err)
}

func (r *RoleRepo) Save(ctx context.Context, role domain.Role) error {
	err := r.db.WithContext(ctx).Save(&role).Error
	if err != nil {
		//  纯技术错误（连接超时等），是无法转换的技术错误，保持原样或包装返回给上级
		return domain.ErrSystemUnavailable.WithData("uid", role.UId).WithCause(err)
	}
	return nil
}
