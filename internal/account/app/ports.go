package app

import (
	"ThreeKingdoms/internal/account/domain"
	"context"
)

type UserRepo interface {
	GetUserByUserName(ctx context.Context, username string) (*domain.User, error)
	Save(ctx context.Context, n domain.User) error
}

type LoginHistoryRepo interface {
	Save(ctx context.Context, history domain.LoginHistory) error
}

type LoginLastRepo interface {
	GetLoginLast(ctx context.Context, uid int) (domain.LoginLast, error)
	Save(ctx context.Context, ll domain.LoginLast) error
}

type PwdEncrypter func(pwd, passcode string) string

type RandSeq func(n int) string
