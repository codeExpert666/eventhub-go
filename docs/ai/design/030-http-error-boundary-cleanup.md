# HTTP Error Boundary Cleanup

## 1. 背景
- 当前 `internal/http/validation` 同时承载两类职责：
  - HTTP 请求体、字段、参数校验错误构造，例如 `BodyValidationError`、`MalformedBodyError` 和 `FieldErrors`。
  - 普通 `error` 到 `*apperror.AppError` 的响应边界收敛，例如 `AppErrorFromError`。
- `AppErrorFromError` 最早服务于 direct HTTP handler，strict-server 迁移后继续被 strict handler 与 OpenAPI response error handler 调用，但它的语义已经不属于 validation。
- Java 版对应语义来自：
  - `ApiResponse.java`
  - `ErrorCode.java`
  - `BusinessException.java`
  - `GlobalExceptionHandler.java`
  - Bean Validation / Spring MVC 参数绑定错误处理
- Go 版应保持 Java 外部契约：错误响应仍输出 `code/message/data/requestId/timestamp`，请求体格式错误、字段校验错误和参数绑定错误继续映射到 `COMMON-400`。

## 2. 目标
- 将普通 `error` 到 `*apperror.AppError` 的兜底转换移动到 `internal/apperror`，让错误模型包拥有错误收敛能力。
- `internal/http/response` 不再保留本地 `normalizeError`；`WriteError` 只通过 `apperror.FromErrorOrInternal` 做 nil / typed nil 防御式兜底。
- 让 `internal/http/validation` 只保留 HTTP 请求校验错误构造能力：
  - 请求体字段校验失败。
  - 请求体缺失或 JSON 格式错误。
  - query/path 参数校验失败。
- 删除 `validation.AppErrorFromError` 这个命名错位的公共 API。
- 保持所有对外 API path、method、状态码、错误码、错误消息和响应字段不变。
- 用单元测试锁定 `apperror` 与 HTTP validation helper 的行为。

## 3. 非目标
- 不修改 `api/openapi/eventhub.yaml` 或 generated OpenAPI code。
- 不改变 strict-server route registration、认证顺序或 ADMIN 授权规则。
- 不改 service、repository、sqlc、migration、数据库表或缓存。
- 不引入新的 validator 或错误处理依赖。
- 不把整个 `internal/http/validation` 包重命名为 `httperror` 或其他新包名；本次只收敛职责和公共函数命名。
- 不新增 ADR。本次是既有错误边界的职责整理，不是新的架构决策。

## 4. 影响范围
- 涉及 Go package：
  - `internal/apperror`
  - `internal/http/validation`
  - `internal/http/handler/auth`
  - `internal/http/handler/user`
  - `internal/http`
- 不触碰 package：
  - `internal/service`
  - `internal/repository`
  - `internal/domain`
  - `internal/security`
  - `api/openapi/gen`
- 涉及 API / 表 / 缓存 / 外部接口：
  - API 契约不变。
  - 数据库、migration、sqlc、缓存和外部接口不变。
- 影响 `docs/ai/parity/java-go-parity-matrix.md`。
  - 需要更新统一响应与校验映射行，说明 Go 端将 `apperror` 与 `http/validation` 的职责拆清，但 Java-Go 外部错误契约不变。

## 5. 领域建模
- `AppError`
  - 位于 `internal/apperror`。
  - 表示可被统一响应 envelope 写出的应用错误，对齐 Java `BusinessException` + `ErrorCode`。
- `FromErrorOrInternal`
  - 位于 `internal/apperror`。
  - 从任意错误链中提取 `AppError`；如果不是应用错误，则包装为 `COMMON-500`。
  - typed nil `*AppError` 按普通 nil 处理，兜底为 `COMMON-500`，避免 HTTP response 层重复维护本地 normalizer。
  - 这是错误模型层能力，不再放在 HTTP validation 包。
- `FieldErrors`
  - 位于 `internal/http/validation`，是 `apperror.Details` 的别名。
  - 表示 HTTP 请求字段、query/path 参数等字段级错误详情。
- `BodyValidationError` / `MalformedBodyError` / `ParameterValidationError`
  - 位于 `internal/http/validation`。
  - 只表达 HTTP 请求校验失败到 `COMMON-400` 的固定映射。

## 6. API 设计
- 对外 API 列表不变。
- 请求参数：
  - strict-server 继续负责 JSON body 解码和 path/query 参数绑定。
  - handler 继续负责 generated request object 到 service Command / Query 的映射和业务字段校验。
- 响应结构：
  - 成功响应仍使用 generated typed `ApiResponseXxx`。
  - 错误响应仍通过 `response.WriteError` 输出 generated `ErrorResponse`；`response` 包只做写出和 envelope 组装，不维护独立错误归一化逻辑。
