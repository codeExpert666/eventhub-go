# OpenAPI Request Contract Gate 实现说明

## 1. 本次改动解决了什么问题

本次是 design 035 / ADR-0027 的阶段一，只解决命名与职责收口问题：原 `internal/http/validation` 实际只构造 HTTP request 相关 `AppError`，不是运行时 OpenAPI request validation 执行器。若后续新增 `internal/http/contract`，继续保留 `validation` 包名会让“错误构造”和“契约校验执行”边界不清。

本阶段将 HTTP request 错误构造能力收敛到 `internal/http/requesterror`，并把公共函数改成错误语义命名。对外 API、错误码、错误消息、响应 envelope、认证/授权行为和 strict-server 路由行为均不变。

## 2. 改动内容
- 新增了什么
  - `internal/http/requesterror/request_error.go`
    - 承载 `FieldErrors`、`MalformedBody`、`InvalidBody`、`InvalidParameters`。
  - `internal/http/requesterror/request_error_test.go`
    - 覆盖 malformed body、invalid body 和 invalid parameters 的 `COMMON-400` 错误构造。
  - 本实现说明。
- 修改了什么
  - 将 `internal/http/validation` 移动并重命名为 `internal/http/requesterror`，package 名从 `validation` 改为 `requesterror`。
  - 将公共函数重命名：
    - `MalformedBodyError` -> `MalformedBody`
    - `BodyValidationError` -> `InvalidBody`
    - `ParameterValidationError` -> `InvalidParameters`
  - 更新 `internal/http/openapi_routes.go`、`internal/http/handler/{auth,system,user}` 中的 import 与调用点。
  - 更新 `AGENTS.md`、近期设计文档、当前 auth parity checklist 和主 parity matrix 中会影响后续执行判断的当前引用。
- 删除了什么
  - 删除空的 `internal/http/validation` 目录；没有保留兼容别名或 wrapper。
- 是否更新 Java-Go parity 记录
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
  - 这次只更新 Go 端包名与函数名，但它属于 Java-Go 契约治理的 Go-only 结构边界：`requesterror` 已落地为错误构造包，`internal/http/contract` 仍待后续阶段新增。

## 3. 为什么这样设计
- 关键设计原因
  - `requesterror` 明确表达“构造 HTTP request 错误”，避免与后续真正执行 OpenAPI request contract gate 的 `contract` 包混淆。
  - `MalformedBody`、`InvalidBody`、`InvalidParameters` 是错误语义命名，不暗示该包负责执行校验。
  - 不保留旧包别名，可以让新代码直接使用清晰边界，避免两个公共 API 长期并存。
- 与 Go 项目当前阶段的匹配点
  - 只改 HTTP 边界包名和调用点，不触碰 service/domain/repository。
  - 不新增依赖，不引入 `kin-openapi`。
  - 不新增 `internal/http/contract`、配置项、provider 装配或 runtime middleware。
  - 不恢复 Go embed，不恢复 `api/openapi/spec.go:SpecYAML()`。
- 与 Java 版业务语义的对齐方式
  - Java 版由 Spring MVC / Bean Validation / `GlobalExceptionHandler` 产生请求错误并统一响应。
  - Go 版本阶段仍通过 `requesterror` + `apperror` + `response.WriteError` 输出同等 `COMMON-400` 语义；真正的 OpenAPI contract gate 后续再按设计单独落地。

## 4. 替代方案
- 方案 A：继续保留 `internal/http/validation` 包名，只改注释。
- 方案 B：新增 `internal/http/contract` 并在同一阶段接入 middleware。
- 方案 C：保留旧函数名作为兼容 wrapper。
- 为什么没有采用
  - 不采用方案 A：包名仍会与后续 runtime request validation 混淆，不能完成 design 035 阶段一目标。
  - 不采用方案 B：本阶段明确只做命名与职责收口，新增 middleware 会扩大行为变化面。
  - 不采用方案 C：仓库内调用点可一次性更新；保留 wrapper 会让旧语义继续扩散。

## 5. 测试与验证
- 跑了哪些测试
  - `go test ./internal/http/requesterror ./internal/http/handler/... ./internal/http -count=1`：通过。
  - `go test ./...`：通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w internal/http/requesterror/request_error.go internal/http/requesterror/request_error_test.go internal/http/openapi_routes.go internal/http/handler/system/validation.go internal/http/handler/auth/validation.go internal/http/handler/user/admin_validation.go`：已执行。
  - `go vet ./...`：通过。
  - `git diff --check`：通过。
  - `sqlc generate`：未运行，本阶段不涉及 SQL、schema、repository 或 sqlc 配置。
  - OpenAPI validate / generate：未运行，本阶段不改变 OpenAPI 契约、生成配置或 generated code。
- 手工验证了哪些场景
  - 使用 `rg` 检查生产 Go 代码中已无 `internal/http/validation` import、`validation.` 调用或旧公共函数名。
  - 检查 `internal/http/openapi_routes.go` 仍只映射 generated strict request/body 与 path/query 绑定错误，没有新增 contract middleware。
- Java-Go parity 如何验证
  - 主 parity matrix 已记录 `requesterror` 阶段一落地、`contract` 仍待后续阶段。
  - 外部错误码、错误消息和响应 envelope 未变化。
- 结果如何
  - 阶段建议验证均已通过；没有发现行为回归。

## 6. 已知限制
- 当前仍没有运行时 OpenAPI request contract gate；`internal/http/contract`、`OPENAPI_SPEC_PATH`、`OPENAPI_REQUEST_VALIDATION_ENABLED`、spec loader 和 middleware 接入均留到后续阶段。
- handler 内部的 `validation.go` 文件名仍表示模块内“请求字段校验 helper”，不是 `internal/http/validation` package；本阶段不重命名这些业务模块文件，避免无意义 churn。
- 历史 design / implementation note 中仍可能保留 `internal/http/validation` 作为当时阶段的执行记录；当前状态以 design/implementation 035、AGENTS 和 parity matrix 为准。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - HTTP request 错误构造、统一错误响应和后续 OpenAPI contract gate 的包边界更清晰，便于解释工程演进。
- 对微服务 / 云原生演进的影响
  - 后续若将 request contract gate 前置到网关或 sidecar，本服务内 `requesterror` 仍可作为 HTTP adapter 层的稳定错误构造边界。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续阶段新增 `internal/http/contract` 时应复用 `requesterror` 构造稳定 `AppError`，但不让 `requesterror` 反向依赖 contract 或 `kin-openapi`。
  - service/domain/repository 继续不依赖 `api/openapi/gen`、`internal/http/contract` 或 `internal/http/requesterror`。
  - 不影响 migration、sqlc 或 OpenAPI generated code。
