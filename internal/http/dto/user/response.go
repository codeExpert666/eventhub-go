// Package user 定义用户模块 HTTP 响应 DTO。
package user

// UserInfoResponse 表示对外返回的用户摘要。
type UserInfoResponse struct {
	ID       int64    `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Status   string   `json:"status"`
	Roles    []string `json:"roles"`
}
