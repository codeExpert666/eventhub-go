# Docker 与本地开发工作流设计

## 1. 背景
- Go 版 EventHub 已具备 HTTP server、system/actuator、OpenAPI、MySQL migration、sqlc、auth/user 基础能力，但本地开发与容器化工作流仍不完整：
  - 根目录缺少 Go 应用 `Dockerfile`。
  - 缺少可同时启动 app、MySQL、Redis 和 migration 的 Go 版 `docker-compose.yml`。
  - `Makefile` 只有基础 fmt/test/vet/sqlc/openapi 命令，尚未形成完整质量门禁。
  - `.golangci.yml` 已存在，但版本固定、本机未安装 fallback 和 README 指引不足。
- Java 版对应来源：
  - `docker-compose.yml`：使用 MySQL 8.4、Redis 7.2，并默认启动后端应用容器。
  - `backend/Dockerfile`：多阶段构建，最终运行镜像不携带 Maven/JDK 编译工具链。
  - `docs/ai/adr/2026-04-27-stage-0-compose-dev-prod-profile.md`：Docker Compose 默认提供完整基础闭环。
  - `docs/ai/adr/2026-05-23-prod-openapi-hardening.md`：生产环境默认关闭 OpenAPI 与 Swagger UI。
  - Java Flyway 迁移文档与 Go 版 `docs/ai/adr/0008-database-migration-golang-migrate.md`。
- 业务上下文是活动预约与票务平台的工程底座：在继续迁移活动、订单、库存、支付前，需要让本地启动、依赖服务、数据库迁移、质量门禁和容器镜像具备可重复执行的闭环。

## 2. 目标
- 新增 Go 多阶段 `Dockerfile`：
  - build 阶段使用 Go 官方镜像编译 `cmd/eventhub`。
  - runtime 阶段使用不含 Go 编译工具链的轻量运行镜像。
  - 容器默认以 prod 安全姿态运行，`OPENAPI_ENABLED=false`。
- 新增 `docker-compose.yml`：
  - 启动 MySQL 8.4、Redis 7.2、migration job 和 Go app。
  - MySQL / Redis 使用 healthcheck。
  - app 依赖 MySQL healthy、Redis healthy 和 migration 成功完成后再启动。
  - app 使用 `GET /actuator/health` 作为容器 healthcheck。
  - 项目推荐入口必须在每次完整启动前移除旧的 `migrate` 容器，避免新增 migration 后复用已完成的一次性容器。
- 明确 migration 执行策略：
  - Go 版不把 migration 自动塞进应用启动流程。
  - Compose 使用一次性 `migrate` service 执行 `up`。
  - Makefile 提供 `migrate-up` / `migrate-down` 供本地和 CI 显式执行。
- 更新 dev/test/prod env example：
  - dev/test 默认开启 OpenAPI。
  - prod 默认关闭 OpenAPI / Swagger。
  - 补齐 MySQL、Redis、auth token 等本地/容器可用配置示例。
- 完善 `Makefile`：
  - 至少包含 `fmt`、`vet`、`test`、`test-race`、`lint`、`quality`、`sqlc`、`migrate-up`、`migrate-down`、`openapi-validate`、`docker-build`、`compose-up`、`compose-down`。
  - `quality` 串联核心质量门禁。
- 完善 `.golangci.yml`：
  - 选择适合当前 Go 后端阶段的低噪音规则。
  - 固定 golangci-lint 版本。
  - Makefile `lint` 优先使用本机工具，本机未安装时使用固定版本 Docker 镜像 fallback。
- 更新 README 本地开发说明：
  - 本机运行、Compose 完整启动、依赖服务启动、migration、lint 安装/不安装两种路径、质量门禁与 prod Swagger 默认关闭。
- 成功标准：
  - `make fmt vet test`、`make lint`、`make quality` 可运行。
  - `make docker-build` 可构建镜像。
  - `make compose-up` 可启动 MySQL、Redis、migration 和 Go app，并保证每次执行都会重新运行 `migrate up`。
  - `curl http://localhost:8080/actuator/health` 返回健康响应。
  - `make compose-down` 可清理运行容器。

