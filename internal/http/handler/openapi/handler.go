// Package openapi provides cross-cutting HTTP handlers for API documentation.
package openapi

import (
	"net/http"

	openapispec "eventhub-go/api/openapi"
)

const swaggerUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>EventHub Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css">
  <style>
    html, body, #swagger-ui {
      margin: 0;
      min-height: 100%;
      background: #ffffff;
    }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
        presets: [SwaggerUIBundle.presets.apis],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>
`

// OpenAPIHandler serves the spec-first OpenAPI document and Swagger UI page.
type OpenAPIHandler struct {
	specYAML []byte
}

// NewOpenAPIHandler creates a documentation handler backed by the embedded OpenAPI contract.
func NewOpenAPIHandler() *OpenAPIHandler {
	return &OpenAPIHandler{specYAML: openapispec.SpecYAML()}
}

// YAML writes the OpenAPI YAML contract.
func (h *OpenAPIHandler) YAML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.specYAML)
}

// RedirectSwagger redirects the short Swagger path to the index path.
func (h *OpenAPIHandler) RedirectSwagger(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
}

// SwaggerUI writes a minimal Swagger UI page that loads /openapi.yaml.
func (h *OpenAPIHandler) SwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(swaggerUIHTML))
}
