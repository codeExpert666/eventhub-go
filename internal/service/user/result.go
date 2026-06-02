package user

// UserResult 是 service 层对外返回的用户摘要。
type UserResult struct {
	ID       int64
	Username string
	Email    string
	Status   string
	Roles    []string
}