## 3. 非目标
- 不改变业务 API 路径、请求字段、响应字段、错误码或 JWT claim。
- 不新增活动、订单、库存、支付等业务模块。
- 不改变 `handler -> service -> repository -> sqlc/database` 分层。
- 不让 handler 直接访问数据库、Redis 或 sqlc。
- 不把数据库 migration 自动绑定到 Go 应用进程启动；应用仍只负责服务自身启动。
- 不引入 Kubernetes、Helm、CI workflow、镜像签名、SBOM 或生产密钥管理。
- 不默认在 prod 开启 Swagger / OpenAPI；如需临时开启必须显式配置。

## 4. 影响范围
- 涉及文件：
  - `Dockerfile`
  - `.dockerignore`
  - `docker-compose.yml`
  - `Makefile`
  - `.golangci.yml`
  - `configs/dev.env.example`
  - `configs/test.env.example`
  - `configs/prod.env.example`
  - `README.md`
  - `internal/config` 和 `internal/app`，用于补齐 Redis 启动配置和可关闭资源。
  - `docs/ai/design/016-docker-and-dev-workflow.md`
  - `docs/ai/implementation/016-docker-and-dev-workflow.md`
  - `docs/ai/adr/0020-docker-runtime-image.md`
  - `docs/ai/adr/0021-migration-execution-policy.md`
  - `docs/ai/adr/0022-golangci-lint-quality-gate.md`
  - `docs/ai/parity/java-go-parity-matrix.md`
- 涉及 API：
  - 不新增 API。
  - 复用现有 `GET /actuator/health` 作为 Compose app healthcheck。
- 涉及表 / 缓存 / 外部接口：
  - 不新增表、索引、唯一约束或 sqlc query。
  - 复用现有 `migrations/`。
  - Redis 作为本地依赖服务启动并可由应用启动期 ping；本次不把 Redis 纳入业务缓存或认证强一致链路。
- 是否影响 parity matrix：是。容器化、dev/prod profile、migration 执行策略、质量门禁和 Go-only 工程差异需要从“待迁移”更新为已对齐或已决策。

## 5. 领域建模
- `RuntimeImage`
  - Go 应用最终运行镜像。
  - 只包含编译后的 `eventhub` 二进制、CA 证书、时区数据和轻量 healthcheck 工具。
  - 不包含 Go 编译器、源码、module cache 或测试工具链。
- `ComposeStack`
  - 本地完整基础闭环，包含 `mysql`、`redis`、`migrate`、`app`。
  - `mysql` 对应 Java Compose 中的 MySQL 8.4。
  - `redis` 对应 Java Compose 中的 Redis 7.2。
  - `migrate` 是 Go 版对 Java Flyway 启动迁移语义的 Go 生态替代。
  - `app` 是 Go 后端容器。
- `MigrationJob`
  - 一次性容器或 Makefile 命令。
  - 使用 golang-migrate 固定版本执行 `migrations/`。
  - 成功完成后 app 才启动。
- `QualityGate`
  - Makefile 中可重复执行的质量门禁集合。
  - 核心包含 `fmt`、`vet`、`test`、`lint`。
- 与 Java 版领域对象的对应关系：
  - Java Maven build stage -> Go build stage。
  - Java JRE runtime image -> Go Alpine runtime image。
  - Java Flyway 自动迁移 -> Go Compose/Makefile 显式 golang-migrate。
  - Java dev/test/prod profile -> Go `EVENTHUB_ENV=dev/test/prod` 与 env examples。

## 6. API 设计
- 本次不新增或修改业务 API。
- Compose app healthcheck 使用现有接口：
  - `GET /actuator/health`
  - 成功期望：HTTP 200，响应体包含 `{"status":"UP"}`。
  - 失败期望：非 2xx 或请求失败时容器标记为 unhealthy。
- `HEAD /actuator/health` 仍可供轻量探针使用，但 Compose 使用 GET，便于用 `wget` 直接判断 HTTP 状态。
- 错误码 / 异常场景：
  - 健康检查路由不存在或应用未启动时，Docker healthcheck 失败，不新增应用错误码。
  - prod 默认禁用 Swagger/OpenAPI 时，`/openapi.yaml` 和 `/swagger/*` 继续按现有 router 返回 `COMMON-404`。
