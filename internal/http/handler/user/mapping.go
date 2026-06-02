package user

import (
	userdto "eventhub-go/internal/http/dto/user"
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
