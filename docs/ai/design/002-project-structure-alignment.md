# Go 项目结构规范化重构设计

## 1. 背景
- 当前 Go 版 EventHub 已完成 HTTP 工程底座：`cmd/eventhub/main.go`、`internal/http`、`internal/apperror`、`internal/page`、`internal/config`、`internal/platform/log` 已可运行并有基础测试。
- Java 版对应语义来自 `backend/src/main/java/com/eventhub` 下的 `common/api`、`common/exception`、`infra/logging`、`modules/system/controller`、`modules/system/service`、`modules/system/dto`、`modules/system/vo` 等分层。
- 上一阶段已经通过 `AGENTS.md`、backend-design-first skill、ADR-0005 和 parity matrix 固化长期 package layout，但运行时代码尚未真正迁移到 `internal/app`、`internal/service/system`、`internal/http/dto`、`internal/platform/idgen` 等目标位置。
- 本次是“项目结构规范化重构”，目标是把已有 HTTP foundation 代码整理到长期规范结构，不新增 auth、user、event、order、payment、database、OpenAPI、Docker 等业务或基础设施功能。

## 2. 目标
- 新增 `internal/app`，把应用启动装配和生命周期控制从 `cmd/eventhub/main.go` 收敛到 app 包，保持 main 极薄。
- 拆分 `internal/config/config.go` 为 `config.go`、`env.go`、`profile.go`，保持现有环境变量和 prod 日志最低 INFO 语义不变。
- 将 request id 从 `internal/http/requestid` 迁移到 `internal/platform/idgen`，保持 `X-Request-Id`、格式校验、context 传递和响应体 requestId 语义不变。
- 拆分 `internal/http/response`、`internal/apperror`、`internal/page` 的单文件实现，让响应模型、writer、错误码、错误类型、错误映射、分页请求和分页响应职责更清楚。
- 新增 `internal/http/dto/system_dto.go`，承载 system HTTP request/response data 对象。
- 新增 `internal/service/system` 和 `internal/platform/clock`，让 system handler 只做 HTTP decode、校验、调用 service 和响应映射。
- 补齐阶段可用的根目录工程资产：`README.md`、`Makefile`、基础 `.golangci.yml`、`configs/*.env.example`，以及 `api/openapi/.gitkeep`、`migrations/.gitkeep`。
- 保持现有 API 路径、响应字段、错误码、分页语义、requestId 行为和测试意图不变。
- 成功标准：
  - `go test ./...` 和 `go vet ./...` 通过。
  - 新增 Makefile 后 `make fmt`、`make test`、`make vet` 可用。
  - 不创建空 Go package，不实现当前阶段外的业务能力。

## 3. 非目标
- 不实现 auth/register/login/refresh/admin/user/event/order/payment/notification/audit 等业务。
- 不引入数据库依赖、migration 工具、sqlc query、repository/mysql 实现或真实持久化。
- 不创建 `internal/domain`、`internal/repository`、`internal/security` 等空 Go package。
- 不新增 OpenAPI 契约文件或生成代码；只保留 `api/openapi/.gitkeep` 作为阶段化落点。
- 不创建 Dockerfile、docker-compose.yml 或 sqlc.yaml，避免当前阶段出现不可运行或误导性的骨架。
- 不改变统一响应 JSON 字段：`code`、`message`、`data`、`requestId`、`timestamp`。
- 不改变错误码字符串：`COMMON-000`、`COMMON-400`、`COMMON-401`、`COMMON-404`、`COMMON-500`、`AUTH-401`、`AUTH-403`、`AUTH-409`。

## 4. 影响范围
- 本次实际触及的 Go package / 模块：
  - `cmd/eventhub`
  - `internal/app`
  - `internal/config`
  - `internal/platform/clock`
  - `internal/platform/idgen`
  - `internal/http/router`
  - `internal/http/middleware`
  - `internal/http/handler`
  - `internal/http/dto`
  - `internal/http/response`
  - `internal/apperror`
  - `internal/page`
  - `internal/service/system`
  - `configs`
  - `api/openapi`
  - `migrations`
  - `docs/ai`
- 本次明确不触及的运行时代码目录：
  - `internal/http/requesterror`：继续保留现有 JSON decode 和字段错误映射。
  - `internal/domain`：当前没有真实领域模型，不创建空 package。
  - `internal/repository`、`internal/repository/mysql`、`internal/repository/mysql/queries`、`internal/repository/mysql/sqlc`：当前没有数据库访问，不创建空 package。
  - `internal/security`：当前没有认证、安全上下文、JWT 或 refresh token，不创建空 package。
  - `internal/platform/db`、`internal/platform/redis`、`internal/platform/crypto`：当前没有真实基础设施能力，不创建空 package。
- DTO 边界检查：
  - 本次新增 system HTTP DTO，统一放入 `internal/http/dto`。
  - `internal/http/response` 只保留 `APIResponse` envelope 和 writer。
  - `internal/service/system` 不依赖 `internal/http/dto`，handler 负责 DTO 与 service command/result 的映射。
