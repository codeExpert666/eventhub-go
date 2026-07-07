# HTTP Error Boundary Cleanup 实现说明

## 1. 本次改动解决了什么问题

本次改动解决了 `internal/http/validation` 包职责混杂的问题。

改动前，`internal/http/validation` 既提供 HTTP 请求校验错误构造，也提供 `validation.AppErrorFromError` 这种“任意 error 到 AppError”的响应边界收敛能力。后者并不属于 validation 语义，容易让后续读者误以为普通业务错误映射也是 HTTP 参数校验的一部分。

改动后，普通 `error` 到 `*apperror.AppError` 的收敛归属 `internal/apperror`；HTTP request 错误构造只保留请求体、字段、query/path 参数校验错误构造。对外 API、HTTP 状态码、错误码、错误消息和响应 envelope 不变。

2026-07-07 后续阶段说明：design/implementation 035 已将这部分 HTTP request 错误构造从 `internal/http/validation` 重命名为 `internal/http/requesterror`，公共函数名收敛为 `MalformedBody`、`InvalidBody`、`InvalidParameters`。本文件中保留的 030 阶段命令和文件名用于记录当时执行历史。

## 2. 改动内容
- 新增了什么
  - `internal/apperror.FromErrorOrInternal`
    - 已是 `AppError` 的错误原样返回。
    - 普通错误包装为 `COMMON-500`，并保留 cause。
    - typed nil `*AppError` 按 nil 处理，兜底为 `COMMON-500`。
  - `internal/http/validation.ParameterValidationError`
    - 统一构造 query/path 参数校验错误。
    - 返回 `COMMON-400` 和 `message=请求参数校验失败`。
  - `internal/http/validation/request_error.go`
    - 承载 `FieldErrors`、`BodyValidationError`、`MalformedBodyError` 和 `ParameterValidationError`。
  - `internal/http/validation/request_error_test.go`
    - 覆盖请求体字段校验、请求体格式错误和参数校验错误构造。
  - `docs/ai/design/030-http-error-boundary-cleanup.md`。
  - `docs/ai/implementation/030-http-error-boundary-cleanup.md`。
- 修改了什么
  - `internal/apperror/mapper.go`
    - 增加错误兜底收敛公共函数。
    - 调整 `FromError`，避免 typed nil `*AppError` 被当作有效 app error 返回。
  - `internal/apperror/error_test.go`
    - 增加 `FromErrorOrInternal` 的 red-green 测试。
  - `internal/http/handler/auth/strict.go`
    - service 错误改为调用 `apperror.FromErrorOrInternal`。
  - `internal/http/handler/user/strict.go`
    - service / security 错误改为调用 `apperror.FromErrorOrInternal`。
  - `internal/http/openapi_routes.go`
    - strict response error handler 改为调用 `apperror.FromErrorOrInternal`。
    - generated path/query 参数绑定错误最终调用当时的 `validation.ParameterValidationError`，035 阶段后对应为 `requesterror.InvalidParameters`。
  - `internal/http/handler/user/admin_validation.go`
    - 管理员 query/path 参数校验失败改为调用当时的 `validation.ParameterValidationError`，删除包内重复的 `queryValidationError`；035 阶段后对应为 `requesterror.InvalidParameters`。
  - `docs/ai/design/029-openapi-strict-server-router-migration.md`
    - 将 strict handler/service 错误收敛描述更新为 `apperror.FromErrorOrInternal`。
  - `docs/ai/implementation/029-openapi-strict-server-router-migration.md`
    - 补充 2026-07-05 后续错误边界整理对 `internal/http/validation` 和 `apperror` 的影响。
  - `docs/ai/parity/java-go-parity-matrix.md`
    - 更新统一响应、错误码与校验映射行，说明 `apperror` 和 `http/validation` 的新职责边界。
  - `internal/http/response/response.go`
    - 删除本地 `normalizeError`。
    - `WriteError` 改为通过 `apperror.FromErrorOrInternal` 兜底 nil / typed nil app error。
    - `errorResponse` 假设调用方已传入归一化后的 app error，只负责 generated `ErrorResponse` 组装。
  - `internal/http/response/response_test.go`
    - 增加 `WriteError(nil)` 行为测试。
    - 增加 AST 结构测试，确保 response 包不再定义本地 `normalizeError`。
- 删除了什么
  - `internal/http/validation/error.go`
    - 删除 `validation.AppErrorFromError`。
  - `internal/http/validation/json.go`
    - 内容迁移到 `request_error.go`，文件名从 JSON 解码历史语义调整为请求错误构造语义。
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次虽不改变外部错误契约，但调整了 Go 端错误收敛与请求校验映射的职责边界，属于 Java-Go 契约治理索引需要记录的 Go-only 结构差异。

## 3. 为什么这样设计
- 关键设计原因
  - `AppError`、错误码和普通错误兜底都属于应用错误模型能力，放在 `internal/apperror` 更符合职责归属。
  - HTTP request 错误构造继续留在 HTTP 层，避免把 `请求参数校验失败` 这类 HTTP 入口语义下沉到 service/domain。
  - 035 阶段后，`requesterror.InvalidParameters` 只接收 `FieldErrors`，不依赖 OpenAPI generated 参数错误类型；oapi-codegen 错误解析仍留在 `internal/http/openapi_routes.go`。
  - 删除 `queryValidationError` 可以避免 user handler 内部复制公共错误消息。
- 与 Go 项目当前阶段的匹配点
  - 不新增依赖。
  - 不改变 strict-server route registration。
  - 不改变 handler -> service -> repository -> sqlc/database 分层。
  - handler 仍负责 HTTP 边界错误映射，service 不依赖 HTTP validation 包。
