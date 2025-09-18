package biz

import (
	"github.com/go-kratos/kratos/v2/log"
)

type UserRepo interface {
}

type Push struct {
	log  *log.Helper
	user UserRepo
}

// NewPush
func NewPush(logger log.Logger, user UserRepo) *Push {
	return &Push{
		log:  log.NewHelper(logger),
		user: user,
	}
}
