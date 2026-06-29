// Package openapi 汇总 EventHub OpenAPI 契约与本地文档资源的路径常量。
package openapi

const (
	// AssetRoot 是仓库内 OpenAPI 静态资源根目录。
	AssetRoot = "api/openapi"
	// SpecPath 是相对于资源根目录的 OpenAPI YAML 契约路径。
	SpecPath = "eventhub.yaml"
	// SwaggerDirPath 是相对于资源根目录的 Swagger UI 静态资源目录。
	SwaggerDirPath = "swagger"
	// SwaggerIndexPath 是相对于资源根目录的 Swagger UI HTML 入口路径。
	SwaggerIndexPath = "swagger/index.html"
	// SwaggerCSSPath 是相对于资源根目录的 Swagger UI 样式文件路径。
	SwaggerCSSPath = "swagger/swagger-ui.css"
	// SwaggerBundlePath 是相对于资源根目录的 Swagger UI 浏览器脚本路径。
	SwaggerBundlePath = "swagger/swagger-ui-bundle.js"
)
