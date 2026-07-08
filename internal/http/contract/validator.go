package contract

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/security"
)

// RequestValidator 在 generated strict handler 前执行 OpenAPI request contract gate。
type RequestValidator struct {
	router          routers.Router
	options         openapi3filter.Options
	validateRequest bool
}

type requestValidatorConfig struct {
	authenticate    func(http.Handler) http.Handler
	validateRequest bool
}

// RequestValidatorOption 调整 OpenAPI request contract gate 的运行时能力。
type RequestValidatorOption func(*requestValidatorConfig)

// WithAuthentication 注入现有 Bearer token 认证 middleware，供 OpenAPI security requirement 触发。
func WithAuthentication(authenticate func(http.Handler) http.Handler) RequestValidatorOption {
	return func(config *requestValidatorConfig) {
		config.authenticate = authenticate
	}
}

// WithRequestValidation 控制是否执行 path/query/body/content-type 等请求契约校验。
//
// 即使关闭请求校验，已注入的 authentication bridge 仍可基于 OpenAPI security requirement
// 执行认证/授权，避免 runtime validation 开关影响受保护 API 的安全边界。
func WithRequestValidation(enabled bool) RequestValidatorOption {
	return func(config *requestValidatorConfig) {
		config.validateRequest = enabled
	}
}

// NewRequestValidator 根据启动期加载完成的 OpenAPI spec 创建 request validator。
func NewRequestValidator(spec *Spec, options ...RequestValidatorOption) (*RequestValidator, error) {
	if spec == nil || spec.Document == nil {
		return nil, errors.New("openapi request contract spec is nil")
	}

	config := requestValidatorConfig{
		validateRequest: true,
	}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}

	document := *spec.Document
	// 运行时请求已经到达本服务实例，匹配时只使用应用 path，不绑定文档和客户端声明的公开 server URL。
	document.Servers = nil

	router, err := legacyrouter.NewRouter(&document)
	if err != nil {
		return nil, fmt.Errorf("initialize openapi request contract router: %w", err)
	}
	return &RequestValidator{
		router:          router,
		validateRequest: config.validateRequest,
		options: openapi3filter.Options{
			AuthenticationFunc: authenticationFunc(config.authenticate),
		},
	}, nil
}

