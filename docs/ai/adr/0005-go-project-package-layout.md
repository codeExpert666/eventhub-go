# Go 项目 package layout 决策 ADR

## 标题
Go 版 EventHub 采用混合式 package layout 固化 handler/service/repository/sqlc 边界

## 状态
- accepted

## 背景
当前 Go 版 EventHub 已完成 HTTP foundation，具备 `cmd/eventhub`、`internal/http`、`internal/apperror`、`internal/page`、`internal/config`、`internal/platform/log` 等基础能力。

2026-05-31 的项目结构规范化重构进一步把规则落到运行时代码：应用装配进入 `internal/app`，system HTTP DTO 进入 `internal/http/dto`，system 基础能力进入 `internal/service/system`，request id 进入 `internal/platform/idgen`，时间抽象进入 `internal/platform/clock`。

后续业务、数据库、OpenAPI、migration、auth/user/event/order/payment 等模块尚未迁移。如果此时不先固化目录规范，后续 Codex 生成代码时容易出现以下问题：

- 为了快速实现，把业务逻辑写进 handler 或 `cmd/eventhub/main.go`。
- handler 直接访问 sqlc、`database/sql` 或 redis。
- sqlc generated model 被当成 domain model 向 HTTP 层泄漏。
- domain 依赖 HTTP DTO、database、redis 或 config。
- security、platform、repository 和 service 边界混杂。
- 为了“看起来完整”提前创建空 Go package，增加无意义编译单元和维护成本。

因此需要在业务和数据库迁移前，将 Go 项目目录结构写入 `AGENTS.md`、skill、docs/ai 和 parity matrix，作为长期工程约束。

## 决策
Go 版 EventHub 采用 `AGENTS.md` 中定义的规范结构作为长期目标，核心边界如下：

- `cmd/eventhub/main.go` 只放可执行入口和最薄的启动调用，不放业务逻辑。
- `internal/app` 放应用装配和生命周期管理。
- `internal/config` 放配置读取、环境和 profile。
- `internal/platform` 放跨业务基础设施能力，例如 db、redis、log、clock、idgen、crypto。
- `internal/http` 放 router、server、middleware、handler、dto、response、validation。
- `internal/apperror` 放应用错误码、错误类型和错误映射。
- `internal/page` 放分页请求和响应模型。
- `internal/domain` 放领域模型、枚举和值对象，不依赖 HTTP、sqlc、database、redis、config。
- `internal/service` 放业务用例、事务边界、权限后的业务规则、并发一致性和幂等决策。
- `internal/repository` 放持久化接口。
- `internal/repository/mysql` 放 MySQL repository 实现。
- `internal/repository/mysql/queries` 放 sqlc query。
- `internal/repository/mysql/sqlc` 放 sqlc generated code。
- `internal/security` 放认证、安全上下文、密码、JWT、refresh token、user agent 摘要等安全基础能力。
- `api/openapi` 放 OpenAPI 契约和生成代码。
- `migrations` 放数据库 migration。
- `configs` 放配置示例。
- `docs/ai` 继续作为工程质量和 Java-Go parity 的一部分。

### HTTP DTO 与 VO 边界

Go 版不创建 `internal/http/vo`。HTTP 请求体、query 参数对象、path 参数辅助对象、HTTP response data、list item / summary / detail response 对象统一放 `internal/http/dto`，并通过 `XxxRequest`、`XxxResponse`、`XxxListItemResponse`、`XxxSummaryResponse`、`XxxDetailResponse` 等后缀表达用途。

`internal/http/response` 只放统一响应 envelope 和 writer，例如 `APIResponse`、`Success` / `Failure`、`WriteSuccess` / `WriteError` / `WriteJSON` / `WriteStatus`，不放具体业务响应 DTO。

Java 项目中常见的 VO 命名在 Go 版不直接复刻：HTTP 展示对象归入 `internal/http/dto`，DDD Value Object 归入 `internal/domain/<domain>` 或 `internal/domain/common`。service 不依赖 `internal/http/dto`，handler 负责 DTO 与 service Command / Query、service result / domain model 之间的映射；repository/mysql 负责 sqlc row 与 domain model 的映射。

同时采用阶段化落地原则：当前阶段没有实现的业务包不创建空 `.go` 文件；允许使用 `.gitkeep` 或 `README.md` 作为非 Go 目录占位，但不要制造无法编译或无意义 package。

当前已落地的运行时结构包括：

- `internal/app`：配置、日志、HTTP server 和进程生命周期装配。
- `internal/http/dto`：system HTTP request/response data。
- `internal/service/system`：system ping/echo/health/info 的非 HTTP 组装逻辑。
- `internal/platform/clock`：跨业务时间来源接口和真实时钟。
- `internal/platform/idgen`：request id 生成、校验和 context 传递。

当前仍按阶段化原则保留、不创建空 Go package 的结构包括：

- `internal/domain`
- `internal/repository`
- `internal/security`
- `internal/platform/db`
- `internal/platform/redis`
- `internal/platform/crypto`
- `internal/repository/mysql/queries`
- `internal/repository/mysql/sqlc`

## 备选方案
- 方案 1：完全横向分层，例如统一使用 `internal/handler`、`internal/service`、`internal/repository`、`internal/domain`。
- 方案 2：完全纵向 modules，例如 `internal/modules/user/{handler,service,repository,domain}`。
- 方案 3：当前选择的混合式结构，在 HTTP、service、repository、domain、security、platform 之间保持清晰边界，并在部分层内按业务域拆包。

## 决策理由
选择当前混合式结构的原因：

- 便于 Java-Go parity：Java 版 Controller / Service / Mapper / Entity / Config / Security 能稳定映射到 Go 的 http / service / repository / domain / config / security 边界。
- 便于 handler/service/repository/sqlc 对照学习：每一层职责明确，适合持续复刻 Java 业务语义而不复制 Spring 结构。
- 便于 Go `internal` package 约束：运行时代码统一放在 `internal` 下，减少被外部误引用的风险。
- 避免业务逻辑散落：handler 不直连数据库，repository/mysql 不处理 HTTP 响应，service 不拼 HTTP JSON。
- 便于控制生成代码边界：sqlc generated code 被限制在 `internal/repository/mysql/sqlc`，OpenAPI generated code 被限制在 `api/openapi/gen`，都不能污染 domain model。
- 便于阶段化演进：当前没有的业务包不创建空 Go package，等 auth/user/event/order 等阶段开始后再按规范补齐。

## 影响
- 好处：
  - 后续 Codex 必须按规范落目录。
  - HTTP foundation 已通过实际代码验证 `cmd -> app -> http/handler -> service` 的基本分层。
  - 目录结构变更需更新 `docs/ai`，让结构演进有设计和实现记录。
  - Java 分层到 Go 目录的映射可以在 parity matrix 中持续追踪。
  - sqlc、OpenAPI 和安全能力有明确边界，减少后续返工。
- 代价：
  - 规则比当前实际目录更完整，短期会存在尚未落地的目标目录。
  - 每次移动 package、拆分层次或引入 repository/sqlc/openapi/migrations 都需要同步更新设计文档和 implementation note。
  - 偏离规范时必须新增或更新 ADR，增加一定文档成本。
- 后续可能需要调整的地方：
  - 如果业务复杂度要求局部纵向 modules，可以在设计文档和 ADR 中说明后调整。
  - 引入 sqlc、migration、OpenAPI 生成器后，需要验证生成路径是否与本 ADR 保持一致。
  - 空 Go package 不应为了凑结构创建；未落地目录只作为长期目标存在。
