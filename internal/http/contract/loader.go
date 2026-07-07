// Package contract contains the OpenAPI runtime request contract gate.
package contract

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Spec is the startup-loaded OpenAPI contract used by later request validation wiring.
type Spec struct {
	Path     string
	Document *openapi3.T
}

// LoadSpec loads, resolves, and validates an OpenAPI document from the file system.
func LoadSpec(specPath string) (*Spec, error) {
	path := strings.TrimSpace(specPath)
	if path == "" {
		return nil, errors.New("openapi spec path is empty")
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load openapi spec %q from filesystem: %w", path, err)
	}
	location := &url.URL{Path: filepath.ToSlash(path)}
	if err := loader.ResolveRefsIn(doc, location); err != nil {
		return nil, fmt.Errorf("resolve openapi spec refs %q: %w", path, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate openapi spec %q: %w", path, err)
	}

	return &Spec{
		Path:     path,
		Document: doc,
	}, nil
}
