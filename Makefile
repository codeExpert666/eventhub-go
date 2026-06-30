# OpenAPI 校验与代码生成工具。
OAPI_CODEGEN_VERSION ?= v2.5.0
KIN_OPENAPI_VERSION ?= v0.131.0
REDOCLY_CLI_VERSION ?= 2.35.1
OPENAPI_SPEC := api/openapi/eventhub.yaml
OAPI_CODEGEN_CONFIG := api/openapi/oapi-codegen.yaml
OPENAPI_LINT_CONFIG := redocly.yaml
OPENAPI_GEN := api/openapi/gen/eventhub.gen.go

# 数据库代码生成与 migration 工具。
SQLC_VERSION ?= v1.30.0
MIGRATE_VERSION ?= v4.19.0
MIGRATE_DATABASE_URL ?= mysql://eventhub:eventhub@tcp(localhost:3306)/eventhub?multiStatements=true
# 默认只回滚最近一个 migration；需要多步回滚时显式传 MIGRATE_STEPS。
MIGRATE_STEPS ?= 1

# golangci-lint 配置使用 v2 schema；固定 runner 版本可减少不同机器的 lint 结果漂移。
GOLANGCI_LINT_VERSION ?= v2.12.2
# `golangci-lint version` 输出不带前缀 v，去掉前缀后用于匹配本机工具版本。
GOLANGCI_LINT_EXPECTED_VERSION := $(patsubst v%,%,$(GOLANGCI_LINT_VERSION))
# Docker fallback 与本机固定版本保持一致。
GOLANGCI_LINT_IMAGE ?= golangci/golangci-lint:$(GOLANGCI_LINT_VERSION)

# 应用镜像构建目标。
DOCKER_IMAGE ?= eventhub-go:local

.PHONY: fmt fmt-check vet test test-race lint quality quality-check sqlc sqlc-check migrate-up migrate-down openapi-lint openapi-validate openapi-generate openapi-check generated-check docker-build compose-up compose-down

fmt:
	gofmt -w .

# check 目标用 $(MAKE) 调已有 target，这种子 make 可以继承调用时父 make 的参数和变量。
fmt-check:
	$(MAKE) fmt
	git diff --exit-code -- '*.go'

vet:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

# lint 高频且较重，优先复用版本匹配的本机工具；缺失或版本不一致时用固定 Docker 镜像兜底。
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

# 日常质量门禁按“格式化 -> 基础静态检查 -> 测试 -> lint”执行，先暴露低成本问题。
# race detector 成本更高，保留为显式 test-race 专项检查。
quality: fmt vet test lint

# 用 recipe 中的 $(MAKE) 明确串行执行顺序，避免并行 make 打乱检查先后顺序。
quality-check:
	$(MAKE) fmt-check
	$(MAKE) vet
	$(MAKE) test
	$(MAKE) lint

# 下面的生成/维护目标频率较低，用 go run module@version 固定版本，避免要求本机预装多个 CLI。
sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

sqlc-check:
	$(MAKE) sqlc
	git diff --exit-code internal/repository/mysql/sqlc

migrate-up:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION) -path migrations -database "$(MIGRATE_DATABASE_URL)" up

# `down $(MIGRATE_STEPS)` 按步数回滚，例：make migrate-down MIGRATE_STEPS=2。
migrate-down:
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION) -path migrations -database "$(MIGRATE_DATABASE_URL)" down $(MIGRATE_STEPS)

openapi-lint:
	npx --yes @redocly/cli@$(REDOCLY_CLI_VERSION) lint $(OPENAPI_SPEC) --config $(OPENAPI_LINT_CONFIG)

openapi-validate:
	go run github.com/getkin/kin-openapi/cmd/validate@$(KIN_OPENAPI_VERSION) $(OPENAPI_SPEC)

openapi-generate:
	mkdir -p api/openapi/gen
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -config $(OAPI_CODEGEN_CONFIG) $(OPENAPI_SPEC)

openapi-check:
	$(MAKE) openapi-validate
	$(MAKE) openapi-generate
	git diff --exit-code $(OPENAPI_GEN)

generated-check:
	$(MAKE) sqlc-check
	$(MAKE) openapi-check

# 构建本地应用镜像；默认标签 eventhub-go:local，可用 DOCKER_IMAGE 覆盖。
docker-build:
	docker build -t $(DOCKER_IMAGE) .

# migration 是一次性容器，先移除旧容器再启动，确保每次 up 都会重新跑迁移检查。
compose-up:
	docker compose rm -sf migrate
	docker compose up --build

# 停止并移除 Compose 容器和网络；默认保留 MySQL / Redis 命名卷数据。
compose-down:
	docker compose down
