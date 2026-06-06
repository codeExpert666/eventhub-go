OAPI_CODEGEN_VERSION ?= v2.5.0
KIN_OPENAPI_VERSION ?= v0.131.0
OPENAPI_SPEC := api/openapi/eventhub.yaml
OPENAPI_GEN := api/openapi/gen/eventhub.gen.go

.PHONY: fmt sqlc test vet openapi-validate openapi-generate openapi-check

fmt:
	gofmt -w .

sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate

test:
	go test ./...

vet:
	go vet ./...

openapi-validate:
	go run github.com/getkin/kin-openapi/cmd/validate@$(KIN_OPENAPI_VERSION) $(OPENAPI_SPEC)

openapi-generate:
	mkdir -p api/openapi/gen
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -generate types,chi-server -package gen -o $(OPENAPI_GEN) $(OPENAPI_SPEC)

openapi-check: openapi-validate openapi-generate
	git diff --exit-code $(OPENAPI_GEN)
