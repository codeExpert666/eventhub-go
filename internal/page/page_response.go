package page

import "fmt"

// Response describes a paged response with derived pagination metadata.
type Response[T any] struct {
	Items       []T   `json:"items"`
	Page        int   `json:"page"`
	Size        int   `json:"size"`
	Total       int64 `json:"total"`
	TotalPages  int64 `json:"totalPages"`
	HasNext     bool  `json:"hasNext"`
	HasPrevious bool  `json:"hasPrevious"`
}

// NewResponse validates the request and derives pagination metadata.
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
