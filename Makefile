OAPI_CODEGEN_VERSION ?= v2.5.0
KIN_OPENAPI_VERSION ?= v0.131.0
SQLC_VERSION ?= v1.30.0
MIGRATE_VERSION ?= v4.19.0
GOLANGCI_LINT_VERSION ?= v2.12.2
GOLANGCI_LINT_EXPECTED_VERSION := $(patsubst v%,%,$(GOLANGCI_LINT_VERSION))

OPENAPI_SPEC := api/openapi/eventhub.yaml
OPENAPI_GEN := api/openapi/gen/eventhub.gen.go
DOCKER_IMAGE ?= eventhub-go:local
GOLANGCI_LINT_IMAGE ?= golangci/golangci-lint:$(GOLANGCI_LINT_VERSION)
MIGRATE_DATABASE_URL ?= mysql://eventhub:eventhub@tcp(localhost:3306)/eventhub?multiStatements=true
MIGRATE_STEPS ?= 1

.PHONY: fmt vet test test-race lint quality sqlc migrate-up migrate-down openapi-validate openapi-generate openapi-check docker-build compose-up compose-down

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		local_version=$$(golangci-lint version 2>/dev/null || true); \
		if printf '%s\n' "$$local_version" | grep -q "version $(GOLANGCI_LINT_EXPECTED_VERSION) "; then \
			golangci-lint run ./...; \
		else \
			echo "golangci-lint $(GOLANGCI_LINT_VERSION) is required; found: $${local_version:-unknown}; falling back to Docker image $(GOLANGCI_LINT_IMAGE)."; \
			docker run --rm -v "$(CURDIR):/app" -w /app $(GOLANGCI_LINT_IMAGE) golangci-lint run ./...; \
		fi; \
	else \
		docker run --rm -v "$(CURDIR):/app" -w /app $(GOLANGCI_LINT_IMAGE) golangci-lint run ./...; \
	fi

quality: fmt vet test lint

sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

migrate-up:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION) -path migrations -database "$(MIGRATE_DATABASE_URL)" up

migrate-down:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION) -path migrations -database "$(MIGRATE_DATABASE_URL)" down $(MIGRATE_STEPS)

openapi-validate:
	go run github.com/getkin/kin-openapi/cmd/validate@$(KIN_OPENAPI_VERSION) $(OPENAPI_SPEC)

openapi-generate:
	mkdir -p api/openapi/gen
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -generate types,chi-server -package gen -o $(OPENAPI_GEN) $(OPENAPI_SPEC)

openapi-check: openapi-validate openapi-generate
	git diff --exit-code $(OPENAPI_GEN)

docker-build:
	docker build -t $(DOCKER_IMAGE) .

compose-up:
	docker compose rm -sf migrate
	docker compose up --build

compose-down:
	docker compose down