- 与 Java 版 OpenAPI / controller 契约差异：
  - Java Actuator health 由 Spring Boot Actuator 提供。
  - Go 版由 `internal/http/handler/system` 返回同等最小 health 语义。
  - Go 不迁移 Springdoc `/swagger-ui.html` 路径；仍使用已有 `/swagger/*`，并由 `OPENAPI_ENABLED` 控制是否注册。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：
  - 不新增 migration 文件。
  - Makefile 和 Compose 只负责执行现有 `migrations/`。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - Compose `migrate` service 在 app 前完成 `up`，避免空库启动后 auth/user 请求撞到缺表。
  - 本地 `migrate-down` 默认回退 1 个版本，降低误删全部 schema 的风险；如需更多步数可通过变量覆盖。
  - 生产部署不应依赖 Compose 示例密码或本地 migration 命令直接操作生产库；README 和 prod env example 会标注密钥与 DSN 必须由部署系统注入。

## 8. 关键流程
- Docker build 正常流程：
  1. build stage 复制 `go.mod` / `go.sum` 并下载依赖。
  2. 复制源码。
  3. 使用 `CGO_ENABLED=0` 编译 `./cmd/eventhub`。
  4. runtime stage 只复制二进制。
  5. runtime 容器以非 root 用户运行。
- Compose 正常流程：
  1. 启动 `mysql`，等待 `mysqladmin ping` 通过。
  2. 启动 `redis`，等待 `redis-cli ping` 通过。
  3. `make compose-up` 先执行 `docker compose rm -sf migrate`，移除上一次已完成或失败的 migration job 容器。
  4. 启动新的 `migrate`，等待 migration `up` 成功完成。
  5. 构建并启动 `app`。
  6. `app` 连接 MySQL，按配置连接 Redis；HTTP server 监听 8080。
  7. Docker healthcheck 请求 `/actuator/health`，成功后 app 标记为 healthy。
- 异常流程：
  - MySQL 不健康：`migrate` 和 `app` 不启动。
  - Redis 不健康：`app` 不启动。
  - migration 失败：`app` 不启动，开发者通过 `docker compose logs migrate` 或 `make migrate-up` 排查。
  - 已存在旧 `migrate` 容器：`make compose-up` 会先移除它，再让 Compose 创建新的 migration job；如果开发者直接运行裸 `docker compose up --build`，仍可能复用旧容器，因此 README 推荐使用 Makefile 入口。
  - app 启动失败：容器退出或 healthcheck unhealthy。
  - lint 本机未安装：Makefile 自动走固定版本 Docker 镜像；若 Docker 不可用则 lint 失败并暴露原因。
- 状态流转：
  - 不涉及业务状态机。
  - 只涉及容器服务状态：created -> starting -> healthy/completed -> app starting -> app healthy。
- handler / service / repository / sqlc/database 分工：
  - handler/service/repository/sqlc 不新增业务逻辑。
  - `internal/config` 只解析 Redis 环境变量。
  - `internal/app/providers` 只做启动期依赖装配和 ping，不承载业务规则。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无，本次不涉及库存或订单。
- 如何防重复提交：无，本次不新增业务写接口。
- 事务边界在哪里：
  - 不改变现有 service 事务边界。
  - migration 由 golang-migrate 自身按文件执行；业务应用不在启动时包裹 migration 事务。
- 缓存放在哪里，为什么：
  - Redis 只作为本地依赖和后续缓存/幂等能力基础启动。
  - 本次不新增 Redis 业务读写，也不把 Redis 作为 auth session 权威记录。
  - 不把 Redis 详情写入 health response，避免扩大 API 契约；Compose 已通过 Redis 容器 healthcheck 控制启动顺序。

## 10. 权限与安全
- 哪些角色能访问：
  - 本次不新增业务权限。
  - health endpoint 仍按当前公开探活语义可访问。
- 鉴权与鉴别约束：
  - 不改变 auth middleware、RBAC 或 bearer token 解析。
- JWT claim 边界：
  - 不修改 JWT。
  - 不把角色、邮箱、用户名、用户状态写入 JWT。
