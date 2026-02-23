package entity

import (
	"errors"
)

var (
	ErrPlayerNotFound = errors.New("player not found")
	ErrCreateCity     = errors.New("create city failed")
)