- 错误码 / 异常场景：
  - 请求体缺失或 JSON 格式错误：HTTP 400，`COMMON-400`，`message=请求体格式不合法`。
  - 请求体字段校验失败：HTTP 400，`COMMON-400`，`message=请求体参数校验失败`。
  - query/path 参数绑定或校验失败：HTTP 400，`COMMON-400`，`message=请求参数校验失败`。
  - service / handler 返回业务 `AppError`：保留原错误码和消息。
  - 未预期普通错误：包装为 `COMMON-500`。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Java 通过 Spring MVC / Bean Validation / 全局异常处理统一收敛异常。
  - Go 通过 `apperror` + `http/validation` + `response.WriteError` 显式收敛错误。
  - 本次只整理 Go 包边界，不改变外部契约。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 本次不接触持久化和事务边界。

## 8. 关键流程
- 正常流程：
  1. strict handler 调用 service。
  2. service 成功返回 result。
  3. handler 映射为 generated success response。
- 请求校验异常流程：
  1. strict-server JSON 解码失败时调用 `validation.MalformedBodyError()`。
  2. handler 字段校验失败时调用 `validation.BodyValidationError(fields)`。
  3. generated chi wrapper 参数绑定失败或 handler 参数校验失败时调用 `validation.ParameterValidationError(fields)`。
  4. `response.WriteError` 写出统一错误 envelope。
- 业务错误异常流程：
  1. service / security / handler 返回 `error`。
  2. strict handler 或 route error handler 调用 `apperror.FromErrorOrInternal(err)`。
  3. 已是 `AppError` 时保留原错误；普通错误包装为 `COMMON-500`。
  4. `response.WriteError` 写出统一错误 envelope。
  5. 如果调用方传入 nil `*AppError`，`WriteError` 也通过 `apperror.FromErrorOrInternal` 兜底为 `COMMON-500`，不在 response 包内重复实现 `normalizeError`。
- handler / service / repository / sqlc/database 分工：
  - handler 仍只做 HTTP 入参、鉴权上下文、响应映射和错误映射。
  - service、repository、sqlc/database 不受影响。

## 9. 并发 / 幂等 / 缓存
- 不涉及库存、订单、支付或状态机。
- 不涉及并发写入、幂等键或缓存。
- 不改变事务边界。

## 10. 权限与安全
- 不改变认证、授权或 JWT claim。
- 不改变 ADMIN 路由判断和 BearerAuth middleware 顺序。
- 未预期普通错误仍包装为 `COMMON-500`，避免向客户端暴露底层错误细节。
- 字段级校验错误只返回已有的业务字段消息，不新增敏感信息输出。

## 11. 测试策略
- 单元测试：
  - `internal/apperror`：
    - `FromErrorOrInternal` 对已有 `AppError` 原样返回。
    - `FromErrorOrInternal` 对普通错误包装为 `COMMON-500`，并保留 cause。
    - `FromErrorOrInternal` 对 typed nil `*AppError` 返回非 nil `COMMON-500`。
  - `internal/http/response`：
    - `WriteError(nil)` 返回 HTTP 500、`COMMON-500`、`message=系统内部错误`，并保留 request id。
    - AST 结构测试确认 response 包不再定义本地 `normalizeError`。
  - `internal/http/validation`：
    - `BodyValidationError` 返回 `COMMON-400`、`请求体参数校验失败` 和字段详情。
    - `MalformedBodyError` 返回 `COMMON-400`、`请求体格式不合法` 和 `body` 字段详情。
    - `ParameterValidationError` 返回 `COMMON-400`、`请求参数校验失败` 和字段详情。
- service / repository 测试：
  - 不涉及。
- migration / sqlc 验证：
  - 不涉及。
- 接口验证：
  - 复用现有 `internal/http` 测试覆盖 malformed body、body validation、query/path validation 和 error envelope。
- OpenAPI validate：
  - 不涉及 OpenAPI schema 或 generated code 变化。
- 异常场景验证：
  - 普通 error 兜底为 `COMMON-500`。
  - 参数错误仍为 `COMMON-400`。
- Java-Go parity 验证：
  - 检查 parity matrix 中统一响应、错误码与校验映射行。
- 需要运行的命令：
  - `gofmt`
  - `go test ./internal/apperror ./internal/http/validation ./internal/http -count=1`
  - `go test ./...`
  - `go vet ./...`

## 12. 风险与替代方案
- 当前方案的风险：
  - 当前工作树已有 strict-server 迁移改动，重构必须避免扩大到无关 runtime route 或 OpenAPI generated code。
  - 文档中仍可能保留旧 `validation.AppErrorFromError` 描述，需要同步更新本次新增文档和 parity 索引。
- 备选方案 A：保留 `validation.AppErrorFromError`，只补注释。
  - 不采用。它不能解决包名和职责误导，后续读者仍会把普通错误收敛误认为 validation 能力。
- 备选方案 B：把整个 `internal/http/validation` 包改名为 `internal/http/requesterror`。
  - 不采用。调用点较多，且当前包名仍适合承载 HTTP 请求校验错误；全包重命名收益低、diff 大。
- 备选方案 C：把 `ParameterValidationError` 放到 `internal/apperror`。
  - 不采用。`请求参数校验失败` 是 HTTP 入参表达，service/domain 不应引入 HTTP validation 语义。
- 后续可演进点：
  - 如果后续字段校验规则显著增加，可评估引入轻量 validator，并通过 ADR 说明收益和代价。