- 是否涉及敏感信息、审计或操作日志：
  - `configs/prod.env.example` 不提供真实密钥；生产密钥必须由外部注入。
  - Dockerfile runtime 默认 `EVENTHUB_ENV=prod` 且 `OPENAPI_ENABLED=false`，避免镜像裸跑时暴露 Swagger。
  - Compose 中的密码和 token secret 只用于本地演示，README 明确不可用于生产。
  - runtime 容器使用非 root 用户，降低容器逃逸或误操作风险。

## 11. 测试策略
- 单元测试：
  - 如新增 Redis config 解析，补充或复用 config 测试验证默认值和 env 覆盖。
- service / repository 测试：
  - 不新增；本次不改业务 service/repository。
- migration / sqlc 验证：
  - 不运行 `make sqlc` 作为必需验证，因为本次不改 SQL query 或 sqlc 配置；但 Makefile 保留 `sqlc` 目标。
  - Compose `migrate` 和 `make migrate-up/down` 使用现有 migration。
- 接口验证：
  - `curl http://localhost:8080/actuator/health` 验证 app 容器可响应健康检查。
- OpenAPI validate：
  - 本次不改 OpenAPI 契约；`quality` 不默认串联 openapi validate，避免每次普通代码质量门禁都触发生成工具下载。
  - `openapi-validate` 保持独立目标，API 契约变更时运行。
- 异常场景验证：
  - 本机未安装 golangci-lint 时，`make lint` 使用固定 Docker 镜像 fallback。
  - prod env example 和 Dockerfile 默认不打开 Swagger。
- Java-Go parity 验证：
  - 对照 Java `docker-compose.yml`、`backend/Dockerfile`、prod OpenAPI hardening ADR、Flyway 迁移策略。
- 需要运行的命令：
  - `make fmt vet test`
  - `make lint`
  - `make quality`
  - `make docker-build`
  - `make compose-up`
  - `curl http://localhost:8080/actuator/health`
  - `make compose-down`

## 12. 风险与替代方案
- 当前方案的风险：
  - `docker compose up --build` 首次需要拉取 Go、MySQL、Redis、migrate 等镜像，受网络和 Docker Hub 状态影响。
  - golangci-lint Docker fallback 需要 Docker 可用；没有本机工具且 Docker 不可用时 lint 无法执行。
  - Compose 使用本地演示密码，不能代表生产部署安全方案。
  - migration job 成功后 app 才启动，但 app runtime 本身不负责自动迁移；部署流程必须显式执行 migration。
  - 裸 `docker compose up --build` 不会感知 bind mount 里的 migration SQL 内容变化；本地完整启动应使用 `make compose-up`。
  - Alpine runtime 带有轻量 shell/wget 以支持 healthcheck，镜像不是最小的 distroless 形态。
- 备选方案：
  - 方案 A：使用 distroless/static 作为最终运行镜像。
  - 方案 B：在 app 启动时自动运行 migration。
  - 方案 C：Compose 只启动 MySQL/Redis，不启动 app。
  - 方案 D：Makefile `lint` 只调用本机 golangci-lint。
  - 方案 E：一次性开启大量 golangci-lint 规则。
- 为什么不选备选方案：
  - 不选方案 A：distroless 更小，但缺少 shell/wget；当前要求 Compose healthcheck 直接使用 health endpoint，Alpine 能在不引入 Go 工具链的前提下提供轻量探针工具。
  - 不选方案 B：自动 migration 会把部署编排职责塞进业务进程，后续多副本启动时容易产生竞争；显式 migration job 更清晰。
  - 不选方案 C：Java 版已经把 app 纳入 Compose，本次目标也是完整本地闭环。
  - 不选方案 D：历史 implementation note 多次记录本机未安装 golangci-lint；没有 fallback 会让质量门禁不可重复。
  - 不选方案 E：当前项目仍在后端迁移阶段，过高噪音规则会把注意力从业务 parity 转移到风格争议；先固定低噪音规则，再按需要演进。
- 后续可演进点：
  - CI 中串联 `make quality`、`make openapi-validate`、`make docker-build`。
  - 增加 compose profile，区分只启动依赖与完整 app。
  - 增加 production deployment 文档，接入 Secret 管理和只读镜像扫描。
  - 需要更小攻击面时评估 distroless，并为 healthcheck 增加应用内 probe 子命令或平台侧 HTTP probe。
