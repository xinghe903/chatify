package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword 加盐哈希
// @param password 输入的密码
// @return string 存储的哈希
// @return error 错误
func HashPassword(password string) (string, error) {
	// 使用 bcrypt 自动生成盐值并哈希
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashed), err
}

// CheckPassword 比对哈希
// @param hashedPassword 存储的哈希
// @param password 输入的密码
// @return error 错误
func CheckPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
