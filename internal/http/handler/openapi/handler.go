// Package openapi 提供 OpenAPI 契约与 Swagger UI 文档入口的 HTTP handler。
package openapi

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	openapispec "eventhub-go/api/openapi"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/response"
)

// staticAsset 描述一个允许通过文档入口访问的本地静态资源。
type staticAsset struct {
	relativePath string
	contentType  string
}

// staticAssets 定义文档入口公开 URL 与本地静态资源之间的白名单映射。
var staticAssets = map[string]staticAsset{
	"/openapi.yaml": {
		relativePath: openapispec.SpecPath,
		contentType:  "application/yaml; charset=utf-8",
	},
	"/swagger/": {
		relativePath: openapispec.SwaggerIndexPath,
		contentType:  "text/html; charset=utf-8",
	},
	"/swagger/index.html": {
		relativePath: openapispec.SwaggerIndexPath,
		contentType:  "text/html; charset=utf-8",
	},
	"/swagger/swagger-ui.css": {
		relativePath: openapispec.SwaggerCSSPath,
		contentType:  "text/css; charset=utf-8",
	},
	"/swagger/swagger-ui-bundle.js": {
		relativePath: openapispec.SwaggerBundlePath,
		contentType:  "application/javascript; charset=utf-8",
	},
}

// requiredStaticAssetPaths 列出启用文档入口时必须存在的本地资源。
var requiredStaticAssetPaths = []string{
	openapispec.SpecPath,
	openapispec.SwaggerIndexPath,
	openapispec.SwaggerCSSPath,
	openapispec.SwaggerBundlePath,
}

// OpenAPIHandler 负责返回本地 OpenAPI 契约和 Swagger UI 静态资源。
type OpenAPIHandler struct {
	// assetRoot 指向配置传入的 OpenAPI 静态资源根目录。
	assetRoot string
}

// NewOpenAPIHandler 使用配置传入的静态资源根目录创建文档 handler。
func NewOpenAPIHandler(assetRoot string) (*OpenAPIHandler, error) {
	handler := &OpenAPIHandler{assetRoot: assetRoot}
	if err := handler.validateStaticAssets(); err != nil {
		return nil, err
	}
	return handler, nil
}

// YAML 返回本地 OpenAPI YAML 契约文件。
func (h *OpenAPIHandler) YAML(w http.ResponseWriter, r *http.Request) {
	h.serveStaticAsset(w, r)
}

// RedirectSwagger 将短路径 /swagger 重定向到 Swagger UI 入口路径。
func (h *OpenAPIHandler) RedirectSwagger(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
}

// SwaggerUI 返回本地 Swagger UI HTML 入口文件。
func (h *OpenAPIHandler) SwaggerUI(w http.ResponseWriter, r *http.Request) {
	h.serveStaticAsset(w, r)
}

// SwaggerAsset 返回白名单内的 Swagger UI 浏览器静态资源。
func (h *OpenAPIHandler) SwaggerAsset(w http.ResponseWriter, r *http.Request) {
	h.serveStaticAsset(w, r)
}

// serveStaticAsset 从资源白名单中选择本地文件，并把缺失资源统一映射为 COMMON-404。
func (h *OpenAPIHandler) serveStaticAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := staticAssets[r.URL.Path]
	if !ok {
		writeNotFound(w, r)
		return
	}
	filePath, ok := h.assetPath(asset.relativePath)
	if !ok {
		writeNotFound(w, r)
		return
	}
	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeNotFound(w, r)
			return
		}
		response.WriteError(w, r, apperror.New(apperror.CommonInternal, "系统内部错误"))
		return
	}
	if info.IsDir() {
		writeNotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", asset.contentType)
	http.ServeFile(w, r, filePath)
}

// validateStaticAssets 在启动期确认启用文档入口所需的静态资源都已随部署产物存在。
func (h *OpenAPIHandler) validateStaticAssets() error {
	if strings.TrimSpace(h.assetRoot) == "" {
		return errors.New("openapi asset root is empty")
	}
	for _, relativePath := range requiredStaticAssetPaths {
		filePath, ok := h.assetPath(relativePath)
		if !ok {
			return fmt.Errorf("openapi asset root %q has invalid asset path %q", h.assetRoot, relativePath)
		}
		info, err := os.Stat(filePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("openapi asset root %q missing required asset %q: %w", h.assetRoot, relativePath, err)
			}
			return fmt.Errorf("openapi asset root %q cannot read required asset %q: %w", h.assetRoot, relativePath, err)
		}
		if info.IsDir() {
			return fmt.Errorf("openapi asset root %q required asset %q is a directory", h.assetRoot, relativePath)
		}
	}
	return nil
}

// assetPath 清洗相对路径，防止通过绝对路径或 .. 访问资源根目录之外的文件。
func (h *OpenAPIHandler) assetPath(relativePath string) (string, bool) {
	clean := filepath.Clean(relativePath)
	if clean == "." || clean == ".." || filepath.IsAbs(clean) ||
		strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return filepath.Join(h.assetRoot, clean), true
}

// writeNotFound 使用项目统一响应格式返回文档资源不存在。
func writeNotFound(w http.ResponseWriter, r *http.Request) {
	response.WriteError(w, r, apperror.New(apperror.CommonNotFound, "请求的资源不存在"))
}
