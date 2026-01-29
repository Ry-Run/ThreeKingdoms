package security

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrJWTSecretMissing = errors.New("JWT_SECRET is not set")

type Claims struct {
	Uid int `json:"uid"`
	jwt.RegisteredClaims
}

func jwtSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, ErrJWTSecretMissing
	}
	return []byte(secret), nil
}

// Award 生成 Token（默认 7 天过期）。
func Award(uid int) (string, error) {
	key, err := jwtSecret()
	if err != nil {
		return "", err
	}

	now := time.Now()
	expireTime := now.Add(7 * 24 * time.Hour)
	claims := &Claims{
		Uid: uid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expireTime),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(key)
}

// ParseToken 解析并验证 Token。
func ParseToken(tokenStr string) (*jwt.Token, *Claims, error) {
	key, err := jwtSecret()
	if err != nil {
		return nil, nil, err
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return key, nil
	})
	if err != nil {
		return nil, nil, err
	}
	if token == nil || !token.Valid {
		return nil, nil, jwt.ErrTokenInvalidClaims
	}
	return token, claims, nil
}