- 涉及 API / 表 / 缓存 / 外部接口：
  - API 路径和响应契约不变。
  - 不涉及数据库表、索引、migration、sqlc、缓存或外部接口。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 是。本次把 Java `SystemController -> SystemService -> DTO/VO` 语义映射到 Go `handler -> service/system -> dto`，并把 requestId 从 HTTP 内部迁移到 platform/idgen，属于 Go 生态化结构落地。

## 5. 领域建模
- `Application`：进程级应用装配对象，持有配置、logger 和 HTTP server；只负责 composition root，不承载业务规则。
- `Clock`：跨业务时间抽象，当前提供 `RealClock`；system service 通过它生成 `serverTime`、`echoedAt`，便于后续测试替换。
- `SystemService`：系统基础能力应用服务，对齐 Java `SystemService`，负责组装 ping、echo、health、info 的非 HTTP 数据。
- `System DTO`：`PingResponse`、`EchoRequest`、`EchoResponse`、`HealthResponse`、`InfoResponse` 等 HTTP data 契约，对齐 Java system request DTO / VO 语义，但采用 Go 的 `dto` 包命名。
- `RequestID`：请求追踪 ID 生成、校验和 context 传递能力，从 HTTP 子包上移到 `platform/idgen`，表示它是跨 HTTP middleware、response、recover 和后续日志/追踪都可复用的基础能力。
- `APIResponse`：统一响应 envelope，继续表达 Java `ApiResponse` 的 `code/message/data/requestId/timestamp` 语义。
- `AppError` / `Code`：显式应用错误和错误码映射，对齐 Java `ErrorCode`、`BusinessException`、`GlobalExceptionHandler` 的外部契约。
- `page.Request` / `page.Response[T]`：分页请求和分页响应模型，对齐 Java `PageRequest` / `PageResponse`。

