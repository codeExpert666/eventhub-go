// Package password 提供密码哈希和校验能力。
package password

import "golang.org/x/crypto/bcrypt"

// BCryptHasher 使用 BCrypt 保存用户密码哈希。
type BCryptHasher struct {
	cost int
}

// NewBCryptHasher 创建使用默认成本的 BCrypt hasher。
func NewBCryptHasher() *BCryptHasher {
	return &BCryptHasher{cost: bcrypt.DefaultCost}
}

// NewBCryptHasherWithCost 创建使用指定成本的 BCrypt hasher。
func NewBCryptHasherWithCost(cost int) *BCryptHasher {
	return &BCryptHasher{cost: cost}
}

// Hash 将明文密码转换为 BCrypt hash。
func (h *BCryptHasher) Hash(plain string) (string, error) {
	cost := bcrypt.DefaultCost
	if h != nil && h.cost > 0 {
		cost = h.cost
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// Matches 判断明文密码是否匹配已保存的 BCrypt hash。
func (h *BCryptHasher) Matches(plain, hashed string) (bool, error) {
	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)); err != nil {
		return false, nil
	}
	return true, nil
}
