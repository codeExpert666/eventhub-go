package page

import "fmt"

const (
	DefaultPage = 1
	DefaultSize = 20
	MaxSize     = 100
)

// Request describes a 1-based page request.
type Request struct {
	Page int `json:"page"`
	Size int `json:"size"`
}

// NewRequest validates and creates a page request.
func NewRequest(page, size int) (Request, error) {
	if page < 1 {
		return Request{}, fmt.Errorf("page must be greater than or equal to 1")
	}
	if size < 1 {
		return Request{}, fmt.Errorf("size must be greater than or equal to 1")
	}
	if size > MaxSize {
		return Request{}, fmt.Errorf("size must be less than or equal to %d", MaxSize)
	}
	return Request{Page: page, Size: size}, nil
}

// DefaultRequest returns the default page request.
func DefaultRequest() Request {
	return Request{Page: DefaultPage, Size: DefaultSize}
}

// Offset converts the 1-based page request into a 0-based database offset.
func (r Request) Offset() int64 {
	return int64(r.Page-1) * int64(r.Size)
}
