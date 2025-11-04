package biz

import (
	"context"
	"strings"

	"github.com/xinghe903/chatify/auth/internal/biz/bo"

	"github.com/xinghe903/chatify/pkg/verify"

	v1 "github.com/xinghe903/chatify/api/auth/v1"

	"github.com/go-kratos/kratos/v2/log"
)

func checkField(c Checker, ctx context.Context, field string) error {
	return c.Check(ctx, field)
}

type Checker interface {
	Check(ctx context.Context, field string) error
}

type UsernameCheck struct {
	userRepo UserRepo
	log      *log.Helper
}

func NewUsernameCheck(userRepo UserRepo, logger *log.Helper) Checker {
	return &UsernameCheck{
		userRepo: userRepo,
		log:      logger,
	}
}

func (u *UsernameCheck) Check(ctx context.Context, username string) error {
	// 用户名规则： 不允许为手机号,且不能包含特殊符号（@, 空格）
	if len(username) > bo.UserNameMaxLength {
		return v1.ErrorUserNameInvalid("username is too long")
	}
	//  检查是否包含禁止字符：@ 或 空格
	if strings.ContainsRune(username, '@') || strings.ContainsAny(username, " \t\n\r") {
		return v1.ErrorUserNameInvalid("username cannot contain special characters(@ or space)")
	}
	// 用户名不允许为手机号
	if verify.IsMobileNumber(username) {
		return v1.ErrorUserNameInvalid("username cannot be a mobile number")
	}
	// 验证用户名是否已存在
	if user, err := u.userRepo.GetByUsername(ctx, username); err != nil {
		u.log.WithContext(ctx).Infof("Failed to get user err=%v, username=%s", err, username)
		return v1.ErrorUserNameInvalid("get username failed")
	} else if user != nil {
		return v1.ErrorUserNameInvalid("username already exists")
	}
	return nil
}

type EmailCheck struct {
	userRepo UserRepo
	log      *log.Helper
}

func NewEmailCheck(userRepo UserRepo, logger *log.Helper) Checker {
	return &EmailCheck{
		userRepo: userRepo,
		log:      logger,
	}
}

func (u *EmailCheck) Check(ctx context.Context, email string) error {
	// 邮箱规则： 校验规则为邮箱
	if len(email) > bo.EmailMaxLength {
		return v1.ErrorEmailInvalid("email is too long")
	}
	if !verify.IsEmail(email) {
		return v1.ErrorEmailInvalid("invalid email")
	}

	// 验证邮箱是否已存在
	if user, err := u.userRepo.GetByEmail(ctx, email); err != nil {
		u.log.WithContext(ctx).Infof("Failed to get user err=%v, email=%s", err, email)
		return v1.ErrorEmailInvalid("get email failed")
	} else if user != nil {
		return v1.ErrorEmailInvalid("email already exists")
	}
	return nil
}

type PhoneCheck struct {
	userRepo UserRepo
	log      *log.Helper
}

func NewPhoneCheck(userRepo UserRepo, logger *log.Helper) Checker {
	return &PhoneCheck{
		userRepo: userRepo,
		log:      logger,
	}
}

func (u *PhoneCheck) Check(ctx context.Context, phone string) error {
	// 手机号可以为空
	if len(phone) == 0 {
		return nil
	}
	// 手机号规则： 校验规则为中国手机号
	if !verify.IsMobileNumber(phone) {
		return v1.ErrorPhoneInvalid("phone cannot be a mobile number")
	}
	// 验证手机号是否已存在
	if user, err := u.userRepo.GetByPhone(ctx, phone); err != nil {
		u.log.WithContext(ctx).Infof("Failed to get user err=%v, phone=%s", err, phone)
		return v1.ErrorPhoneInvalid("get phone failed")
	} else if user != nil {
		return v1.ErrorPhoneInvalid("phone already exists")
	}
	return nil
}

type UserStatusCheck struct {
	log *log.Helper
}

func NewUserStatusCheck(logger *log.Helper) Checker {
	return &UserStatusCheck{
		log: logger,
	}
}

func (u *UserStatusCheck) Check(ctx context.Context, status string) error {
	s := bo.UserStatus(status)
	if s != bo.UserStatusActive {
		statusMsg := "user is not active"
		if s == bo.UserStatusRevoked {
			statusMsg = "user account has been revoked"
		} else if s == bo.UserStatusLocked {
			statusMsg = "user account is locked"
		}
		u.log.WithContext(ctx).Infof("User status is invalid: %s", s)
		return v1.ErrorUserStatusInvalid(statusMsg)
	}
	return nil
}
