// Package openapi exposes the spec-first EventHub OpenAPI contract.
package openapi

import _ "embed"

// eventhubYAML is the single source of truth served by /openapi.yaml.
//
//go:embed eventhub.yaml
var eventhubYAML []byte

// SpecYAML returns a copy of the embedded OpenAPI YAML contract.
func SpecYAML() []byte {
	copied := make([]byte, len(eventhubYAML))
	copy(copied, eventhubYAML)
	return copied
}
