package http_test

import (
	"context"
	"net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"

	openapispec "eventhub-go/api/openapi"
)

func TestRouterContractRoutesMatchOpenAPISpec(t *testing.T) {
	router, _ := testAuthRouter(t)

	routerRoutes := collectRouterContractRoutes(t, router)
	specRoutes := collectOpenAPIContractRoutes(t)

	routerOnly := diffContractRoutes(routerRoutes, specRoutes)
	specOnly := diffContractRoutes(specRoutes, routerRoutes)
	if len(routerOnly) > 0 || len(specOnly) > 0 {
		t.Fatalf(
			"runtime router and OpenAPI paths/methods differ\nrouter 有但 spec 没有:\n%s\nspec 有但 router 没有:\n%s",
			formatContractRouteDiff(routerOnly),
			formatContractRouteDiff(specOnly),
		)
	}
}

type contractRoute struct {
	method string
	path   string
}

func (route contractRoute) String() string {
	return route.method + " " + route.path
}

type contractRouteSet map[contractRoute]struct{}

func collectRouterContractRoutes(t *testing.T, handler http.Handler) contractRouteSet {
	t.Helper()

	routes, ok := handler.(chi.Routes)
	if !ok {
		t.Fatalf("runtime router must implement chi.Routes, got %T", handler)
	}

	result := contractRouteSet{}
	err := chi.Walk(routes, func(method string, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if route, ok := newContractRoute(method, path); ok {
			result[route] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk runtime router routes: %v", err)
	}
	return result
}

func collectOpenAPIContractRoutes(t *testing.T) contractRouteSet {
	t.Helper()

	doc := loadRouterContractOpenAPIDocument(t)
	result := contractRouteSet{}
	for routePath, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}
		for method := range pathItem.Operations() {
			if route, ok := newContractRoute(method, routePath); ok {
				result[route] = struct{}{}
			}
		}
	}
	return result
}

func loadRouterContractOpenAPIDocument(t *testing.T) *openapi3.T {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate OpenAPI router contract test file")
	}
	specPath := filepath.Join(
		filepath.Dir(currentFile),
		"..",
		"..",
		filepath.FromSlash(openapispec.AssetRoot),
		filepath.FromSlash(openapispec.SpecPath),
	)

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load %s: %v", specPath, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate %s: %v", specPath, err)
	}
	return doc
}

func newContractRoute(method, routePath string) (contractRoute, bool) {
	method = strings.ToUpper(strings.TrimSpace(method))
	routePath = strings.TrimSpace(routePath)
	if method == "" || routePath == "" || method == http.MethodOptions {
		return contractRoute{}, false
	}
	if isDocumentationRoute(routePath) || !isContractPath(routePath) {
		return contractRoute{}, false
	}
	return contractRoute{method: method, path: normalizeContractPath(routePath)}, true
}

func isContractPath(routePath string) bool {
	return strings.HasPrefix(routePath, "/api/") || strings.HasPrefix(routePath, "/actuator/")
}

func isDocumentationRoute(routePath string) bool {
	return routePath == "/openapi.yaml" ||
		routePath == "/swagger" ||
		strings.HasPrefix(routePath, "/swagger/")
}

func normalizeContractPath(routePath string) string {
	segments := strings.Split(routePath, "/")
	for i, segment := range segments {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			continue
		}

		param := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
		if name, _, ok := strings.Cut(param, ":"); ok {
			segments[i] = "{" + name + "}"
		}
	}
	return strings.Join(segments, "/")
}

func diffContractRoutes(left, right contractRouteSet) []contractRoute {
	var diff []contractRoute
	for route := range left {
		if _, ok := right[route]; !ok {
			diff = append(diff, route)
		}
	}
	sortContractRoutes(diff)
	return diff
}

func sortContractRoutes(routes []contractRoute) {
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].path != routes[j].path {
			return routes[i].path < routes[j].path
		}
		return routes[i].method < routes[j].method
	})
}

func formatContractRouteDiff(routes []contractRoute) string {
	if len(routes) == 0 {
		return "  (none)"
	}

	var builder strings.Builder
	for _, route := range routes {
		builder.WriteString("  - ")
		builder.WriteString(route.String())
		builder.WriteByte('\n')
	}
	return strings.TrimRight(builder.String(), "\n")
}
