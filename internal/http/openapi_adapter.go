package http

import (
	"context"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	authhandler "eventhub-go/internal/http/handler/auth"
	systemhandler "eventhub-go/internal/http/handler/system"
	userhandler "eventhub-go/internal/http/handler/user"
)

// openAPIAdapter 聚合业务模块 handler，实现 generated StrictServerInterface。
//
// 这个类型只负责把 generated operation 分发给对应业务 handler；具体 HTTP 入参映射、字段校验、
// service 调用和响应模型转换仍留在各业务模块 handler 中。
type openAPIAdapter struct {
	system *systemhandler.Handler
	auth   *authhandler.Handler
	user   *userhandler.Handler
}

// newOpenAPIAdapter 创建生产 strict server 适配器。
func newOpenAPIAdapter(system *systemhandler.Handler, auth *authhandler.Handler, user *userhandler.Handler) *openAPIAdapter {
	return &openAPIAdapter{
		system: system,
		auth:   auth,
		user:   user,
	}
}

// Health 处理 GET /actuator/health。
func (a *openAPIAdapter) Health(ctx context.Context, request openapigen.HealthRequestObject) (openapigen.HealthResponseObject, error) {
	if a.system == nil {
		return nil, routeNotAvailable()
	}
	return a.system.HealthStrict(ctx, request)
}

// HealthHead 处理 HEAD /actuator/health。
func (a *openAPIAdapter) HealthHead(ctx context.Context, request openapigen.HealthHeadRequestObject) (openapigen.HealthHeadResponseObject, error) {
	if a.system == nil {
		return nil, routeNotAvailable()
	}
	return a.system.HealthHeadStrict(ctx, request)
}

// Info 处理 GET /actuator/info。
func (a *openAPIAdapter) Info(ctx context.Context, request openapigen.InfoRequestObject) (openapigen.InfoResponseObject, error) {
	if a.system == nil {
		return nil, routeNotAvailable()
	}
	return a.system.InfoStrict(ctx, request)
}

// InfoHead 处理 HEAD /actuator/info。
func (a *openAPIAdapter) InfoHead(ctx context.Context, request openapigen.InfoHeadRequestObject) (openapigen.InfoHeadResponseObject, error) {
	if a.system == nil {
		return nil, routeNotAvailable()
	}
	return a.system.InfoHeadStrict(ctx, request)
}

// ListAdminUsers 处理 GET /api/v1/admin/users。
func (a *openAPIAdapter) ListAdminUsers(ctx context.Context, request openapigen.ListAdminUsersRequestObject) (openapigen.ListAdminUsersResponseObject, error) {
	if a.user == nil {
		return nil, routeNotAvailable()
	}
	return a.user.ListAdminUsersStrict(ctx, request)
}

// UpdateAdminUserStatus 处理 PATCH /api/v1/admin/users/{userId}/status。
func (a *openAPIAdapter) UpdateAdminUserStatus(ctx context.Context, request openapigen.UpdateAdminUserStatusRequestObject) (openapigen.UpdateAdminUserStatusResponseObject, error) {
	if a.user == nil {
		return nil, routeNotAvailable()
	}
	return a.user.UpdateAdminUserStatusStrict(ctx, request)
}

// Login 处理 POST /api/v1/auth/login。
func (a *openAPIAdapter) Login(ctx context.Context, request openapigen.LoginRequestObject) (openapigen.LoginResponseObject, error) {
	if a.auth == nil {
		return nil, routeNotAvailable()
	}
	return a.auth.LoginStrict(ctx, request)
}

// Logout 处理 POST /api/v1/auth/logout。
func (a *openAPIAdapter) Logout(ctx context.Context, request openapigen.LogoutRequestObject) (openapigen.LogoutResponseObject, error) {
	if a.auth == nil {
		return nil, routeNotAvailable()
	}
	return a.auth.LogoutStrict(ctx, request)
}

// RefreshToken 处理 POST /api/v1/auth/refresh。
func (a *openAPIAdapter) RefreshToken(ctx context.Context, request openapigen.RefreshTokenRequestObject) (openapigen.RefreshTokenResponseObject, error) {
	if a.auth == nil {
		return nil, routeNotAvailable()
	}
	return a.auth.RefreshTokenStrict(ctx, request)
}

// Register 处理 POST /api/v1/auth/register。
func (a *openAPIAdapter) Register(ctx context.Context, request openapigen.RegisterRequestObject) (openapigen.RegisterResponseObject, error) {
	if a.auth == nil {
		return nil, routeNotAvailable()
	}
	return a.auth.RegisterStrict(ctx, request)
}

// GetCurrentUser 处理 GET /api/v1/me。
func (a *openAPIAdapter) GetCurrentUser(ctx context.Context, request openapigen.GetCurrentUserRequestObject) (openapigen.GetCurrentUserResponseObject, error) {
	if a.user == nil {
		return nil, routeNotAvailable()
	}
	return a.user.GetCurrentUserStrict(ctx, request)
}

// Echo 处理 POST /api/v1/system/echo。
func (a *openAPIAdapter) Echo(ctx context.Context, request openapigen.EchoRequestObject) (openapigen.EchoResponseObject, error) {
	if a.system == nil {
		return nil, routeNotAvailable()
	}
	return a.system.EchoStrict(ctx, request)
}

// Ping 处理 GET /api/v1/system/ping。
func (a *openAPIAdapter) Ping(ctx context.Context, request openapigen.PingRequestObject) (openapigen.PingResponseObject, error) {
	if a.system == nil {
		return nil, routeNotAvailable()
	}
	return a.system.PingStrict(ctx, request)
}

func routeNotAvailable() *apperror.AppError {
	return apperror.New(apperror.CommonNotFound, "请求的资源不存在")
}