## 6. API 设计
- 本次不新增或修改 API。
- 既有 API 保持：
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET /actuator/health`
  - `HEAD /actuator/health`
  - `GET /actuator/info`
  - `HEAD /actuator/info`
- 统一响应字段保持：

```json
{
  "code": "COMMON-000",
  "message": "成功",
  "data": {},
  "requestId": "req-123",
  "timestamp": "2026-05-31T20:00:00+08:00"
}
```

- 错误码 / 异常场景：
  - echo JSON 解析失败仍返回 HTTP 400、`COMMON-400`、`message=请求体格式不合法`。
  - echo 字段校验失败仍返回 HTTP 400、`COMMON-400`、`message=请求体参数校验失败`，字段错误仍放入 `data`。
  - 未匹配路由和不支持方法仍映射为 HTTP 404、`COMMON-404`、`message=请求的资源不存在`。
  - panic 未提交响应时仍映射为 HTTP 500、`COMMON-500`；已提交响应后仍只记录日志，不追加错误体。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Go 版不复制 Spring 注解、Actuator 实现和 Java `VO` 包命名；保持路径、字段、错误码和状态语义兼容。

## 7. 数据设计
- 本次不新增数据库表、字段、索引、唯一约束、migration 或 sqlc query。
- 不新增 `sqlc.yaml`，避免当前没有 schema/query 时让 `sqlc generate` 产生误导。
- `page.Request.Offset()` 和 `page.Response[T]` 拆分后保持原有计算语义，为后续 repository/sqlc 层继续提供分页换算能力。
- 数据一致性考虑：
  - 当前没有持久化和事务边界。
  - system service 只组装无状态响应数据，不维护跨请求状态。

## 8. 关键流程
- 应用启动流程：
  1. `cmd/eventhub/main.go` 调用 `app.Run()`。
  2. `internal/app.Bootstrap()` 加载 config、初始化 logger、设置 `slog.Default()`、创建 HTTP server。
  3. `internal/app.Run()` 监听 `SIGINT/SIGTERM` 并把 context 传给 HTTP server。
  4. HTTP server 优雅关闭语义保持原有 10 秒窗口和二次 Ctrl+C 强制退出能力。
- HTTP 请求流程：
  1. request id middleware 从 `idgen.HeaderRequestID` 读取或生成 requestId。
  2. requestId 写入响应头和 request context。
  3. recover middleware 通过 `idgen.RequestIDFromContext` 获取 requestId 记录 panic。
  4. handler decode/validate HTTP DTO。
  5. handler 将 DTO 映射为 service command。
  6. `internal/service/system` 组装业务无关的 system result。
  7. handler 将 service result 映射为 HTTP DTO，调用 `response.WriteSuccess` 或 `response.WriteJSON`。
- 文件迁移映射：
  - `cmd/eventhub/main.go` 的配置、日志、signal 和 server 装配迁移到 `internal/app/bootstrap.go`、`internal/app/lifecycle.go`。
  - `internal/http/requestid/requestid.go` 迁移到 `internal/platform/idgen/request_id.go`。
  - `internal/http/handler/system_handler.go` 中的 DTO 迁移到 `internal/http/dto/system_dto.go`。
  - `internal/http/handler/system_handler.go` 中的 system 数据组装迁移到 `internal/service/system/service.go`。
  - `internal/config/config.go` 拆分为 `config.go`、`env.go`、`profile.go`。
  - `internal/http/response/response.go` 拆分为 `api_response.go`、`writer.go`。
  - `internal/apperror/error.go` 拆分为 `code.go`、`error.go`、`mapper.go`。
  - `internal/page/page.go` 拆分为 `page_request.go`、`page_response.go`。
- 阶段化保留：
  - `api/openapi/.gitkeep` 和 `migrations/.gitkeep` 只表示未来落点，不声明当前已有契约或 migration。
  - 不为 auth、user、repository、security、domain 创建空 Go package。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及库存扣减、订单、支付、幂等键、事务或缓存。
- requestId 仍是 per-request context 数据，不使用全局可变状态。
- `Clock` 当前无状态；`RealClock.Now()` 每次读取系统时间，不引入并发共享状态。
- HTTP server 生命周期仍由 context 触发优雅关闭，保留原有并发行为。
- 后续如果 system service 接入 DB/Redis health components，需要在新的设计文档中补充超时、缓存和降级策略。

## 10. 权限与安全
- 本次不实现认证、授权、RBAC、JWT、refresh token 或安全上下文。
- `GET /api/v1/system/ping`、`POST /api/v1/system/echo`、`GET/HEAD /actuator/health`、`GET/HEAD /actuator/info` 的公开访问语义不变。
- requestId 校验规则保持：
  - 长度 1 到 64。
  - 首字符必须是字母或数字。
  - 后续字符允许字母、数字、点、下划线、短横线。
  - 非法外部 requestId 不透传，服务端重新生成。
- 不新增 JWT claim，也不在文档建议中把角色、邮箱、用户名、用户状态写入 JWT。

## 11. 测试策略
- 单元测试：
  - 更新 requestId 测试到 `internal/platform/idgen`。
  - 保持 `apperror`、`page`、`response` 现有测试语义。
- handler / HTTP 测试：
  - 保持现有 `httptest` 覆盖 ping、echo、validation、requestId、health/info GET/HEAD、not found、panic recover。
  - 验证结构重构后响应字段和错误码不变。
- service / repository 测试：
  - 当前新增 `internal/service/system`，系统服务逻辑简单，可通过现有 handler 测试覆盖端到端行为；如后续加入更复杂分支，再补 service 单元测试。
  - repository 不适用，本次不新增数据库访问。
- migration / sqlc 验证：
  - 不适用，本次没有 migration、schema、sqlc query 或 sqlc 配置。
- 接口验证 / OpenAPI validate：
  - 不适用，本次没有 OpenAPI 契约文件。
- Java-Go parity 验证：
  - 对照 Java `SystemController`、`SystemService`、`EchoRequest`、`PingInfo`、`EchoInfo`，确认 Go handler/service/dto 边界保持同等语义。
  - 对照 Java `ApiResponse`、`ErrorCode`、`PageRequest`、`PageResponse`，确认拆分文件不改变外部契约。
- 需要运行：
  - `gofmt -w .`
  - `go test ./...`
  - `go vet ./...`
  - `make fmt`
  - `make test`
  - `make vet`
  - 如 `.golangci.yml` 落地且本机有工具，则运行 `golangci-lint run`；若工具不可用，在实现说明和最终总结中记录。

## 12. 风险与替代方案
- 当前方案的风险：
  - 文件移动较多，主要风险是 import 更新遗漏或测试包路径未同步。
  - system service 抽出后如果直接返回 HTTP DTO，会破坏 DTO 边界；本次通过 service result 类型规避。
  - `internal/app` 引入后，若后续继续在 router/server 中散落装配逻辑，composition root 边界仍需持续收敛。
  - `.golangci.yml` 落地后，如果本机没有 `golangci-lint`，只能记录为暂不可运行。
- 备选方案：
  - 方案 A：只更新文档，不移动运行时代码。
  - 方案 B：一次性创建完整长期目录和空 Go package。
  - 方案 C：把 system DTO 继续留在 handler，等业务模块更多后再统一迁移。
  - 方案 D：让 service 直接复用 `internal/http/dto`，减少映射代码。
- 为什么不选备选方案：
  - 不选方案 A：当前目标是实际结构重构，继续停留在规则层无法降低后续迁移成本。
  - 不选方案 B：空 Go package 会制造无意义编译单元，违背阶段化落地原则。
  - 不选方案 C：system 是当前唯一 HTTP 模块，正好适合先把 DTO 落点验证清楚。
  - 不选方案 D：service 依赖 HTTP DTO 会破坏 handler/service 边界，不利于后续业务复用和测试。
- 后续可演进点：
  - auth/user/event/order 等业务开始后，再按规范补齐 `domain`、`service`、`repository`、`security` 等真实 package。
  - 引入数据库后再新增 migration、sqlc.yaml、repository/mysql 和 migration/sqlc 验证命令。
  - 引入 OpenAPI 后再创建 `api/openapi/eventhub.yaml` 和生成代码目录。
  - app/router/server 的依赖注入可以继续从“默认装配”演进到更显式的 dependency struct，以便集成测试替换 service。