- 与 Java 版业务语义的对齐方式
  - Java 仍由 `GlobalExceptionHandler` 统一收敛异常并输出 `ApiResponse`。
  - Go 通过 `apperror.FromErrorOrInternal`、`requesterror.MalformedBody` / `InvalidBody` / `InvalidParameters` 和 `response.WriteError` 显式完成等价语义。
  - `response.WriteError` 内部对 nil 保留防御式兜底，但兜底逻辑复用 `apperror.FromErrorOrInternal`，不在 response 包重复维护错误归一化函数。
  - 对外 `COMMON-400`、`COMMON-500`、错误消息和字段详情结构不变。

## 4. 替代方案
- 方案 A：保留 `validation.AppErrorFromError`，只改注释。
  - 没有采用。它无法解决 validation 包公共 API 的误导性。
- 方案 B：把整个 `internal/http/validation` 包重命名为 `internal/http/requesterror`。
  - 030 阶段没有采用，因为当时优先收敛 `apperror` 与 HTTP request 错误构造边界。035 阶段为承接 OpenAPI request contract gate，已采用该命名收口。
- 方案 C：把 `ParameterValidationError` 放入 `internal/apperror`。
  - 没有采用。query/path 参数错误是 HTTP 入参语义，不应作为通用应用错误模型 API 暴露给 service/domain。
- 方案 D：让 `response.WriteError` 直接接收普通 `error`。
  - 没有采用。`response` 包应只负责写出 `*AppError`，不承担普通错误分类和收敛，避免 response 与错误模型职责混合。
- 方案 E：保留 `response.normalizeError` 只处理 nil。
  - 没有采用。`apperror.FromErrorOrInternal` 已经是错误兜底收敛入口；继续保留本地 normalizer 会形成重复边界，并且容易遗漏 typed nil 这类 Go interface 细节。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./internal/apperror ./internal/http/validation -count=1`
    - 失败原因符合预期：`apperror.FromErrorOrInternal` 和 `validation.ParameterValidationError` 尚未定义。
  - GREEN：`gofmt -w ... && go test ./internal/apperror ./internal/http/validation -count=1`
    - 通过。
  - RED：`go test ./internal/apperror ./internal/http/response -count=1`
    - 失败原因符合预期：
      - typed nil `*AppError` 通过 `FromErrorOrInternal` 返回 nil。
      - `response` 包仍定义本地 `normalizeError`。
  - GREEN：`gofmt -w ... && go test ./internal/apperror ./internal/http/response -count=1`
    - 通过。
  - `go test ./internal/http -count=1`
    - 通过。
  - `go test ./...`
    - 通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - 已对相关 Go 文件执行 `gofmt`。
  - 已对所有已改且仍存在的 Go 文件执行 `gofmt -l` 检查，无输出。
  - `go vet ./...`：通过。
  - `make lint`：通过，输出 `0 issues.`。
  - `git diff --check`：通过。
  - `make quality-check`：未通过，停在 `fmt-check` 的 `git diff --exit-code -- '*.go'`。原因是当前工作树包含本次和既有未提交 Go diff，该 target 设计为在无 Go diff 的读检查场景使用；它不是测试、vet 或 lint 失败。
  - `sqlc generate`：未运行，本次不涉及 SQL、schema 或 sqlc 配置。
  - OpenAPI validate / generate：未运行，本次不涉及 OpenAPI 契约或 generated code。
- 手工验证了哪些场景
  - 使用 `rg` 检查生产代码中已无 `validation.AppErrorFromError` 和 `queryValidationError` 调用。
  - 使用结构测试检查 `internal/http/response` 已无 `normalizeError`。
  - 检查 `internal/http/openapi_routes.go` 的 generated 参数错误解析仍保留在 HTTP route adapter 内，没有让 `validation` 包依赖 OpenAPI generated error 类型。
- Java-Go parity 如何验证
  - 更新 `docs/ai/parity/java-go-parity-matrix.md` 的统一响应、错误码与校验映射行。
  - 确认外部 `code/message/data/requestId/timestamp` 契约不变。
- 结果如何
  - 代码层面的单元测试、HTTP 包测试、全量 Go 测试、vet、lint 和 whitespace 检查均通过。
  - `make quality-check` 的失败原因已确认是 dirty worktree 中存在 Go diff，不是本次代码质量失败。

## 6. 已知限制
- 030 阶段曾保留 `internal/http/validation` 包名，没有整体改成 `requesterror`。
  - 035 阶段已解除该限制：当前 HTTP request 错误构造包为 `internal/http/requesterror`，不执行 OpenAPI contract 校验。
- 历史 implementation note 中仍可能提到早期 `internal/http/validation/json.go` 或 `error.go` 的新增背景。
  - 本次新增 030 文档作为当前状态索引，不回写所有历史记录。
- `make quality-check` 在 dirty worktree 中不适合作为最终读检查。
  - 后续如果希望 dirty worktree 也能跑完整读检查，可考虑新增一个不执行 `git diff --exit-code` 的 `quality-dirty-check` target。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 错误边界更清楚：应用错误模型、HTTP 请求校验和响应写出各自职责明确。
- 对微服务 / 云原生演进的影响
  - 后续拆分服务时，`internal/apperror` 可以继续作为应用错误模型基础；HTTP 入参校验错误仍留在 HTTP adapter 层。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 新增 HTTP handler 时，service 错误应调用 `apperror.FromErrorOrInternal` 收敛。
  - 新增 body 字段校验时继续调用 `requesterror.InvalidBody`。
  - 新增 query/path 参数校验时继续调用 `requesterror.InvalidParameters`。
  - response 包后续只新增写出或 envelope 组装能力，不再新增本地错误归一化 helper。
  - 不影响 migration、sqlc 或 OpenAPI 生成策略。
