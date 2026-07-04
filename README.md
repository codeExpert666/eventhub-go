# EventHub Go

EventHub Go 是 Java 版 EventHub 的 Go port，用 Go 生态自然写法复刻 Java 版的业务语义、API 契约、错误码、数据库模型、测试策略和文档沉淀方式。

Java 版参考项目：

```text
/Users/xinnz/Library/Mobile Documents/com~apple~CloudDocs/Code/Java/eventhub
```

## 当前能力

- HTTP foundation：应用入口、router、server、requestId、recover、统一响应、错误码、分页、system ping/echo/health/info。
- OpenAPI / Swagger：spec-first `api/openapi/eventhub.yaml`，dev/test 默认开启，prod 默认关闭。
- Auth / user 基础能力：注册、登录、refresh、logout、当前用户、管理员用户查询与状态更新。
- 持久化底座：MySQL migration、sqlc、repository/mysql、Testcontainers MySQL 集成测试。
- 本地工程闭环：Dockerfile、Docker Compose、Makefile 质量门禁、golangci-lint 配置与 Docker fallback。

## 前置条件

- Go 1.24+
- Docker / Docker Compose
- Node.js / npx（仅运行 `make openapi-lint` 时需要；CI 会自动安装 Node）
- 可选：`golangci-lint` 本机安装

本机安装固定版本 golangci-lint：

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
```

不安装也可以运行：

```bash
make lint
```

当本机找不到 `golangci-lint`，或本机版本不是固定版本 `v2.12.2` 时，Makefile 会使用固定版本 Docker 镜像 `golangci/golangci-lint:v2.12.2` 执行 lint。`.golangci.yml` 使用 v2 配置格式，因此本机安装时也应使用上述固定版本。

## 完整 Docker Compose 启动

```bash
make compose-up
```

该命令会启动：

- `mysql`: MySQL 8.4
- `redis`: Redis 7.2
- `migrate`: 使用 golang-migrate 执行 `migrations/`
- `app`: Go 后端应用容器

应用容器会等待 MySQL healthy、Redis healthy、migration 成功完成后启动，并使用 `GET /actuator/health` 做 healthcheck。
`make compose-up` 会先移除上一次已完成或失败的 `migrate` 容器，再启动完整 Compose 栈，确保 migration job 从新容器执行。
如果直接运行 `docker compose up`，Compose 复用已退出的 `migrate` 容器时也会重新启动该容器并执行 `migrate up`，且 bind mount 能看到新增的 `migrations/` 文件；项目仍推荐 Makefile 入口，以减少旧容器状态、旧 command/env/image 配置和排障歧义。

验证：

```bash
curl http://localhost:8080/actuator/health
```

清理容器：

```bash
make compose-down
```

Compose 中 app 使用 prod-like 本地演示配置，`OPENAPI_ENABLED=false`，因此默认不会暴露 Swagger / OpenAPI。Compose 示例密码和 token secret 只适合本地演示，不能用于生产。

## 本机开发启动

只启动依赖：

```bash
docker compose up -d mysql redis
```

执行 migration：

```bash
make migrate-up
```

加载 dev 环境变量并启动应用：

```bash
set -a
source configs/dev.env.example
set +a
go run ./cmd/eventhub
```

常用地址：

- Health: `http://localhost:8080/actuator/health`
- Info: `http://localhost:8080/actuator/info`
- System ping: `http://localhost:8080/api/v1/system/ping`
- OpenAPI YAML（dev/test 默认开启）: `http://localhost:8080/openapi.yaml`
- Swagger UI（dev/test 默认开启）: `http://localhost:8080/swagger/`

回滚最近 1 个 migration 版本：

```bash
make migrate-down
```

如需指定数据库或回滚步数：

```bash
MIGRATE_DATABASE_URL='mysql://eventhub:eventhub@tcp(localhost:3306)/eventhub?multiStatements=true' make migrate-up
MIGRATE_STEPS=2 make migrate-down
```

## 质量门禁

