# Docker 与本地开发工作流实现说明

## 1. 本次改动解决了什么问题

本次补齐 Go 版 EventHub 的容器化、本地开发命令和质量门禁闭环：

- 新增多阶段 Dockerfile，最终运行镜像不携带 Go 编译工具链。
- 新增 Docker Compose，能启动 MySQL 8.4、Redis 7.2、migration job 和 Go app。
- Compose app 使用 `/actuator/health` 做 healthcheck。
- Makefile 补齐 fmt、vet、test、test-race、lint、quality、sqlc、migration、OpenAPI、Docker 和 Compose 目标。
- golangci-lint 固定版本并提供本机未安装时的 Docker fallback。
- dev/test/prod env example 补齐 Redis 配置，并保持 prod 默认关闭 OpenAPI/Swagger。
- README 更新本地开发、Compose、migration、lint 和质量门禁说明。
- `.dockerignore` 进一步补齐中文分类注释和本地/敏感/可再生产物忽略规则，让 Docker build context 更稳定、更小，也更不容易误带本机状态。

## 2. 改动内容
- 新增了什么
  - `Dockerfile`：Go build stage + Alpine runtime stage。
  - `.dockerignore`：减少 Docker build context；后续补充中文分类注释和更完整的本地文件过滤规则。
  - `docker-compose.yml`：编排 `mysql`、`redis`、`migrate`、`app`。
  - `docs/ai/design/016-docker-and-dev-workflow.md`。
  - `docs/ai/implementation/016-docker-and-dev-workflow.md`。
  - `docs/ai/adr/0020-docker-runtime-image.md`。
  - `docs/ai/adr/0021-migration-execution-policy.md`。
  - `docs/ai/adr/0022-golangci-lint-quality-gate.md`。
- 修改了什么
  - `.dockerignore`
    - 按 Git/CI、Docker 元数据、OS/IDE、本地 AI/索引状态、文档、本地环境/密钥、构建测试产物、日志八类组织规则。
    - 新增 `.github/`、`.dockerignore`、`Dockerfile`、`docker-compose*.yml`、`docker-compose*.yaml`、`.gitattributes`、本地 swap 文件、`.env*`、`*.env`、密钥/证书、`bin/`、`build/`、`out/`、coverage/test/profile 产物和 `logs/` 忽略规则。
    - 使用 `!*.env.example` 和 `!configs/*.env.example` 保留配置示例，避免把真实本地环境文件和示例配置混为一谈。
    - 刻意不忽略 `api/`、`migrations/`、`configs/*.env.example`，因为 OpenAPI YAML 通过 Go embed 编入二进制，migration 和配置示例仍属于项目运行契约。
  - `Makefile`
    - 新增固定版本变量：`SQLC_VERSION`、`MIGRATE_VERSION`、`GOLANGCI_LINT_VERSION`。
    - 新增 `test-race`、`lint`、`quality`、`migrate-up`、`migrate-down`、`docker-build`、`compose-up`、`compose-down`。
    - `lint` 优先使用本机 `golangci-lint`，缺失时使用 `golangci/golangci-lint:v1.64.8`。
    - `compose-up` 在启动完整栈前先执行 `docker compose rm -sf migrate`，确保一次性 migration job 从新容器执行；已退出旧容器被启动时也会重跑 command，但会沿用旧容器创建时保存的 command/env/image 配置。
  - `.golangci.yml`
    - 固定低噪音规则：`gofmt`、`govet`、`ineffassign`、`staticcheck`、`unused`。
    - 使用 `disable-all: true` 避免默认规则漂移。
  - `internal/config`
    - 新增 Redis 环境变量解析：地址、用户名、密码、DB、连接/读/写超时。
  - `internal/app/providers/platform.go`
    - 配置 `EVENTHUB_REDIS_ADDR` 时创建 Redis client 并在启动期 ping。
  - `internal/app/application.go` / `internal/app/bootstrap.go`
    - 应用生命周期纳入 Redis client 关闭。
    - bootstrap 后续装配失败时清理已打开的 MySQL / Redis。
  - `configs/*.env.example`
    - 补齐 Redis 配置示例。
    - prod 继续 `OPENAPI_ENABLED=false`。
  - `README.md`
    - 更新完整本地开发、Compose、migration、lint fallback 和质量门禁说明。
  - `internal/service/user/admin_users_test.go`
    - 调整一处 composite literal 写法，让固定 golangci-lint 镜像内的 gofmt 检查通过；测试语义不变。
  - `docs/ai/parity/java-go-parity-matrix.md`
    - 更新质量门禁、Redis 边界和容器化部署配置行。
- 删除了什么
  - 未删除生产代码。
  - 旧 README 内容被重写，因为它仍描述 Docker/OpenAPI/auth 等能力“待迁移”，已不符合当前仓库状态。
