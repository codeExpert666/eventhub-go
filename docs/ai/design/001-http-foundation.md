# Go 版 HTTP 工程底座设计

## 1. 背景
- 当前 Go 仓库只有协作规则和文档模板，尚未建立可运行的 HTTP 服务、统一响应、统一错误、requestId、日志和基础探活接口。
- Java 版对应语义主要来自：
  - `backend/src/main/java/com/eventhub/common/api/ApiResponse.java`
  - `backend/src/main/java/com/eventhub/common/api/ErrorCode.java`
  - `backend/src/main/java/com/eventhub/common/exception/BusinessException.java`
  - `backend/src/main/java/com/eventhub/common/exception/GlobalExceptionHandler.java`
  - `backend/src/main/java/com/eventhub/infra/logging/RequestIdFilter.java`
  - `backend/src/main/java/com/eventhub/common/api/PageRequest.java`
  - `backend/src/main/java/com/eventhub/common/api/PageResponse.java`
  - `backend/src/main/java/com/eventhub/modules/system/controller/SystemController.java`
  - `backend/src/main/java/com/eventhub/modules/system/service/SystemService.java`
  - `backend/src/test/java/com/eventhub/modules/system/controller/SystemControllerTest.java`
- 本次是 Go 版 EventHub 的工程起点，目标不是逐行迁移 Spring Boot，而是先用 Go idiom 建立与 Java 版业务契约一致的 HTTP 基础能力。

## 2. 目标
- 初始化 Go module 和 HTTP 服务入口 `cmd/eventhub/main.go`。
- 建立统一响应 `APIResponse`，字段固定为 `code/message/data/requestId/timestamp`。
- 初始化错误码与 HTTP status 映射：
  - `COMMON-000`：成功，HTTP 200
  - `COMMON-400`：请求参数不合法，HTTP 400
  - `COMMON-401`：业务处理失败，HTTP 400
  - `COMMON-404`：资源不存在，HTTP 404
  - `COMMON-500`：系统内部错误，HTTP 500
  - `AUTH-401`：认证失败，HTTP 401
  - `AUTH-403`：权限不足，HTTP 403
  - `AUTH-409`：账号信息已存在，HTTP 409
