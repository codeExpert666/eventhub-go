# Go 版 HTTP 工程底座实现说明

## 1. 本次改动解决了什么问题

本次改动解决了 Go 版 EventHub 缺少可运行 HTTP 工程底座的问题。

Go 仓库现在已经具备最小可运行后端：统一响应、错误码与 `AppError`、requestId middleware、recover middleware、JSON validation、分页模型、system ping/echo、health/info、dev/test/prod 配置雏形、`slog` 结构化日志和 httptest 覆盖。

本次复查后补强了进程退出信号处理：第一次 `SIGINT/SIGTERM` 会触发 HTTP server 优雅关闭，并立即停止 signal notify 注册，使第二次 `Ctrl+C` 恢复默认行为，可以在优雅关闭卡住时强制退出。

本次补齐了 Actuator HEAD 契约：`HEAD /actuator/health` 与 `HEAD /actuator/info` 现在会返回 HTTP 200、保留 `X-Request-Id` 响应头，并且不写响应体，用于对齐 Java 版健康检查轻量探测语义。

本次修复了 recover middleware 在响应已提交后仍继续写统一错误体的问题。现在未提交响应时 panic 仍返回 `COMMON-500`；如果 handler 已经写出响应头或响应体后再 panic，recover 只记录服务端日志，不再追加错误 JSON。

本次实现对齐 Java 版基础工程语义，但没有逐行迁移 Spring Boot、Spring MVC、Bean Validation、MDC 或 Actuator 的实现方式。

## 2. 改动内容
- 新增了 Go module：
  - `go.mod`
  - `go.sum`
- 新增了应用入口：
  - `cmd/eventhub/main.go`
  - 入口中首次退出信号触发后立即调用 `stop()`，避免第二次 `Ctrl+C` 被 signal notify 继续接管。
- 新增了配置和日志基础设施：
  - `internal/config/config.go`
  - `internal/platform/log/logger.go`
- 新增了 HTTP server 和 router：
  - `internal/http/server.go`
  - `internal/http/router.go`
  - router 显式注册 `HEAD /actuator/health` 和 `HEAD /actuator/info`。
- 新增了 requestId 与 recover middleware：
  - `internal/http/requestid/requestid.go`
  - `internal/http/middleware/request_id.go`
  - `internal/http/middleware/recover.go`
  - recover middleware 增加响应提交状态追踪，避免已提交响应后再写 `COMMON-500`。
- 新增了统一响应、validation、错误码和分页模型：
  - `internal/http/response/response.go`
  - `internal/http/requesterror/json.go`
  - `internal/apperror/error.go`
  - `internal/page/page.go`
- 新增了系统基础 handler：
  - `internal/http/handler/system_handler.go`
  - Actuator HEAD handler 只写 HTTP 200 和响应头，不写响应体。
- 新增了 httptest 和基础对象测试：
  - `internal/http/router_test.go`
  - `internal/http/response/response_test.go`
  - `internal/http/requestid/requestid_test.go`
  - `internal/apperror/error_test.go`
  - `internal/page/page_test.go`
- 新增并更新了设计文档和 ADR：
  - `docs/ai/design/001-http-foundation.md`
  - `docs/ai/adr/0002-web-router-chi.md`
  - `docs/ai/adr/0003-error-response-contract.md`
  - `docs/ai/adr/0004-config-and-logging.md`
- 是否更新 Java-Go parity 记录：
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
  - 本次触发了 API 路径、响应字段、错误码、validation 行为、分页语义、requestId、router/config/logging 技术取舍和测试策略的 parity 更新条件。

## 3. 为什么这样设计
- `APIResponse` 保留 `code/message/data/requestId/timestamp`，直接对齐 Java `ApiResponse<T>` 的外部契约。
- `Code` + `AppError` 用 Go 显式错误返回表达 Java `ErrorCode` + `BusinessException` 的语义，避免用 `panic` 表达可预期业务失败。
- requestId 使用 `context.Context` 传递，而不是模拟 Java MDC；这是 Go HTTP 链路中更自然、可测试的请求级上下文方式。
- recover middleware 只兜底未预期 panic，并统一返回 `COMMON-500`；业务失败仍由 `AppError` 显式返回。
- recover middleware 只在 HTTP 响应尚未提交时写统一错误体；已提交响应无法安全改写状态码和 body，因此只记录 panic 日志，避免客户端收到部分成功响应后又混入错误 JSON。
- validation 在 handler 边界完成 JSON 解码和字段校验，对齐 Java 全局异常处理输出的 `COMMON-400` 语义。
- `PageRequest/PageResponse` 保持 Java 版 1-based page、默认第一页、默认 20 条、最大 100 条和派生分页元数据规则。
- `chi` 只承担路由和 middleware 编排，不接管响应、错误或业务分层，便于保持 Go 标准库边界。
- Actuator HEAD 端点使用显式 `router.Head` 注册，而不是隐式把 HEAD 映射到 GET；这样既对齐 Java 版 SecurityConfig 的方法级放行，也避免 HEAD 请求在 `httptest` 中写出 JSON body。
- 配置先使用环境变量和少量默认值，日志使用标准库 `slog`，避免在工程底座阶段引入过重依赖。
- signal lifecycle 由入口统一控制：`server.Run` 负责根据 context 做 10 秒优雅关闭，`main` 负责在首次信号后停止 signal notify，让二次中断回到操作系统默认行为。