常用命令：

```bash
make fmt
make fmt-check
make vet
make test
make test-race
make lint
make quality
make quality-check
```

`make quality` 串联：

```text
fmt -> vet -> test -> lint
```

`make quality-check` 串联：

```text
fmt-check -> vet -> test -> lint
```

其中 `*-check` 目标会先执行对应格式化或生成命令，再用 `git diff --exit-code` 暴露未提交漂移，供本地验证和 CI 复用。

SQL、OpenAPI 和容器相关命令：

```bash
make sqlc
make sqlc-check
make openapi-lint
make openapi-validate
make openapi-generate
make openapi-check
make openapi-breaking-check
make generated-check
make docker-build
make compose-up
make compose-down
```

OpenAPI 相关门禁分工：

- `make openapi-lint`：使用固定版本 Redocly CLI 检查通用 OpenAPI 文档质量，例如 operationId、tags、summary、schema 示例和未使用组件提示。
- `make openapi-validate`：使用 kin-openapi 检查 OpenAPI 结构、引用和 schema 合法性。
- `make openapi-check`：串联 validate、oapi-codegen generate 和 generated file diff，确认契约和生成代码没有漂移。
- `make openapi-breaking-check`：使用固定版本 oasdiff 比较 base ref 与当前工作区的 `api/openapi/eventhub.yaml`，阻断 `/api/v1/**` breaking changes；本地默认 base ref 是 `origin/main`，缺失时先执行 `git fetch origin main`。
- `go test ./...`：包含项目自定义 OpenAPI policy test，检查统一响应 envelope、错误响应集中引用、RBAC 文档元数据、router/spec 对齐和真实响应契约。

## 环境配置

示例文件：

- `configs/dev.env.example`
- `configs/test.env.example`
- `configs/prod.env.example`

关键约束：

- `EVENTHUB_ENV=dev/test` 时，`OPENAPI_ENABLED` 默认开启。
- `EVENTHUB_ENV=prod` 时，`OPENAPI_ENABLED` 默认关闭。
- `OPENAPI_ASSET_ROOT` 指向 OpenAPI YAML 与 Swagger UI 本地静态资源根目录；本地默认填相对路径 `api/openapi`，这适用于从仓库根目录启动（相对路径按进程当前工作目录解析），容器默认填绝对路径 `/app/api/openapi`。
- `OPENAPI_ENABLED=true` 时会在启动期校验 `OPENAPI_ASSET_ROOT` 下的 `eventhub.yaml` 和 Swagger UI HTML/CSS/JS 是否存在；资源缺失会启动失败，禁用时不校验也不注册文档路由。
- Dockerfile runtime 默认 `EVENTHUB_ENV=prod`、`OPENAPI_ENABLED=false` 且 `OPENAPI_ASSET_ROOT=/app/api/openapi`。
- 生产环境必须显式注入 `EVENTHUB_ACCESS_TOKEN_SIGNING_SECRET` 和真实数据库/Redis 凭据。

## 目录结构

```text
cmd/eventhub/              可执行入口，保持极薄
internal/app/              应用装配与生命周期
internal/config/           环境变量配置、profile 和对外配置结构
internal/http/             router、server、middleware、handler、dto、response、validation
internal/service/          业务 service
internal/repository/       repository interface
internal/repository/mysql/ sqlc 包装与 MySQL repository 实现
internal/platform/         db、redis、clock、idgen、log 等基础设施
internal/security/         JWT、refresh token、principal、password
api/openapi/               OpenAPI 契约与生成代码
migrations/                golang-migrate SQL migration
configs/                   环境变量示例
docs/ai/                   设计、实现说明、ADR、Java-Go parity matrix
```

## 文档纪律

非微小修改必须先更新 `docs/ai/design/`，实现后更新 `docs/ai/implementation/`，关键取舍写入 `docs/ai/adr/`，语义、契约、模型、测试或 Go-only 结构差异写入 `docs/ai/parity/java-go-parity-matrix.md`。