// Middleware 根据匹配到的 OpenAPI operation 校验请求，并保证 request body 可被后续 strict handler 重读。
func (v *RequestValidator) Middleware(next http.Handler) http.Handler {
	if v == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, pathParams, err := v.router.FindRoute(r)
		if err != nil {
			response.WriteError(w, r, apperror.New(apperror.CommonNotFound, "请求的资源不存在"))
			return
		}

		input := &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: pathParams,
			Route:      route,
			Options:    &v.options,
		}

		var validateErr error
		if v.validateRequest {
			validateErr = openapi3filter.ValidateRequest(r.Context(), input)
		} else {
			validateErr = validateSecurityRequirements(r.Context(), input)
		}
		if validateErr != nil {
			response.WriteError(w, r, appErrorFromValidationError(validateErr))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func appErrorFromValidationError(err error) *apperror.AppError {
	if appErr := appErrorFromSecurityError(err); appErr != nil {
		return appErr
	}

	var requestErr *openapi3filter.RequestError
	if !errors.As(err, &requestErr) {
		return requesterror.InvalidParameters(requesterror.FieldErrors{
			"request": "请求不符合 OpenAPI 契约",
		})
	}

	if parameter := requestErr.Parameter; parameter != nil {
		name := parameter.Name
		if name == "" {
			name = "parameter"
		}
		fields := requesterror.FieldErrors{name: parameterErrorMessage(parameter, requestErr)}
		switch parameter.In {
		case openapi3.ParameterInHeader:
			return requesterror.InvalidHeaders(fields)
		case openapi3.ParameterInCookie:
			return requesterror.InvalidCookies(fields)
		default:
			return requesterror.InvalidParameters(fields)
		}
	}

	if requestErr.RequestBody != nil {
		if unsupportedContentType(requestErr) {
			return requesterror.UnsupportedContentType(contentType(requestErr))
		}
		if errors.Is(requestErr.Err, openapi3filter.ErrInvalidRequired) {
			return requesterror.MalformedBody()
		}
		if malformedBody(requestErr) {
			return requesterror.MalformedBody()
		}
		return requesterror.InvalidBody(bodyFieldErrors(requestErr))
	}

	return requesterror.InvalidParameters(requesterror.FieldErrors{
		"request": "请求不符合 OpenAPI 契约",
	})
}

func parameterErrorMessage(parameter *openapi3.Parameter, requestErr *openapi3filter.RequestError) string {
	switch parameter.In {
	case openapi3.ParameterInPath:
		return parameter.Name + " 不符合路径参数契约"
	case openapi3.ParameterInQuery:
		return parameter.Name + " 不符合查询参数契约"
	case openapi3.ParameterInHeader:
		return parameter.Name + " 不符合请求头契约"
	case openapi3.ParameterInCookie:
		return parameter.Name + " 不符合 Cookie 契约"
	default:
		if requestErr.Reason != "" {
			return requestErr.Reason
		}
		return "请求参数不符合 OpenAPI 契约"
	}
}

func unsupportedContentType(requestErr *openapi3filter.RequestError) bool {
	if strings.Contains(requestErr.Reason, "Content-Type") {
		return true
	}
	var parseErr *openapi3filter.ParseError
	return errors.As(requestErr.Err, &parseErr) &&
		(parseErr.Kind == openapi3filter.KindUnsupportedFormat || strings.Contains(parseErr.Reason, "unsupported content type"))
}

func contentType(requestErr *openapi3filter.RequestError) string {
	if requestErr.Input == nil || requestErr.Input.Request == nil {
		return ""
	}
	return requestErr.Input.Request.Header.Get("Content-Type")
}

func malformedBody(requestErr *openapi3filter.RequestError) bool {
	if strings.Contains(requestErr.Reason, "failed to decode request body") {
		return true
	}
	var parseErr *openapi3filter.ParseError
	if !errors.As(requestErr.Err, &parseErr) {
		return false
	}
	return parseErr.Kind != openapi3filter.KindUnsupportedFormat
}

func bodyFieldErrors(requestErr *openapi3filter.RequestError) requesterror.FieldErrors {
	var schemaErr *openapi3.SchemaError
	if errors.As(requestErr.Err, &schemaErr) {
		field := "body"
		if pointer := schemaErr.JSONPointer(); len(pointer) > 0 {
			field = pointer[0]
		}
		message := schemaErr.Reason
		if message == "" {
			message = "字段不符合请求体 schema"
		}
		return requesterror.FieldErrors{field: message}
	}
	return requesterror.FieldErrors{
		"body": "请求体不符合 OpenAPI schema",
	}
}

func validateSecurityRequirements(ctx context.Context, input *openapi3filter.RequestValidationInput) error {
	if input == nil || input.Route == nil || input.Route.Operation == nil {
		return nil
	}
	securityRequirements := input.Route.Operation.Security
	if securityRequirements == nil {
		securityRequirements = &input.Route.Spec.Security
	}
	if securityRequirements == nil {
		return nil
	}
	return openapi3filter.ValidateSecurityRequirements(ctx, input, *securityRequirements)
}

func authenticationFunc(authenticate func(http.Handler) http.Handler) openapi3filter.AuthenticationFunc {
	if authenticate == nil {
		return openapi3filter.NoopAuthenticationFunc
	}
	return func(_ context.Context, input *openapi3filter.AuthenticationInput) error {
		if input == nil || input.RequestValidationInput == nil || input.RequestValidationInput.Request == nil {
			return openAPIAuthenticationError(input, unauthorizedError())
		}
		if input.SecuritySchemeName != "BearerAuth" {
			return openAPIAuthenticationError(input, unauthorizedError())
		}

		request := input.RequestValidationInput.Request
		authenticatedRequest, appErr := authenticateRequest(authenticate, request)
		if appErr != nil {
			return openAPIAuthenticationError(input, appErr)
		}
		*request = *authenticatedRequest

		if appErr := authorizeRequiredRoles(input); appErr != nil {
			return openAPIAuthenticationError(input, appErr)
		}
		return nil
	}
}

func authenticateRequest(authenticate func(http.Handler) http.Handler, request *http.Request) (*http.Request, *apperror.AppError) {
	probe := newAuthenticationProbeResponseWriter()
	var authenticatedRequest *http.Request
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		authenticatedRequest = r
	})

	authenticate(next).ServeHTTP(probe, request)
	if authenticatedRequest == nil {
		return nil, appErrorFromAuthenticationStatus(probe.status)
	}
	return authenticatedRequest, nil
}

