// Package clock provides time sources for runtime code.
package clock

import "time"

// Clock abstracts current time access for services that need testable timestamps.
type Clock interface {
	Now() time.Time
}

// RealClock reads time from the operating system.
type RealClock struct{}

// Now returns the current local time.
func (RealClock) Now() time.Time {
	return time.Now()
}
