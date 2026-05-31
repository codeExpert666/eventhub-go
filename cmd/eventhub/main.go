// Package main 提供 EventHub HTTP 服务的可执行入口。
package main

import (
	"os"

	"eventhub-go/internal/app"
)

// main 启动 EventHub 应用，并在启动或运行失败时以非零状态码退出进程。
func main() {
	if err := app.Run(); err != nil {
		os.Exit(1)
	}
}