type authenticationProbeResponseWriter struct {
	header http.Header
	status int
}

func newAuthenticationProbeResponseWriter() *authenticationProbeResponseWriter {
	return &authenticationProbeResponseWriter{header: http.Header{}}
}

func (w *authenticationProbeResponseWriter) Header() http.Header {
	return w.header
}

func (w *authenticationProbeResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return len(body), nil
}

func (w *authenticationProbeResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func authorizeRequiredRoles(input *openapi3filter.AuthenticationInput) *apperror.AppError {
	roles, present, ok := requiredRoles(input.RequestValidationInput.Route.Operation)
	if !present {
		return nil
	}
	if !ok || len(roles) == 0 {
		return forbiddenError()
	}

	principal, ok := security.PrincipalFromContext(input.RequestValidationInput.Request.Context())
	if !ok {
		return unauthorizedError()
	}
	for _, role := range roles {
		if !principalHasRole(principal, role) {
			return forbiddenError()
		}
	}
	return nil
}

func requiredRoles(operation *openapi3.Operation) ([]string, bool, bool) {
	if operation == nil || operation.Extensions == nil {
		return nil, false, true
	}
	rawRoles, present := operation.Extensions["x-required-roles"]
	if !present {
		return nil, false, true
	}

	switch roles := rawRoles.(type) {
	case []string:
		return normalizeRoles(roles), true, true
	case []any:
		values := make([]string, 0, len(roles))
		for _, role := range roles {
			value, ok := role.(string)
			if !ok {
				return nil, true, false
			}
			values = append(values, value)
		}
		return normalizeRoles(values), true, true
	default:
		return nil, true, false
	}
}

func normalizeRoles(roles []string) []string {
	values := make([]string, 0, len(roles))
	for _, role := range roles {
		value := strings.ToUpper(strings.TrimSpace(role))
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func principalHasRole(principal security.Principal, role string) bool {
	required := requiredAuthority(role)
	for _, authority := range principal.Authorities {
		if strings.ToUpper(strings.TrimSpace(authority)) == required {
			return true
		}
	}
	return false
}

func requiredAuthority(role string) string {
	value := strings.ToUpper(strings.TrimSpace(role))
	if strings.HasPrefix(value, "ROLE_") {
		return value
	}
	return "ROLE_" + value
}

func openAPIAuthenticationError(input *openapi3filter.AuthenticationInput, appErr *apperror.AppError) error {
	if input == nil {
		return appErr
	}
	return input.NewError(appErr)
}

func appErrorFromAuthenticationStatus(status int) *apperror.AppError {
	switch status {
	case http.StatusForbidden:
		return forbiddenError()
	case http.StatusNotFound:
		return apperror.New(apperror.CommonNotFound, "请求的资源不存在")
	case http.StatusInternalServerError:
		return apperror.New(apperror.CommonInternal, "")
	default:
		return unauthorizedError()
	}
}

func appErrorFromSecurityError(err error) *apperror.AppError {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	var securityErr *openapi3filter.SecurityRequirementsError
	if errors.As(err, &securityErr) {
		for _, child := range securityErr.Errors {
			if appErr := appErrorFromSecurityError(child); appErr != nil {
				return appErr
			}
		}
		return unauthorizedError()
	}
	return nil
}

func unauthorizedError() *apperror.AppError {
	return apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录")
}

func forbiddenError() *apperror.AppError {
	return apperror.New(apperror.AuthForbidden, "权限不足")
}