- 是否更新 Java-Go parity 记录
  - 已更新。
  - 本次触发容器化、dev/test/prod 配置、migration 执行策略、Redis 启动依赖、质量门禁和 Go-only 工程取舍的 parity 更新。
  - 本次 `.dockerignore` 细化未再次修改 parity matrix；它不改变 API、错误码、数据库模型、认证边界或业务流程，现有“容器化、部署配置与质量门禁”记录已覆盖 Docker 构建上下文管理。
  - 本次 Compose migration job 文档语义复核未更新 parity matrix；它只澄清 Docker Compose 对已退出一次性容器的启动/复用语义，不改变 API、错误码、数据库模型、migration 文件、repository 行为或 Java-Go 业务对齐状态。

## 3. 为什么这样设计
- 关键设计原因
  - Dockerfile 对齐 Java 多阶段镜像思路，但用 Go build stage + 不含 Go 工具链的 Alpine runtime stage 表达 Go 生态运行方式。
  - Alpine runtime 比 `golang` 镜像攻击面更小，同时保留 `wget` 支持 Compose 直接调用 `/actuator/health`。
  - migration 采用 Compose 一次性 job 和 Makefile 显式命令，不塞进 app 启动，避免后续多副本部署时多个 app 竞争 schema 变更。
  - Compose 中 app 等待 MySQL healthy、Redis healthy 和 migration completed，保证空库本地栈可启动；Makefile `compose-up` 额外移除旧 `migrate` 容器，让 migration job 使用新容器执行，减少一次性 job 状态和旧 command/env/image 配置带来的歧义。
  - golangci-lint 固定版本并提供 Docker fallback，解决本机没安装工具时 lint 不可运行的问题。
  - `.golangci.yml` 只启用低噪音规则，适合当前业务迁移阶段。
  - `.dockerignore` 采用“构建所需文件保留、本地状态和可再生产物排除”的边界，既减少 context 噪音，也避免把 `api/openapi/eventhub.yaml` 这类 Go embed 输入误排除。
- 与 Go 项目当前阶段的匹配点
  - `handler -> service -> repository -> sqlc/database` 没有被破坏。
  - Redis 只在 composition root 启动期装配和 ping，不进入 handler/service 业务语义。
  - `Application` 统一管理进程级资源释放，符合当前 `internal/app` composition root 边界。
  - Makefile 让本地验证、CI 入口和 README 说明统一。
- 与 Java 版业务语义的对齐方式
  - 对齐 Java Compose 的 MySQL 8.4、Redis 7.2 和后端应用本地完整闭环。
  - 对齐 Java Dockerfile 的“构建工具不进入最终运行镜像”目标。
  - 对齐 Java prod OpenAPI hardening：Go Dockerfile runtime 和 prod env example 默认关闭 Swagger/OpenAPI。
  - Go 版不复刻 Flyway 自动启动迁移，而是用 golang-migrate job/Makefile 保留 migration 语义并明确部署职责边界。

## 4. 替代方案
- 方案 A：最终镜像使用 `golang`。
  - 没有采用。它会把 Go 编译器和更多构建期内容带入 runtime。
- 方案 B：最终镜像使用 distroless 或 scratch。
  - 没有采用。当前要求容器内直接 healthcheck `/actuator/health`，Alpine 能更低成本提供 wget；后续可在平台 probe 或应用内 probe 子命令成熟后再评估。
- 方案 C：app 启动自动执行 migration。
  - 没有采用。会混淆部署编排和业务进程职责，也不利于未来多副本启动。
- 方案 D：只在 README 中要求开发者安装 golangci-lint。
  - 没有采用。历史验证已经多次遇到本机未安装导致 lint 未运行。
- 方案 E：一次性开启大量 lint 规则。
  - 没有采用。当前阶段优先保障业务 parity、分层边界和低噪音质量门禁，复杂度/风格规则留到后续逐步引入。
- 方案 F：把 `api/`、`migrations/`、`configs/` 也全部排除。
  - 没有采用。`api/openapi/eventhub.yaml` 是 Go embed 输入，`migrations/` 和配置示例是部署/迁移契约的一部分；当前 Dockerfile 只编译二进制，但过度忽略会让后续 Docker 内校验或 embed 资源更容易缺失。

