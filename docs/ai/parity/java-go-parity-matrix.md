# Java-Go Parity Matrix

本文记录 Go 版 EventHub 与 Java 版 EventHub 在业务语义、接口契约、错误码、数据库模型、测试策略和工程文档上的对齐状态。

Java 版参考项目：

```text
/Users/xinnz/Library/Mobile Documents/com~apple~CloudDocs/Code/Java/eventhub
```

最近核验日期：2026-06-22。

本矩阵只做领域级索引；接口字段、错误场景和流程细节放入专题 parity 文档、design、implementation note 或 ADR。

## 状态说明

- `已对齐`：当前 Go 代码、文档或测试已经支撑本阶段对应语义。
- `已决策`：Go 版已经通过设计文档或 ADR 固化方向，但当前阶段尚未完成全部代码落地。
- `规则已初始化`：Go 版已写入约束，但尚未有业务代码可验证。
- `待迁移`：Java 版已有生产能力，Go 版尚未实现。
- `待决策`：Go 版需要 ADR 或设计文档明确技术选择。
- `不适用`：Java 版实现细节不直接迁移到 Go。

## 对齐矩阵

| 领域 | Java 版来源 | Go 版目标 | 当前状态 | 差异 / 索引 |
| --- | --- | --- | --- | --- |
| 协作、文档与命名规则 | `AGENTS.md`、Java `docs/ai/*`、`docs/templates/*` | `AGENTS.md`、`.agents/skills/backend-design-first/SKILL.md`、`docs/templates/*`、`docs/ai/*` | 已对齐 | Go 版保留 design-first、implementation note、ADR、parity matrix、质量门禁和固定总结格式；文档命名使用 Go 仓库三位迭代序号和四位 ADR 序号，不迁移 Java 日期前缀。参考 `docs/ai/design/000-go-port-project-rules.md`、`006-parity-matrix-audit.md`、`017-doc-naming-rule-alignment.md`。 |
| 工程入口、应用装配与 HTTP 基线 | `EventhubApplication.java`、Spring Boot Web、Java `pom.xml` | `go.mod`、`cmd/eventhub/main.go`、`internal/app`、`internal/http/{router,server}.go`、`Makefile` | 已对齐 | Go 使用标准库 HTTP server + chi，不迁移 Spring/Maven 容器模型；`Bootstrap(ctx)`、provider、`Application` 生命周期和 `main` 错误记录已落地。参考 ADR-0002、design/implementation 001、011。 |
| 分层、package layout 与依赖边界 | Java controller/service/mapper/entity/config/security 分层 | Go `handler -> service -> repository -> sqlc/database`，`internal/app`、`platform`、`security` | 已对齐 | 已落地 system/auth/user、repository/mysql、security/jwt/refresh/password、platform/db/redis/log/idgen；未开始模块不创建空 package。handler 依赖具体 service，repository interface 保留为持久化边界，router 只注册路由和最终 middleware 函数；auth middleware 已从 wrapper 对象收敛为 provider 装配的 `Authenticate` 函数。参考 design/implementation 002、005、009、010、012、025，ADR-0005、0006。 |
| 统一响应、错误码与校验映射 | `ApiResponse.java`、`ErrorCode.java`、`BusinessException.java`、`GlobalExceptionHandler.java` | `internal/http/response`、`internal/apperror`、`internal/http/validation`、`internal/http/middleware/recover.go` | 已对齐 | 响应字段为 `code/message/data/requestId/timestamp`；当前错误码覆盖 `COMMON-000/400/401/404/500`、`AUTH-401/403/409`。JSON 格式、字段校验、未匹配路由、panic 和 auth/security 失败均映射统一 envelope。参考 ADR-0003。 |
| requestId、配置、日志与基础设施配置 | `RequestIdFilter.java`、`application*.yml`、Logback MDC、`AuthTokenProperties.java` | `internal/platform/idgen`、`internal/http/middleware/request_id.go`、`internal/config`、`internal/platform/{log,db,redis}`、`configs/*.env.example` | 已对齐 | 合法 `X-Request-Id` 复用，非法或缺失时重建；写入响应头、context、响应体和 `slog`。Go 配置覆盖 app/env/port/version/log、MySQL、Redis、OpenAPI、access/refresh token。参考 design/implementation 008、016。 |
| system 与 actuator API | `SystemController.java`、`SystemService.java`、`EchoRequest.java`、`PingInfo.java`、`EchoInfo.java`、Actuator 配置与测试 | `internal/http/handler/system`、`internal/http/dto/system`、`internal/service/system`、`internal/http/router.go` | 已对齐 | 已实现 `GET /api/v1/system/ping`、`POST /api/v1/system/echo`、`GET/HEAD /actuator/health`、`GET/HEAD /actuator/info`。Go 未复刻 Spring Security 对未知路由的默认认证拦截，统一返回 `COMMON-404`。参考 design/implementation 001。 |
| 分页契约 | `PageRequest.java`、`PageResponse.java` | `internal/page` | 已对齐 | 保持 1-based page、默认 1/20、最大 100、offset、`totalPages`、`hasNext`、`hasPrevious` 和 items 浅拷贝语义。参考 design/implementation 001。 |
| OpenAPI / Swagger | `OpenApiConfig.java`、Controller `@Operation/@Tag`、Springdoc dev/test/prod 开关、`OpenApiProductionSecurityTest`、Spring Security ADMIN 角色约束 | `api/openapi/eventhub.yaml`、`api/openapi/oapi-codegen.yaml`、`api/openapi/openapi_policy_test.go`、`api/openapi/spec.go`、`api/openapi/gen/eventhub.gen.go`、`internal/http/{router_contract_test.go,openapi_contract_test.go}`、`internal/http/handler/openapi`、`Makefile` | 已对齐 | Go 选择 spec-first，以 `api/openapi/eventhub.yaml` 作为契约源，`oapi-codegen` 通过 `api/openapi/oapi-codegen.yaml` 生成类型和 chi server interface；dev/test 默认启用 `/openapi.yaml` 和 `/swagger/*`，prod 默认关闭并返回 `COMMON-404`。Go 不复刻 Springdoc 注解扫描和 `/v3/api-docs`/`/swagger-ui.html` 路径；Go test policy 强制 operationId、summary/description、tags、业务 JSON 响应、2xx 最外层 `ApiResponse` envelope、非 2xx 集中引用 `components.responses`、组件错误响应统一使用 `ErrorResponse`、`ErrorResponse` 顶层复用 `ApiResponse`、admin `BearerAuth` 与精确 `x-required-roles: [ADMIN]`，并阻止非 admin operation 误标 `ADMIN`；`internal/http` 额外通过 `chi.Walk` 对比运行时 router 与 OpenAPI `paths`/`methods`，并用 `openapi3filter.ValidateResponse` 校验少量关键真实响应的 status、media type 和 body schema，覆盖 `/api/**` 与 `/actuator/**`，排除受 `OPENAPI_ENABLED` 控制的文档路由。当前审计见 `docs/ai/parity/current-auth-contract-checklist.md`。参考 design/implementation 015、018、021、022、023、024，ADR-0018、0019。 |
| 数据库模型、migration 与 sqlc | Flyway `V1__init_backend_foundation.sql`、`V2__stage_1_auth_jwt_rbac.sql`、`V3__create_auth_sessions.sql`、MyBatis mapper/entity | `migrations/000001_system_bootstrap.*.sql`、`000002_auth_schema.*.sql`、`sqlc.yaml`、`internal/repository`、`internal/repository/mysql/{queries,sqlc}`、`internal/platform/db` | 已对齐 | users/roles/user_roles/auth_sessions 表、字段、唯一约束、外键、索引、状态字符串和 seed 已对齐；Go 将 Java V2/V3 合并为从空库起步的 `000002_auth_schema`。参考 design/implementation 007，ADR-0007、0008、0010。 |
| 数据库与迁移测试策略 | Java H2 test profile、mapper tests、auth session concurrency tests | `internal/repository/mysql/mysql_repository_integration_test.go`、`internal/platform/db/errors_test.go` | 已对齐 | Go 刻意不采用 H2，使用 Testcontainers MySQL 验证 migration up/down、seed、unique constraint、repository 事务上下文和 auth session 条件更新；Docker/provider 不可用时 skip。测试覆盖对照见 `docs/ai/parity/test-coverage-comparison.md`。参考 ADR-0009、design/implementation 018。 |
| Redis 认证一致性边界 | Java Redis 配置、Actuator redis health、auth session MySQL 权威记录、Java compose Redis | `internal/platform/redis`、`internal/config`、`internal/app/providers/platform.go`、`docker-compose.yml`、`configs/*.env.example` | 已决策 | Redis client 和 compose 服务已落地，配置 `EVENTHUB_REDIS_ADDR` 时启动期 ping；当前不参与认证强一致，refresh/logout/禁用用户判断仍以 MySQL 为准。参考 design/implementation 016。 |
| Auth、当前用户与管理员用户 API | `AuthController.java`、`UserController.java`、`AdminUserController.java`、auth DTO/VO、`AuthServiceImpl.java`、`SecurityConfig.java`、auth integration tests | `internal/http/handler/{auth,user}`、`internal/http/dto/{auth,user}`、`internal/service/{auth,user}`、`internal/http/middleware/{auth,rbac}.go`、`internal/security/*`、`internal/http/auth_integration_test.go` | 已对齐 | Go 已实现 register/login/refresh/logout、`GET /api/v1/me`、`GET /api/v1/admin/users`、`PATCH /api/v1/admin/users/{userId}/status`；字段、错误码、RBAC、JWT claim、refresh token 轮换和 session 语义见 `docs/ai/parity/java-auth-api-contract.md`，当前 P0/P1/P2 审计见 `docs/ai/parity/current-auth-contract-checklist.md`。认证 middleware 仍对齐 Java `JwtAuthenticationFilter` + `AuthenticatedPrincipalService` 的主体加载语义，但 Go 内部由 provider 装配最终 `Authenticate` 函数交给 router 使用。参考 design/implementation 008、013、014、018、025，ADR-0011 至 0017。 |
| 认证错误与安全响应 | `AuthException.java`、`RestAuthenticationEntryPoint.java`、`RestAccessDeniedHandler.java`、`SecurityErrorResponseWriter.java` | `internal/apperror`、`internal/http/handler/{auth,user}`、`internal/http/middleware/{auth,rbac}.go` | 已对齐 | 重复账号、账号密码错误、用户禁用、缺失/过期/篡改 access token、无效/过期/重放 refresh token、普通用户访问管理员接口、更新不存在用户均映射到 Java 对齐的 HTTP 状态、错误码和核心消息。细节见 `docs/ai/parity/java-auth-api-contract.md` 和 `docs/ai/parity/current-auth-contract-checklist.md`。 |
| 当前阶段 parity audit | Java 当前 controller、DTO、VO、ErrorCode、migration、mapper、安全配置、OpenAPI 配置和测试目录 | `docs/ai/design/018-current-parity-audit.md`、`docs/ai/implementation/018-current-parity-audit.md`、`docs/ai/parity/current-auth-contract-checklist.md`、`docs/ai/parity/test-coverage-comparison.md`、`internal/http/auth_integration_test.go` | 已对齐 | 2026-06-06 审计未发现生产代码 P0 差异；P1 单条 auth/admin smoke flow 已通过 `TestAuthParitySmokeFlow` 补齐；P2 有意差异集中在 Go spec-first OpenAPI 路径、prod 文档关闭状态码和 sqlc offset 防护。 |
| 容器化、部署配置与质量门禁 | Java `backend/Dockerfile`、`docker-compose.yml`、`application-dev/prod/test.yml`、prod OpenAPI hardening ADR | Go `Dockerfile`、`docker-compose.yml`、`configs/*.env.example`、`README.md`、`Makefile`、`.golangci.yml`、`.github/workflows/ci.yml` | 已对齐 | Go 多阶段 Dockerfile、MySQL/Redis/migrate/app compose、healthcheck、prod 默认关闭 OpenAPI、显式 migration job、`make quality`/`make quality-check`、固定 golangci-lint v2 版本、v2 配置格式、本机版本探测与 Docker fallback、GitHub Actions CI 已落地；CI 通过 `make quality-check`、`make generated-check` 和 Docker job 复用 Makefile 校验 fmt/vet/test/lint、sqlc/OpenAPI 生成物漂移和 Docker build，并已将官方 `actions/checkout` / `actions/setup-go` 升级到 Node 24 兼容主版本，不做发布部署或镜像推送。参考 design/implementation 016、019、020，ADR-0020、0021、0022。 |
| 活动、场次、票种、库存、订单、支付、通知、审计 | Java `docs/roadmap/stage-2` 至 `stage-7`；当前 Java production code 尚未落地 | Go 后续 event/order/payment/notification/audit 相关 domain/service/repository/handler | 待决策 | 双端当前 production code 都未进入这些业务模块；Go 后续以 Java roadmap 或未来 Java 实现为语义来源，重点补库存扣减、幂等、订单状态机、支付回调、通知和操作日志。 |

## 后续维护规则

1. 每迁移一个 Java 业务模块，必须新增或更新对应矩阵行。
2. 主矩阵只保留领域级状态、差异和索引；接口字段、错误场景、测试断言写入专题 parity 文档。
3. 如果 Go 版刻意偏离 Java 实现方式但保持业务语义一致，需要在设计文档或 ADR 中说明，并由矩阵索引。
4. 如果 Go 版无法保持接口或错误码兼容，必须新增 ADR 或在设计文档中写明理由。
