package user

import (
	"testing"

	"eventhub-go/internal/page"
	usersvc "eventhub-go/internal/service/user"
)

func TestToOpenAPIAdminUserPageMapsPaginationAndItems(t *testing.T) {
	result := page.Response[usersvc.UserResult]{
		Items: []usersvc.UserResult{
			{
				ID:       42,
				Username: "alice",
				Email:    "alice@example.com",
				Status:   "ENABLED",
			},
			{
				ID:       43,
				Username: "bob",
				Email:    "bob@example.com",
				Status:   "DISABLED",
				Roles:    []string{"ADMIN"},
			},
		},
		Page:        2,
		Size:        20,
		Total:       42,
		TotalPages:  3,
		HasNext:     true,
		HasPrevious: true,
	}

	data := toOpenAPIAdminUserPage(result)

	if data.Page != result.Page ||
		data.Size != result.Size ||
		data.Total != result.Total ||
		data.TotalPages != result.TotalPages ||
		data.HasNext != result.HasNext ||
		data.HasPrevious != result.HasPrevious {
		t.Fatalf("page data = %#v, want pagination fields from result", data)
	}
	if len(data.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(data.Items))
	}
	if data.Items[0].Id != result.Items[0].ID ||
		data.Items[0].Username != result.Items[0].Username ||
		string(data.Items[0].Email) != result.Items[0].Email ||
		string(data.Items[0].Status) != result.Items[0].Status {
		t.Fatalf("first item = %#v, want first user result mapped", data.Items[0])
	}
	if data.Items[0].Roles == nil || len(data.Items[0].Roles) != 0 {
		t.Fatalf("first item roles = %#v, want empty non-nil roles", data.Items[0].Roles)
	}
	if len(data.Items[1].Roles) != 1 || data.Items[1].Roles[0] != "ADMIN" {
		t.Fatalf("second item roles = %#v, want ADMIN role", data.Items[1].Roles)
	}
}
