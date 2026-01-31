package repo

import (
	"ThreeKingdoms/internal/account/domain"
	"context"
	"errors"

	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{
		db: db,
	}
}

func (r *UserRepo) GetUserByUserName(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err == nil {
		return &user, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 技术错误 → 业务错误
		return nil, domain.ErrUserNotFound.WithData("username", username)
	}
	//  纯技术错误（连接超时等），是无法转换的技术错误，保持原样或包装返回给上级
	return nil, domain.ErrSystemUnavailable.WithData("username", username).WithCause(err)
}

func (r *UserRepo) Save(ctx context.Context, user domain.User) error {
	err := r.db.WithContext(ctx).Save(&user).Error
	if err != nil {
		//  纯技术错误（连接超时等），是无法转换的技术错误，保持原样或包装返回给上级
		return domain.ErrSystemUnavailable.WithData("username", user.Username).WithCause(err)
	}
	return nil
}