- 建立 `AppError`，由业务层显式返回，不用 `panic` 表达业务失败。
- 建立 requestId middleware：优先读取合法 `X-Request-Id`，否则生成新值；写入响应头、`context.Context`、结构化日志字段和统一响应体。
- 建立 recover middleware：未预期 panic 统一转换为 `COMMON-500`。
- recover middleware 在响应尚未提交时才写 `COMMON-500`；如果 handler 已写出响应头或响应体后才 panic，只记录 panic 日志，不再追加统一错误体，避免返回损坏 JSON 或混合响应。
- 建立 validation 工具，将请求体缺失、JSON 解析失败、字段校验失败映射到 `COMMON-400`。
- 建立 `PageRequest/PageResponse` 通用分页对象，保持 Java 版 1-based page、默认 size、最大 size 和派生分页元数据语义。
- 暴露基础接口：
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET /actuator/health`
  - `HEAD /actuator/health`
  - `GET /actuator/info`
  - `HEAD /actuator/info`
- 建立进程退出信号处理：首次 `SIGINT/SIGTERM` 触发 HTTP server 10 秒优雅关闭，并立即停止 signal notify，使第二次 `Ctrl+C` 恢复默认行为、可以强制退出。
- 建立 dev/test/prod 配置雏形和 `slog` 结构化日志。
- 用 `httptest` 覆盖 requestId、统一响应、错误映射、ping、echo、health、info 和 panic recover。

## 3. 非目标
- 不连接数据库，不新增 migration、sqlc query 或 repository。
- 不实现认证、授权、JWT、用户、活动、订单、支付等业务模块。
- 不实现 OpenAPI 文档生成；后续 API 契约稳定后再引入。
- 不实现 Java Actuator 的完整组件健康检查树，仅提供 Go 版最小 `health/info` 兼容入口。
- 不迁移 Spring Security、Spring MVC、Bean Validation、MDC 或 Actuator 的内部实现结构。
- 不在本次引入响应缓冲来强制把“已写出部分响应后 panic”的场景改写为 `COMMON-500`；这类场景已经越过 HTTP 提交边界，只做服务端日志兜底。

## 4. 影响范围
- 新增 Go module：`go.mod`、`go.sum`。
- 新增入口：`cmd/eventhub/main.go`。
- 新增配置：`internal/config`。
- 新增日志：`internal/platform/log`。
- 新增 HTTP 基础设施：
  - `internal/http/server.go`
  - `internal/http/router.go`
  - `internal/http/middleware/request_id.go`
  - `internal/http/middleware/recover.go`
  - `internal/http/response`
  - `internal/http/validation`
  - `internal/http/handler/system_handler.go`
- 新增错误与分页基础包：
  - `internal/apperror`
  - `internal/page`
- 新增测试覆盖对应 package。
- 本次影响 API 路径、响应字段、错误码、validation 行为、Go-only router/config/logging 取舍和测试策略，需要更新 `docs/ai/parity/java-go-parity-matrix.md`。

## 5. 领域建模
- `APIResponse[T]`
  - Go 版统一响应对象，对齐 Java `ApiResponse<T>`。
  - 字段：`code`、`message`、`data`、`requestId`、`timestamp`。
  - `timestamp` 使用 RFC3339/RFC3339Nano JSON 字符串，等价于 Java `OffsetDateTime` 的 ISO 输出语义。
- `Code`
  - Go 版错误码枚举值，承载应用层 code、默认 message、HTTP status。
  - 对齐 Java `ErrorCode`，但使用 Go 常量和方法表达。
- `AppError`
  - Go 版显式应用错误，对齐 Java `BusinessException` 的业务失败承载职责。
  - 服务层后续通过 `return nil, apperror.New(...)` 表达可预期失败；handler 负责映射响应。
- `RecoverResponseWriter`
  - recover middleware 内部使用的响应写入装饰器，用于记录当前请求是否已经调用过 `WriteHeader` 或 `Write`。
  - 只服务于 panic 后是否还能写统一错误响应的判断，不改变 handler 的正常写出行为。
- `RequestID`
  - 请求追踪 ID，对齐 Java `RequestIdFilter` 的 `X-Request-Id` 与 `requestId` 语义。
  - Go 版放入 `context.Context`，日志和响应从 context 读取。
- `PageRequest`
  - `page` 从 1 开始，默认 `1`，`size` 默认 `20`，最大 `100`。
  - `Offset()` 转换为数据库后续需要的 0-based offset。
- `PageResponse[T]`
  - 字段：`items/page/size/total/totalPages/hasNext/hasPrevious`。
  - `totalPages/hasNext/hasPrevious` 由 `PageRequest` 和 `total` 推导，不由外部直接传入。
- `PingInfo`
  - 字段：`serviceName/activeProfiles/serverTime`，对齐 Java `PingInfo`。
- `EchoRequest/EchoInfo`
  - 请求字段：`message/tag`。
  - 响应字段：`message/tag/echoedAt`。
  - 校验：`message` 必填且长度不超过 64，`tag` 可选且长度不超过 32。

## 6. API 设计
- 所有业务 API 成功响应使用 `APIResponse` 包装。

```json
{
  "code": "COMMON-000",
  "message": "成功",
  "data": {},
  "requestId": "req-123",
  "timestamp": "2026-05-30T12:00:00+08:00"
}
```

- `GET /api/v1/system/ping`
  - 请求：无。
  - 成功：HTTP 200，`code=COMMON-000`。
  - `data.serviceName` 默认 `eventhub-backend`，`data.activeProfiles` 来自 Go 配置，`data.serverTime` 为服务端时间。
- `POST /api/v1/system/echo`
  - 请求：

```json
{
  "message": "hello eventhub",
  "tag": "bootstrap"
}
```

  - 成功：HTTP 200，返回 `data.message/data.tag/data.echoedAt`。
  - JSON 语法错误或请求体缺失：HTTP 400，`COMMON-400`，`message=请求体格式不合法`，`data.body=请求体缺失或 JSON 格式错误`。
  - 字段校验失败：HTTP 400，`COMMON-400`，`message=请求体参数校验失败`，`data.<field>` 写入字段级错误。
- `GET /actuator/health`
  - 成功：HTTP 200。
  - 响应：`{"status":"UP"}`。
  - 当前不包装 `APIResponse`，保留运维探活端点的轻量语义。
- `HEAD /actuator/health`
  - 成功：HTTP 200。
  - 响应体为空，保留 `X-Request-Id` 响应头。
  - 对齐 Java 版 `SystemControllerTest#healthEndpointShouldPermitHeadRequest` 和 SecurityConfig 中 Actuator HEAD 放行语义，方便负载均衡、代理或部署平台做轻量探测。
- `GET /actuator/info`
  - 成功：HTTP 200。
  - 响应包含 `app/name/profile/version` 等基础信息。
  - 当前不包装 `APIResponse`，保留运维信息端点和业务响应的边界。
