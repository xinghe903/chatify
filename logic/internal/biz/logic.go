package biz

import (
	"github.com/go-kratos/kratos/v2/log"
)

type UserRepo interface {
}

type Logic struct {
	log  *log.Helper
	user UserRepo
}

// NewLogic
func NewLogic(logger log.Logger, user UserRepo) *Logic {
	return &Logic{
		log:  log.NewHelper(logger),
		user: user,
	}
}
