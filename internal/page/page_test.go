package page_test

import (
	"testing"

	"eventhub-go/internal/page"
)

func TestDefaultRequest(t *testing.T) {
	request := page.DefaultRequest()
	if request.Page != 1 || request.Size != 20 {
		t.Fatalf("unexpected default page request: %+v", request)
	}
}

func TestRequestOffset(t *testing.T) {
	request, err := page.NewRequest(3, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if request.Offset() != 40 {
		t.Fatalf("unexpected offset: %d", request.Offset())
	}
}

func TestRequestRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		page int
		size int
	}{
		{name: "page less than one", page: 0, size: 20},
		{name: "size less than one", page: 1, size: 0},
		{name: "size too large", page: 1, size: page.MaxSize + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := page.NewRequest(tt.page, tt.size); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestResponseDerivesPaginationMetadata(t *testing.T) {
	request, err := page.NewRequest(2, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	response, err := page.NewResponse([]string{"c", "d"}, request, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.TotalPages != 3 {
		t.Fatalf("unexpected total pages: %d", response.TotalPages)
	}
	if !response.HasNext {
		t.Fatal("expected has next")
	}
	if !response.HasPrevious {
		t.Fatal("expected has previous")
	}
}

func TestResponseForEmptyResult(t *testing.T) {
	response, err := page.NewResponse([]string{}, page.DefaultRequest(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.TotalPages != 0 || response.HasNext || response.HasPrevious {
		t.Fatalf("unexpected empty pagination metadata: %+v", response)
	}
}
