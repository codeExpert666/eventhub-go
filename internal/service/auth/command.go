package auth

// RegisterCommand 表示注册用户的写入输入。
type RegisterCommand struct {
	Username string
	Email    string
	Password string
}

// LoginCommand 表示登录用户的写入输入。
type LoginCommand struct {
	UsernameOrEmail string
	Password        string
}
