// Package clock 提供运行时代码使用的时间来源抽象。
package clock

import "time"

// Clock 抽象当前时间获取能力，便于业务服务在测试中注入可控时间。
type Clock interface {
	// Now 返回当前时间。
	Now() time.Time
}

// RealClock 使用操作系统时间作为真实时间来源。
type RealClock struct{}

// Now 返回当前本地时间。
func (RealClock) Now() time.Time {
	return time.Now()
}
