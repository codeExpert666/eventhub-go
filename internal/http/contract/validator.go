package contract

import (
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
)

// RequestValidator 在 generated strict handler 前执行 OpenAPI request contract gate。
type RequestValidator struct {
	router  routers.Router
	options openapi3filter.Options
}

// NewRequestValidator 根据启动期加载完成的 OpenAPI spec 创建 request validator。
func NewRequestValidator(spec *Spec) (*RequestValidator, error) {
	if spec == nil || spec.Document == nil {
		return nil, errors.New("openapi request contract spec is nil")
	}

	document := *spec.Document
	// 运行时请求已经到达本服务实例，匹配时只使用应用 path，不绑定文档和客户端声明的公开 server URL。
	document.Servers = nil

	router, err := legacyrouter.NewRouter(&document)
	if err != nil {
		return nil, fmt.Errorf("initialize openapi request contract router: %w", err)
	}
	return &RequestValidator{
		router: router,
		options: openapi3filter.Options{
			// 阶段三只校验 path/query/body/content-type；BearerAuth 与 x-required-roles
			// 暂时继续留在现有 security middleware，阶段四再迁移到这里。
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
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
		if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
			response.WriteError(w, r, appErrorFromValidationError(err))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func appErrorFromValidationError(err error) *apperror.AppError {
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
		return requesterror.InvalidParameters(requesterror.FieldErrors{
			name: parameterErrorMessage(parameter, requestErr),
		})
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