## 4. 替代方案
- 方案 A：只用标准库 `http.ServeMux`。
  - 没有采用，因为后续业务模块会需要更清晰的路由分组、middleware 编排、NotFound/MethodNotAllowed 定制和路径参数能力；`chi` 在保持标准库兼容的同时减少样板代码。
- 方案 B：使用 Gin、Echo 或 Fiber。
  - 没有采用，因为这些框架更容易把响应、上下文和错误处理推向框架风格；当前 Go 版更需要稳住与 Java 版契约的对齐，而不是引入完整 Web 框架。
- 方案 C：引入 validator/viper/zerolog/logrus 等依赖。
  - 没有采用，因为本阶段只有少量字段校验、配置读取和结构化日志需求，标准库已经足够；后续复杂度上升时再通过 ADR 引入更合适。
- 方案 D：把 health/info 也包进 `APIResponse`。
  - 没有采用，因为 Java 版 Actuator 端点属于运维探活边界，不是业务 API；Go 版先保留轻量运维响应，业务接口继续统一包裹。
- 方案 E：把 system handler 拆成 service 层。
  - 没有采用，因为本次 system 能力只组装配置和时间，不接数据库也没有业务状态；过早抽 service 只会增加样板。后续业务模块仍必须遵守 `handler -> service -> repository -> sqlc/database`。
- 方案 F：只在 `main` 退出时 `defer stop()`。
  - 没有采用，因为优雅关闭窗口内第二次 `Ctrl+C` 仍可能被当前 signal notify 接管，不能及时恢复默认强制退出行为。
- 方案 G：使用 `chi/middleware.GetHead` 自动把 HEAD 请求映射到 GET handler。
  - 没有采用，因为当前契约希望 HEAD 探测不写响应体；显式 HEAD handler 更清楚，也更容易用 `httptest` 稳定验证。
- 方案 H：recover middleware 缓冲整个响应，直到 handler 正常结束后再写给客户端。
  - 没有采用，因为这会改变流式响应和大响应的语义，并增加内存占用；当前工程底座更适合用轻量 ResponseWriter wrapper 记录提交状态。

## 5. 测试与验证
- 跑了哪些测试：
  - `go test ./internal/http`：通过，覆盖 Actuator HEAD 契约，以及 recover 在响应未提交/已提交两类 panic 场景下的行为。
  - `go test ./...`：通过。
- 跑了哪些质量门禁：
  - `gofmt`：已对新增 Go 文件执行。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `sqlc generate`：不适用，本次没有 SQL、schema 或 sqlc 配置变化。
  - migration 测试：不适用，本次没有数据库 migration。
  - OpenAPI validate：不适用，本次没有 OpenAPI 文件。
- 手工验证了哪些场景：
  - 对照 Java `ApiResponse`、`ErrorCode`、`RequestIdFilter`、`SystemControllerTest`、`PageRequestTest` 和 `PageResponseTest`，确认 Go 端字段、错误码、requestId 校验规则和分页语义一致。
  - 对照 Java `SystemControllerTest#healthEndpointShouldPermitHeadRequest` 和 `SecurityConfig` 中 `HEAD /actuator/health`、`HEAD /actuator/info` 的放行规则，确认 Go 端已显式注册 Actuator HEAD 路由。
  - 复查 recover middleware，确认 handler 未写响应时 panic 会返回 HTTP 500 + `COMMON-500`，handler 已写响应后 panic 不再追加 `COMMON-500`。
  - 复查 `cmd/eventhub/main.go`，确认首次信号取消 context 后会立即调用 `stop()` 解除 signal notify 注册；`defer stop()` 继续作为正常退出和启动失败场景的兜底清理。
- Java-Go parity 如何验证：
  - 已在 parity matrix 中新增 HTTP foundation、统一响应、错误码、requestId、system API、health/info、分页、配置日志和测试策略记录。
- 结果如何：
  - 当前 HTTP 工程底座在无数据库、无认证前提下可编译、可测试、可运行。

## 6. 已知限制
- 当前不接数据库，因此 `/actuator/health` 只返回应用存活状态，没有 db、redis 等组件树。
- 当前不实现认证，因此 `AUTH-401/AUTH-403/AUTH-409` 只完成错误码与响应映射初始化，真实链路待 auth 模块迁移。
- 当前没有 OpenAPI 文件或生成器，API 契约只通过设计文档和 httptest 约束。
- validation 目前是手写规则，适合工程底座；如果后续请求 DTO 增多，需要评估是否引入验证库。
- `info` 端点只返回应用名、环境、版本和时间，后续可接入构建信息。
- recover middleware 当前不做响应缓冲，因此无法把“已提交部分响应后才 panic”的场景改写成标准 `COMMON-500`；这种情况下只保证不污染响应体并记录服务端日志。
- 当前没有 Makefile、CI 或 lint 配置；本次按可行质量门禁执行 `gofmt/go test/go vet`。

## 7. 对后续版本的影响
- 对简历可用版的价值：
  - 建立了可展示的 Go HTTP 工程基线，后续用户、活动、票种、订单、支付模块都可以复用统一响应、错误、requestId、日志和测试方式。
- 对微服务 / 云原生演进的影响：
  - requestId、结构化日志、health/info 和显式错误码为后续接入网关、日志平台、监控和服务拆分留出稳定边界。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响：
  - 业务模块应沿用 `AppError`、`APIResponse` 和 request context，不要在 handler 中直连数据库。
  - 引入数据库后需要补 repository/sqlc/migration 设计，并扩展 health components。
  - 引入 OpenAPI 后需要将当前 API 契约固化到 OpenAPI validate。