- `HEAD /actuator/info`
  - 成功：HTTP 200。
  - 响应体为空，保留 `X-Request-Id` 响应头。
  - Java 版安全配置显式放行 `HEAD /actuator/info`；Go 版同步补齐该方法契约。
- 未匹配路径或方法：
  - 路径不存在：HTTP 404，`COMMON-404`，`message=请求的资源不存在`。
  - 方法不允许：HTTP 404 或 405 不作为本次强契约；Go 版路由基础阶段先统一兜底为 `COMMON-404`，后续认证/权限引入时再对齐 Java 安全链路中部分方法的 `AUTH-401` 行为。
- 与 Java 版差异：
  - Java Actuator 由 Spring Boot 提供，Go 版先手写 `health/info` 最小端点。
  - Java validation 由 Bean Validation 抛异常后进入 `GlobalExceptionHandler`，Go 版在 handler 边界显式解码和校验。
  - Java requestId 存在 MDC，Go 版使用 `context.Context` + `slog.Attr`。

## 7. 数据设计
- 本次不新增数据库表、索引、唯一约束、migration 或 sqlc query。
- `PageRequest.Offset()` 为后续 repository/sqlc 层保留分页换算语义，但本次不访问数据库。
- 数据一致性风险不涉及持久化；响应对象中派生字段通过构造函数集中计算，避免分页元数据不一致。

## 8. 关键流程
- 正常请求流程：
  1. `request_id` middleware 读取或生成 requestId。
  2. requestId 写入响应头和 request context。
  3. `recover` middleware 兜底捕获未预期 panic。
  4. chi router 分发到 handler。
  5. handler 完成 HTTP 入参解析、validation 和响应映射。
  6. response package 从 context 读取 requestId，写出统一 JSON。
- Actuator HEAD 流程：
  1. `request_id` middleware 仍然先写入 `X-Request-Id`，保持探测请求可追踪。
  2. chi router 显式匹配 `HEAD /actuator/health` 和 `HEAD /actuator/info`。
  3. handler 只写 HTTP 200 和 JSON content type，不写响应体，避免 `httptest` 与真实 HTTP server 在 HEAD body 处理上的差异。
- `POST /echo` validation 流程：
  1. handler 调用 validation 解码 JSON。
  2. 空 body、格式错误、未知多段 JSON 均返回 `COMMON-400`。
  3. 字段规则失败返回字段级 map。
  4. 通过校验后组装 `EchoInfo` 并返回成功响应。
- panic 流程：
  1. 任意后续 handler 或 middleware panic。
  2. recover middleware 记录错误日志，带 requestId。
  3. 如果响应尚未提交，返回 HTTP 500 和 `COMMON-500`，不泄露 panic 细节。
  4. 如果响应已经通过 `WriteHeader` 或 `Write` 提交，recover middleware 不再写响应体，只保留 panic 日志；这是为了避免在已经发送的响应后追加统一错误 JSON，导致客户端收到损坏响应。
- 进程退出流程：
  1. `cmd/eventhub/main.go` 通过 `signal.NotifyContext` 监听 `SIGINT/SIGTERM`。
  2. 第一次退出信号取消 context，`server.Run` 进入最长 10 秒的 HTTP 优雅关闭窗口。
  3. 首次信号触发后立即调用 `stop()` 停止 signal notify 注册，恢复后续信号默认行为。
  4. 如果优雅关闭卡住，第二次 `Ctrl+C` 可以直接强制终止进程。
- 分层分工：
  - `handler` 只处理 HTTP 入参、校验、响应映射和基础系统数据组装。
  - 本次无业务 service/repository/database；后续业务模块仍遵守 `handler -> service -> repository -> sqlc/database`。
  - `response`、`apperror`、`page` 属于跨模块基础设施，不承载业务状态流转。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及库存、订单、支付、事务或超卖风险。
  - requestId 基于每个 HTTP 请求独立生成或复用，放在 request-scoped context 中，不使用全局可变状态，避免并发串用。
  - recover middleware 的响应提交状态记录挂在单次请求的 ResponseWriter wrapper 上，不共享全局状态。
  - 不引入缓存。
- `PageResponse` 构造时复制 `items` 切片，降低调用方后续修改切片导致响应元数据和内容不一致的风险；元素本身仍为浅拷贝。

