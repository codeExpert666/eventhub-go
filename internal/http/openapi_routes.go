package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
	"eventhub-go/internal/http/response"
)

// registerOpenAPIRoutes 将 eventhub.yaml 生成的 strict server routes 接入主 chi router。
//
// 这里是业务 API 与 actuator API 的运行时 route 入口：path/method 由 oapi-codegen 生成代码注册，
// 本函数只负责把生成代码需要的 handler、middleware 和错误处理器接到项目统一 HTTP 外壳中。
func registerOpenAPIRoutes(router chi.Router, deps RouterDependencies) {
	// openAPIAdapter 聚合各业务模块 handler，实现 generated StrictServerInterface。
	// 缺失模块能力由 adapter 返回 COMMON-404，避免 router 重新按模块维护路由表。
	strictServer := newOpenAPIAdapter(deps.System, deps.Auth, deps.User)
	baseRouter := router
	if deps.RequestContract != nil {
		// Contract gate 必须作为 chi route-level middleware 包住 generated wrapper，
		// 才能先于 generated path/query 绑定执行完整 OpenAPI request validation。
		baseRouter = router.With(deps.RequestContract)
	}
	openapigen.HandlerWithOptions(
		openapigen.NewStrictHandlerWithOptions(
			strictServer,
			nil,
			openapigen.StrictHTTPServerOptions{
				// strict handler 在进入业务方法前解码 JSON body；失败时统一写为请求体格式错误。
				RequestErrorHandlerFunc: writeOpenAPIRequestBodyError,
				// strict handler 在业务方法返回后写出 generated response；业务 error、响应类型不匹配
				// 或写出失败时统一写为错误 envelope。
				ResponseErrorHandlerFunc: writeOpenAPIResponseError,
			},
		),
		openapigen.ChiServerOptions{
			BaseRouter: baseRouter,
			// generated chi wrapper 在执行 middleware 前绑定 path/query 参数；失败时统一写为字段级参数校验错误。
			ErrorHandlerFunc: writeOpenAPIParameterError,
		},
	)
}

// writeOpenAPIRequestBodyError 将 strict handler 的 JSON body 解码错误映射为统一请求体格式错误。
func writeOpenAPIRequestBodyError(w http.ResponseWriter, r *http.Request, _ error) {
	response.WriteError(w, r, requesterror.MalformedBody())
}

// writeOpenAPIResponseError 将 strict handler 传出的业务错误、响应类型错误或写出错误写成统一错误响应。
func writeOpenAPIResponseError(w http.ResponseWriter, r *http.Request, err error) {
	response.WriteError(w, r, apperror.FromErrorOrInternal(err))
}

// writeOpenAPIParameterError 将 generated chi wrapper 的 path/query 绑定错误写成统一参数校验响应。
func writeOpenAPIParameterError(w http.ResponseWriter, r *http.Request, err error) {
	response.WriteError(w, r, parameterValidationError(err))
}

// parameterValidationError 从 oapi-codegen 参数错误中提取字段名，并转成前端稳定可读的字段错误。
func parameterValidationError(err error) *apperror.AppError {
	field := "parameter"
	var invalidFormat *openapigen.InvalidParamFormatError
	if errors.As(err, &invalidFormat) {
		field = invalidFormat.ParamName
	}
	var tooManyValues *openapigen.TooManyValuesForParamError
	if errors.As(err, &tooManyValues) {
		field = tooManyValues.ParamName
	}

	message := field + " 格式不合法"
	// page/size/userId 是当前 OpenAPI 参数中对用户最常见的输入错误，保留更明确的中文提示。
	switch field {
	case "page", "size":
		message = field + " 必须是整数"
	case "userId":
		message = "userId 必须是正整数"
	}
	return requesterror.InvalidParameters(requesterror.FieldErrors{field: message})
}