## 5. 测试与验证
- 跑了哪些测试
  - `make fmt vet test`：通过。
  - `make lint`：通过。
  - `make quality`：通过。
  - `make docker-build`：通过。
  - `make compose-up`：通过，MySQL 8.4、Redis 7.2、migration job 和 app 均启动成功；后续修复补充了启动前移除旧 `migrate` 容器的步骤。
  - `curl http://localhost:8080/actuator/health`：返回 `{"status":"UP"}`。
  - `make compose-down`：通过，容器和网络已清理。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w .`：通过，由 `make fmt` 和 `make quality` 执行。
  - `go vet ./...`：通过。
  - `go test ./...`：通过。
  - `golangci-lint run ./...`：通过；本机未安装，实际使用固定 Docker 镜像 fallback。
  - `sqlc generate`：未运行；本次不修改 SQL query、schema 或 sqlc 配置。
  - `openapi-validate`：未运行；本次不修改 OpenAPI 契约。
- 手工验证了哪些场景
  - Docker build 使用多阶段缓存，最终产物命名为 `eventhub-go:local`。
  - Compose migration 输出：
    - `1/u system_bootstrap`
    - `2/u auth_schema`
  - app 以 `env=prod` 启动，并持续通过 `/actuator/health` healthcheck。
  - `make compose-down` 后前台 `make compose-up` 会话正常退出。
  - review 修复后通过 `make -n compose-up` 确认命令顺序为先移除 `migrate` 容器，再执行 `docker compose up --build`。
  - 本次 `.dockerignore` 细化后运行 `docker build -t eventhub-go:dockerignore-check .`，构建上下文为 8.92kB，`COPY . .` 后 `go build ./cmd/eventhub` 成功。
  - 本次 `.dockerignore` 细化后运行 `go test ./...`：通过。
  - 本次 `.dockerignore` 细化后运行 `go vet ./...`：通过。
  - 本次 `.dockerignore` 细化后运行 `make lint`：通过。
  - 本次 Compose migration job 文档语义复核使用最小 Compose job 验证：已退出的一次性容器被 `docker compose up` 再次启动时会重新执行 command；作为 `service_completed_successfully` 依赖时，再次启动 app 也会重新启动已退出的 job。
  - 本次 Compose migration job 文档语义复核运行 `docker compose config --quiet`：通过。
- Java-Go parity 如何验证
  - 对照 Java `docker-compose.yml` 的 MySQL/Redis/app 编排。
  - 对照 Java `backend/Dockerfile` 的多阶段构建与 runtime 不含构建工具链目标。
  - 对照 Java prod OpenAPI hardening ADR，确认 Go runtime/prod env 默认 `OPENAPI_ENABLED=false`。
  - 对照 Go ADR-0008，确认 migration 仍使用 golang-migrate，而不是回到 Flyway 或应用内手写初始化。
- 结果如何
  - 所有要求的验证命令均通过。
  - `make lint` 首次运行时因固定 Docker 镜像的 gofmt 检查发现一处已有测试格式写法，已调整后重跑通过。

## 6. 已知限制
- 当前版本还缺什么
  - 还没有 CI workflow 自动执行 `make quality`、`make docker-build` 或 Compose smoke test。
  - Compose 示例密码和 token secret 只适合本地演示。
  - Dockerfile runtime 使用 Alpine，不是最小的 distroless/scratch。
  - Redis 只作为启动期依赖和未来缓存底座，不参与业务健康详情或认证强一致。
  - 如果绕过 Makefile 直接执行裸 `docker compose up --build`，Compose 可能复用已经成功退出的一次性 `migrate` 容器；旧容器被启动时会重跑 command，并能通过 bind mount 看到新增 migration 文件，但不会吸收后续修改过的 command/env/image 配置。本地完整启动推荐使用 `make compose-up`。
  - `.dockerignore` 已排除常见本地文件和产物；未来若新增 `go:embed` 静态资源、Dockerfile 内测试/校验步骤，或需要把 README/docs 打包进镜像，必须同步审视 ignore 规则。
- 哪些地方后面需要继续演进
  - 增加 CI 质量门禁和镜像构建流水线。
  - 生产部署文档需要补 migration job、Secret 管理、镜像扫描和回滚策略。
  - 后续如果引入业务缓存、幂等 token 或库存热点缓存，需要单独设计 Redis 数据模型和一致性边界。
  - 如果需要更小运行镜像，可评估 distroless 并替换 healthcheck 方案。
- 与 Java 版仍有哪些差距
  - Java Flyway 可随 Spring Boot 启动自动迁移；Go 版刻意采用显式 migration job/Makefile。
  - Java Actuator 可聚合组件健康详情；Go 当前 health 仍保持最小 `UP` 响应。
  - Java Compose 文档历史上包含 Swagger 访问说明；Go Compose app 使用 prod-like 配置，默认不暴露 Swagger/OpenAPI。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 项目现在具备可展示的 Dockerfile、Compose、migration、healthcheck、Makefile、lint fallback 和质量门禁闭环。
- 对微服务 / 云原生演进的影响
  - migration job 与 app runtime 分离，更容易迁移到 CI/CD、Kubernetes Job 或发布流水线。
  - runtime 镜像默认 prod 安全姿态，给后续部署基线打底。
  - Compose 中显式服务健康依赖有利于后续拆分 Redis、MySQL、app 观测和启动顺序。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 新增 SQL/query 后继续运行 `make sqlc`，必要时补 migration 测试。
  - API 契约变化仍运行 `make openapi-validate` / `make openapi-check`。
  - 常规代码改动优先运行 `make quality`。
  - 容器相关改动运行 `make docker-build` 和 Compose smoke test。