## 10. 权限与安全
- 本次不实现认证、授权和 JWT。
- `GET /api/v1/system/ping`、`POST /api/v1/system/echo`、`GET /actuator/health`、`GET /actuator/info` 当前默认公开。
- 不新增 JWT claim，更不会把角色、邮箱、用户名、用户状态写入 JWT。
- requestId 安全规则对齐 Java：
  - 首字符必须是字母或数字。
  - 后续字符允许字母、数字、点、下划线、短横线。
  - 总长度不超过 64。
  - 不合法的外部 requestId 不透传，改由服务端重新生成。
  - recover middleware 返回统一内部错误，避免向调用方暴露堆栈或实现细节。
  - 已提交响应后 panic 只记录服务端日志，不再向客户端追加错误体，避免泄露内部错误细节或污染已发送响应。
- prod 配置默认日志级别保持 info，后续敏感配置和依赖探活细节在引入真实依赖时再细化。

## 11. 测试策略
- 单元测试：
  - `apperror`：错误码默认消息、HTTP status、`AppError` 自定义消息。
  - `page`：默认分页、offset、非法参数、总页数、上一页/下一页语义。
  - `response`：成功/失败统一响应字段、requestId 透传。
- handler / HTTP 测试：
  - `GET /api/v1/system/ping` 返回 `COMMON-000` 和 `serviceName`。
  - `POST /api/v1/system/echo` 成功回显。
  - echo 空 message、超长字段、非法 JSON 返回 `COMMON-400`。
  - requestId 复用合法请求头、拒绝非法请求头并生成新值。
  - 缺失路由返回 `COMMON-404`。
  - panic recover 返回 `COMMON-500`。
  - handler 尚未写响应时 panic：HTTP 500，`COMMON-500`。
  - handler 已写响应后 panic：保留原响应状态和内容，不追加 `COMMON-500`，但 recover 仍吞掉 panic 并记录日志。
  - `GET /actuator/health` 返回 `UP`。
  - `HEAD /actuator/health` 返回 HTTP 200、无响应体，并带 `X-Request-Id`。
  - `GET /actuator/info` 可访问并返回应用信息。
  - `HEAD /actuator/info` 返回 HTTP 200、无响应体，并带 `X-Request-Id`。
- migration / sqlc 验证：
  - 不适用，本次没有数据库变化。
- OpenAPI validate：
  - 不适用，本次不引入 OpenAPI 文件。
- Java-Go parity 验证：
  - 对照 Java `SystemControllerTest` 和基础对象测试，确认响应字段、错误码、requestId、安全规则、分页语义一致。
- 需要运行：
  - `gofmt`
  - `go test ./...`
  - `go vet ./...`

## 12. 风险与替代方案
- 风险：
  - Go 版没有 Spring Boot Actuator，`health/info` 当前只覆盖最小可用性，后续连接数据库、Redis 后需要扩展组件健康检查。
  - Go 版 validation 不使用反射标签库，当前规则手写在请求 DTO 附近，后续字段变多时需要约束组织方式。
  - `chi` 引入第三方依赖，需要通过 ADR 固化路由器选择。
- 备选方案：
  - 使用标准库 `http.ServeMux`：依赖最少，但路径参数、中间件组合、NotFound/MethodNotAllowed 定制和后续模块分组不如 chi 直接。
  - 使用 Gin/Echo/Fiber：功能完整，但框架风格更重，响应、中间件和错误处理容易被框架约定主导，不利于保持 Go 标准库兼容边界。
  - 引入 validator/viper/zerolog 等依赖：短期能力更丰富，但当前工程底座只需要少量配置、校验和日志能力，标准库足够表达。
  - recover middleware 对响应体做内存缓冲：可以在 handler 写出后 panic 时仍改写为 `COMMON-500`，但会改变 streaming/大响应语义并增加内存占用，本阶段不采用。
- 当前选择：
  - 使用 `chi` 作为轻量路由和中间件组合层。
  - 使用标准库 `encoding/json`、`log/slog`、`context`、`net/http` 实现响应、日志和服务入口。
  - 手写最小 validation，保留后续按复杂度引入验证库的空间。
  - Actuator HEAD 端点显式注册到 router，而不是依赖 `chi/middleware.GetHead` 自动把 HEAD 映射到 GET；这样可以保证 HEAD handler 不写响应体，并让契约在路由表中更直观。
  - recover middleware 使用轻量 ResponseWriter wrapper 记录提交状态；未提交时写统一错误，已提交时只记录日志。
- 后续可演进点：
  - 引入 OpenAPI 生成/校验。
  - 数据库接入后扩展 actuator health components。
  - 认证模块落地后补充 `AUTH-401/AUTH-403/AUTH-409` 的真实链路测试。
  - 在 CI 中固化 `go test ./...`、`go vet ./...`、lint 和 OpenAPI validate。
