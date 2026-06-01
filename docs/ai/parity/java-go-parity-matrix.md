# Java-Go Parity Matrix

本文记录 Go 版 EventHub 与 Java 版 EventHub 在业务语义、接口契约、错误码、数据库模型、测试策略和文档沉淀方式上的对齐状态。

Java 版参考项目：

```text
/Users/xinnz/Library/Mobile Documents/com~apple~CloudDocs/Code/Java/eventhub
```

## 状态说明

- `已对齐`：Go 版已经建立对应规则、文档或实现。
- `已决策`：Go 版已经通过设计文档或 ADR 固化方向，但当前阶段可能尚未完成全部代码落地。
- `规则已初始化`：Go 版已写入约束，但尚未有业务代码可验证。
- `待迁移`：Java 版已有能力，Go 版尚未实现。
- `待决策`：Go 版需要 ADR 或设计文档明确技术选择。
- `不适用`：Java 版实现细节不直接迁移到 Go。

## 对齐矩阵

| 领域 | Java 版来源 | Go 版目标 | 当前状态 | 说明 |
| --- | --- | --- | --- | --- |
| AI 协作规则 | `AGENTS.md` | `AGENTS.md` | 已对齐 | 保留设计优先、文档沉淀、固定 7 项总结，补充 Go 分层、质量门禁和 parity matrix 触发条件/记录字段。 |
| Agent skill | `.agents/skills/backend-design-first/SKILL.md` | `.agents/skills/backend-design-first/SKILL.md` | 已对齐 | 改写为 Go port、Java-Go parity、sqlc/database 和 Go 验证命令语境；已补充独立 parity 文档步骤，明确触发条件和记录字段。 |
| Codex 配置目录 | `.codex/config.toml` | `.codex/config.toml` | 已对齐 | Go 版沿用 Java 版最小项目级配置：`model`、reasoning effort、personality、开发期 approval policy 和 sandbox mode。 |
| 设计模板 | `docs/templates/design-template.md` | `docs/templates/design-template.md` | 已对齐 | 沿用 12 个小节，补充 Go package、sqlc、migration、OpenAPI validate 和 parity 验证。 |
| 实现说明模板 | `docs/templates/implementation-note-template.md` | `docs/templates/implementation-note-template.md` | 已对齐 | 沿用 7 个小节，补充 Go 质量门禁和 Java-Go parity。 |
| ADR 模板 | `docs/templates/adr-template.md` | `docs/templates/adr-template.md` | 已对齐 | 沿用 Java 版大纲，补充 Go 生态取舍说明。 |
| docs/ai 目录 | `docs/ai/design`、`implementation`、`adr` | 同名目录加 `parity` | 已对齐 | Go 版增加 `parity`，用于持续记录双端差异；README 已补齐 parity 的定位、触发条件、记录字段、状态值和与其他文档的关系。 |
| 工程纪律 ADR | 多份 Java ADR | `docs/ai/adr/0001-go-port-engineering-discipline.md` | 已对齐 | 明确 Go 版长期迁移纪律。 |
| 分层边界 | Java `controller / service / mapper / domain` | Go `handler -> service -> repository -> sqlc/database` | 已决策 | system 基础模块已落地 `handler -> service`；repository/sqlc 边界待数据库阶段迁移，后续业务代码必须按此边界实现。 |
| Service contract 文件边界 | Java Service 类、request DTO、VO 返回对象 | Go `internal/service/<domain>/service.go`、`command.go`、`query.go`、`result.go`、use case 文件 | 已对齐 | Go 版不逐字照搬 Java 单 Service 类结构；service 输入输出在同 package 内按 Command / Query / Result 拆分，业务方法按 use case 拆文件。system 样板已拆为 `service.go`、`command.go`、`result.go`、`ping.go`、`echo.go`、`actuator.go`；无 Query 时不创建空 `query.go`。参考 `docs/ai/design/004-service-contract-boundary.md`、`docs/ai/implementation/004-service-contract-boundary.md`、`docs/ai/adr/0005-go-project-package-layout.md`。 |
| Go package layout / 项目目录结构 | Java Controller / Service / Mapper / Entity / Config / Security 等分层 | Go `cmd` + `internal/app` + `internal/http` + `internal/service` + `internal/repository` + `internal/domain` + `internal/platform` + `internal/security` | 已对齐 | Go 版不逐行复刻 Spring Boot，而是用 Go package、`internal`、constructor injection、显式 router 和 repository interface 表达同等边界；当前已落地 `internal/app`、`internal/http/handler/system`、`internal/http/dto/system`、`internal/service/system`、`internal/platform/clock`、`internal/platform/idgen`，仍不为未开始业务创建空 Go package。参考 `docs/ai/design/002-project-structure-alignment.md`、`docs/ai/design/005-http-module-package-boundary.md`、`docs/ai/adr/0005-go-project-package-layout.md`、`docs/ai/implementation/002-project-structure-alignment.md`、`docs/ai/implementation/005-http-module-package-boundary.md`。 |
| HTTP handler / DTO 模块化组织 | Java Controller、`dto/request`、VO / response 对象按业务模块组织 | Go `internal/http/handler/<module>`、`internal/http/dto/<module>` | 已对齐 | Go 版保持横向分层，但在 HTTP handler/dto 内按模块拆子包；system 已迁移为 `handler/system` 与 `dto/system` 样板，后续 auth/user/event/order 默认沿用。参考 `docs/ai/design/005-http-module-package-boundary.md`、`docs/ai/implementation/005-http-module-package-boundary.md`、`docs/ai/adr/0005-go-project-package-layout.md`、`docs/ai/adr/0006-http-dto-vs-vo-boundary.md`。 |
| HTTP DTO / Java VO 对照 | Java Request DTO、Response DTO、VO 命名习惯；例如 `modules/system/dto/request/EchoRequest`、`modules/system/vo/PingInfo`、`modules/system/vo/EchoInfo`、`modules/auth/vo/*` | Go `internal/http/dto/<module>` 承载 HTTP request/response；不设置 `internal/http/vo`；`internal/http/response` 只承载 `APIResponse` envelope 和 writer；domain value object 放 `internal/domain` | 已对齐 | Go 版不逐字照搬 Java VO 命名；当前没有 `internal/http/vo`、`internal/**/vo`、`*VO` struct 或误放在 `internal/http/response` 的具体业务 DTO，system `EchoRequest`、`PingResponse`、`EchoResponse`、`HealthResponse`、`InfoResponse` 已位于 `internal/http/dto/system`，service 不依赖 HTTP DTO。Java 中 View Object 语义在 Go 版归入 HTTP DTO，DDD Value Object 归入 domain。参考 `docs/ai/design/003-http-dto-boundary.md`、`docs/ai/design/005-http-module-package-boundary.md`、`docs/ai/adr/0005-go-project-package-layout.md`、`docs/ai/adr/0006-http-dto-vs-vo-boundary.md`、`docs/ai/implementation/003-http-dto-boundary.md`、`docs/ai/implementation/005-http-module-package-boundary.md`。 |
| 业务错误 | Java `BusinessException` / `ErrorCode` | Go 显式错误类型 / 错误码映射 | 待迁移 | 未来实现时对齐 Java 错误码和响应结构，不用 `panic` 表达业务错误。 |
| API 契约 | Java controller / OpenAPI / MockMvc 测试 | Go handler / OpenAPI / HTTP 测试 | 待迁移 | 后续每个 API 设计需对照 Java 路径、字段、状态码和错误码。 |
| 数据库模型 | Java migration / mapper / entity | Go migration / sqlc / repository | 待迁移 | 后续表、字段、索引、唯一约束和状态值需对齐 Java 版。 |
| JWT claim 边界 | Java auth ADR 和实现 | Go auth token 设计 | 规则已初始化 | Go 版禁止把角色、邮箱、用户名、用户状态写入 JWT。 |
| 测试策略 | Java unit / integration / MockMvc / H2 | Go unit / service / repository / API / migration 测试 | 规则已初始化 | Go module 已建立，HTTP 基础测试已使用 `httptest` 覆盖；后续数据库模块再补 migration/sqlc/repository 测试。 |
| 质量门禁 | Java Maven test、OpenAPI、profile 测试 | Go `gofmt`、`go test ./...`、`go vet ./...`、lint、sqlc、migration、OpenAPI validate | 规则已初始化 | 当前已可运行 `gofmt`、`go test ./...`、`go vet ./...`；lint、sqlc、migration、OpenAPI validate 待对应工具和契约引入。 |
| Spring Boot / Maven | Java 基础工程 | Go module / Makefile / CI | 规则已初始化 | 已初始化 `go.mod`、`cmd/eventhub` 和基础 Makefile，不迁移 Spring/Maven 结构；CI 后续补齐。参考 `docs/ai/design/001-http-foundation.md`、`docs/ai/implementation/001-http-foundation.md`、`docs/ai/design/002-project-structure-alignment.md`。 |
| MyBatis | Java mapper 持久化边界 | sqlc/database + repository | 待决策 | 已在规则中指定 sqlc/database 边界，具体 sqlc 配置待工程初始化。 |
| H2 测试 profile | Java test profile | Go migration / test database strategy | 待决策 | Go 版不默认采用 H2，需要在数据库测试设计中另行决策。 |
| HTTP 工程底座 | Java `EventhubApplication`、Spring Boot Web 基础工程 | `cmd/eventhub/main.go`、`internal/http/server.go`、`internal/http/router.go` | 已对齐 | Go 版使用标准库 HTTP server + chi router 建立最小可运行服务，不迁移 Spring Boot 容器；首次退出信号触发优雅关闭并立即释放 signal notify，保留二次 Ctrl+C 强制退出语义。参考 `docs/ai/adr/0002-web-router-chi.md`。 |
| Web router | Java Spring MVC annotation routing | Go `github.com/go-chi/chi/v5` | 已对齐 | 对齐路径、HTTP method 和 middleware 语义，不复制 Spring MVC 注解模型。参考 `docs/ai/adr/0002-web-router-chi.md`。 |
| 统一响应体 | Java `common/api/ApiResponse.java` | Go `internal/http/response.APIResponse` | 已对齐 | 字段保持 `code/message/data/requestId/timestamp`；timestamp 使用 Go `time.Time` JSON ISO 格式。参考 `docs/ai/adr/0003-error-response-contract.md`。 |
| 错误码与业务错误 | Java `ErrorCode`、`BusinessException`、`GlobalExceptionHandler` | Go `internal/apperror.Code`、`AppError`、`internal/http/response` | 已对齐 | 初始化 `COMMON-000/400/401/404/500` 和 `AUTH-401/403/409`；业务失败使用显式错误返回，不用 panic。参考 `docs/ai/adr/0003-error-response-contract.md`。 |
| 参数校验错误 | Java Bean Validation + `GlobalExceptionHandler` | Go `internal/http/validation` + handler 显式校验 | 已对齐 | JSON 格式错误和字段校验失败统一映射为 `COMMON-400`，字段错误通过 `data` 返回；Go 版先手写最小校验。 |
| requestId | Java `infra/logging/RequestIdFilter.java`、Logback MDC | Go `internal/platform/idgen`、`internal/http/middleware/request_id.go`、`slog` 字段 | 已对齐 | 复用合法 `X-Request-Id`，非法或缺失时生成新值；写入响应头、context、日志字段和统一响应体。request id 从 HTTP 子包上移到 platform/idgen，作为跨 middleware、recover、response、日志的基础能力。参考 `docs/ai/adr/0004-config-and-logging.md`、`docs/ai/design/002-project-structure-alignment.md`。 |
| panic / 未预期异常 | Java `GlobalExceptionHandler#handleUnexpectedException` | Go `internal/http/middleware/recover.go` | 已对齐 | 未预期 panic 统一记录日志；响应未提交时返回 `COMMON-500`，响应已提交时不再追加错误体，避免损坏客户端响应；业务错误不通过 panic 表达。 |
| system ping | Java `SystemController#ping`、`SystemService#ping`、`PingInfo` | Go `internal/http/handler/system.Handler.Ping`、`internal/service/system.Service.Ping`、`internal/http/dto/system.PingResponse` | 已对齐 | `GET /api/v1/system/ping` 返回统一响应和 `serviceName/activeProfiles/serverTime`；handler 只做 HTTP 映射，service 负责数据组装。 |
| system echo | Java `SystemController#echo`、`EchoRequest`、`EchoInfo` | Go `internal/http/handler/system.Handler.Echo`、`internal/service/system.Service.Echo`、`internal/http/dto/system.EchoRequest/EchoResponse` | 已对齐 | `POST /api/v1/system/echo` 校验 `message/tag` 并回显 `message/tag/echoedAt`；HTTP DTO 不泄漏到 service。 |
| Actuator health/info | Java Spring Boot Actuator `/actuator/health`、`/actuator/info`，`SystemControllerTest#healthEndpointShouldPermitHeadRequest`，`SecurityConfig` Actuator GET/HEAD 放行 | Go `internal/http/handler/system.Handler.Health/HealthHead/Info/InfoHead`、`internal/service/system.Service.Health/Info`、`internal/http/router.go` | 已对齐 | Go 版先实现无数据库依赖的最小 GET health/info，并显式补齐 `HEAD /actuator/health`、`HEAD /actuator/info`；HEAD 返回 HTTP 200、保留 requestId 头且不写响应体，后续接入 DB/Redis 后补 components 语义。 |
| 分页契约 | Java `PageRequest`、`PageResponse` | Go `internal/page` | 已对齐 | 保持 1-based page、默认 1/20、最大 100、offset、totalPages、hasNext、hasPrevious 规则。 |
| 配置与日志 | Java `application*.yml`、Logback、MDC | Go `internal/config`、`internal/platform/log`、`slog` | 已对齐 | dev/test/prod 雏形和 JSON 结构化日志已初始化；`internal/config` 已按 config/env/profile 拆分且保持环境变量兼容，后续数据库、Redis、JWT 配置接入时继续扩展。参考 `docs/ai/adr/0004-config-and-logging.md`、`docs/ai/design/002-project-structure-alignment.md`。 |
| HTTP 基础测试 | Java `SystemControllerTest`、`ApiResponseTest`、`PageRequestTest`、`PageResponseTest`、`BusinessExceptionTest` | Go `internal/http/*_test.go`、`internal/apperror/error_test.go`、`internal/page/page_test.go` | 已对齐 | 使用 `httptest` 覆盖 requestId、统一响应、错误映射、ping、echo、health/info GET、health/info HEAD、panic recover 未提交/已提交响应场景和分页语义。 |

## 后续维护规则

1. 每迁移一个 Java 业务模块，必须新增或更新对应矩阵行。
2. 如果 Go 版刻意偏离 Java 版实现方式，但保持业务语义一致，需要在设计文档中说明。
3. 如果 Go 版无法保持接口或错误码兼容，必须新增 ADR 或在设计文档中写明理由。
4. 矩阵只记录对齐状态和索引，详细设计仍放在 `docs/ai/design/`、`docs/ai/implementation/` 和 `docs/ai/adr/`。
