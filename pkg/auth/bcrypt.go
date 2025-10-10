package auth

import "golang.org/x/crypto/bcrypt"

// 加盐哈希
func HashPassword(password string) (string, error) {
	// 使用 bcrypt 自动生成盐值并哈希
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashed), err
}

// 比对哈希
func CheckPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
