# OpenAPI 校验与代码生成工具。
OAPI_CODEGEN_VERSION ?= v2.5.0
KIN_OPENAPI_VERSION ?= v0.131.0
REDOCLY_CLI_VERSION ?= 2.35.1
OASDIFF_VERSION ?= v1.21.0
OPENAPI_SPEC := api/openapi/eventhub.yaml
OAPI_CODEGEN_CONFIG := api/openapi/oapi-codegen.yaml
OPENAPI_LINT_CONFIG := redocly.yaml
OPENAPI_GEN := api/openapi/gen/eventhub.gen.go
OPENAPI_BASE_REF ?= origin/main
OPENAPI_BREAKING_MATCH_PATH ?= ^/api/v1($$|/)

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

.PHONY: fmt fmt-check vet test test-race lint quality quality-check sqlc sqlc-check migrate-up migrate-down openapi-lint openapi-validate openapi-generate openapi-check openapi-breaking-check generated-check docker-build compose-up compose-down

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

openapi-breaking-check:
	@# 输出流处理原则：只丢弃该 Git 命令在当前检查场景中会产生、且会干扰 Make 输出的那一路。
	@# rev-parse 只检查 base ref 是否能解析为 commit；成功时会把 commit SHA 写到 stdout。
	@# 因此这里丢弃 stdout，避免正常通过时打印无关 SHA；失败噪声已由 --quiet 抑制，不必再加 2>/dev/null。
	@if ! git rev-parse --verify --quiet "$(OPENAPI_BASE_REF)^{commit}" >/dev/null; then \
		echo "OpenAPI breaking check requires base ref '$(OPENAPI_BASE_REF)'."; \
		echo "Fetch it first, for example: git fetch origin main"; \
		exit 2; \
	fi
	@# cat-file -e 只检查 base ref 中是否存在 OpenAPI spec blob；成功时本身不输出内容。
	@# 因此不用额外丢弃 stdout；失败时 Git 会向 stderr 打印底层 fatal 信息，这里丢弃 stderr，改用下面的业务化提示。
	@if ! git cat-file -e "$(OPENAPI_BASE_REF):$(OPENAPI_SPEC)" 2>/dev/null; then \
		echo "OpenAPI breaking check could not find $(OPENAPI_SPEC) in base ref '$(OPENAPI_BASE_REF)'."; \
		echo "Verify the base branch contains the OpenAPI spec, then rerun this target."; \
		exit 2; \
	fi
	@# oasdiff breaking 的参数顺序是 base 在前、revision 在后；这里用 base branch 的 spec 作为旧契约，当前工作区 spec 作为新契约。
	@# "$(OPENAPI_BASE_REF):$(OPENAPI_SPEC)" 是 Git object path，可直接读取 base ref 中的历史 YAML；"$(OPENAPI_SPEC)" 是当前工作区文件。
	@# --fail-on ERR 只在 oasdiff 判定存在 error 级 breaking change 时失败，warning / informational diff 不阻断 Make target。
	@# --match-path 将检查范围收敛到 /api/v1，避免 actuator、Swagger 文档路由或未来其他版本路径影响 v1 兼容性门禁。
	go run github.com/oasdiff/oasdiff@$(OASDIFF_VERSION) breaking "$(OPENAPI_BASE_REF):$(OPENAPI_SPEC)" "$(OPENAPI_SPEC)" --fail-on ERR --match-path '$(OPENAPI_BREAKING_MATCH_PATH)'

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
