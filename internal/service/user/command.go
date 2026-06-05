package user

// UpdateUserStatusCommand 表示管理员更新用户状态的写操作输入。
type UpdateUserStatusCommand struct {
	UserID int64
	Status string
}
