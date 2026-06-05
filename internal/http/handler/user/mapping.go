package user

import (
	userdto "eventhub-go/internal/http/dto/user"
	"eventhub-go/internal/page"
	usersvc "eventhub-go/internal/service/user"
)

func toUserInfoResponse(result usersvc.UserResult) userdto.UserInfoResponse {
	roles := result.Roles
	if roles == nil {
		roles = []string{}
	}
	return userdto.UserInfoResponse{
		ID:       result.ID,
		Username: result.Username,
		Email:    result.Email,
		Status:   result.Status,
		Roles:    roles,
	}
}

func toUserInfoPageResponse(result page.Response[usersvc.UserResult]) page.Response[userdto.UserInfoResponse] {
	items := make([]userdto.UserInfoResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toUserInfoResponse(item))
	}
	return page.Response[userdto.UserInfoResponse]{
		Items:       items,
		Page:        result.Page,
		Size:        result.Size,
		Total:       result.Total,
		TotalPages:  result.TotalPages,
		HasNext:     result.HasNext,
		HasPrevious: result.HasPrevious,
	}
}
