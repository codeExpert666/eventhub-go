// Package system 定义 system HTTP 请求与响应的数据契约。
package system

// EchoRequest 表示 POST /api/v1/system/echo 的请求体。
type EchoRequest struct {
	Message string  `json:"message"`
	Tag     *string `json:"tag"`
}
