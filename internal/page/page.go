package page

import "fmt"

const (
	DefaultPage = 1
	DefaultSize = 20
	MaxSize     = 100
)

type Request struct {
	Page int `json:"page"`
	Size int `json:"size"`
}

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

func DefaultRequest() Request {
	return Request{Page: DefaultPage, Size: DefaultSize}
}

func (r Request) Offset() int64 {
	return int64(r.Page-1) * int64(r.Size)
}

type Response[T any] struct {
	Items       []T   `json:"items"`
	Page        int   `json:"page"`
	Size        int   `json:"size"`
	Total       int64 `json:"total"`
	TotalPages  int64 `json:"totalPages"`
	HasNext     bool  `json:"hasNext"`
	HasPrevious bool  `json:"hasPrevious"`
}

func NewResponse[T any](items []T, request Request, total int64) (Response[T], error) {
	if request.Page < 1 {
		return Response[T]{}, fmt.Errorf("page must be greater than or equal to 1")
	}
	if request.Size < 1 {
		return Response[T]{}, fmt.Errorf("size must be greater than or equal to 1")
	}
	if request.Size > MaxSize {
		return Response[T]{}, fmt.Errorf("size must be less than or equal to %d", MaxSize)
	}
	if total < 0 {
		return Response[T]{}, fmt.Errorf("total must be greater than or equal to 0")
	}

	copiedItems := make([]T, len(items))
	copy(copiedItems, items)

	totalPages := calculateTotalPages(total, request.Size)
	return Response[T]{
		Items:       copiedItems,
		Page:        request.Page,
		Size:        request.Size,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     int64(request.Page) < totalPages,
		HasPrevious: totalPages > 0 && request.Page > 1 && int64(request.Page) <= totalPages,
	}, nil
}

func calculateTotalPages(total int64, size int) int64 {
	if total == 0 {
		return 0
	}
	return (total + int64(size) - 1) / int64(size)
}
