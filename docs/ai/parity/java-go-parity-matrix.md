# Java-Go Parity Matrix

本文记录 Go 版 EventHub 与 Java 版 EventHub 在业务语义、接口契约、错误码、数据库模型、测试策略和文档沉淀方式上的对齐状态。

Java 版参考项目：

```text
/Users/xinnz/Library/Mobile Documents/com~apple~CloudDocs/Code/Java/eventhub
```

最近核验日期：2026-06-02。

本矩阵只做对齐索引，不替代设计文档、implementation note 或 ADR。详细背景统一放在 `docs/ai/design/`、`docs/ai/implementation/` 和 `docs/ai/adr/`。

## 状态说明

- `已对齐`：当前 Go 代码、文档或测试已经能支撑本阶段对应语义。
- `已决策`：Go 版已经通过设计文档或 ADR 固化方向，但当前阶段可能尚未完成全部代码落地。
- `规则已初始化`：Go 版已写入约束，但尚未有业务代码可验证。
- `待迁移`：Java 版已有能力，Go 版尚未实现。
- `待决策`：Go 版需要 ADR 或设计文档明确技术选择。
- `不适用`：Java 版实现细节不直接迁移到 Go。

## 对齐矩阵

| 领域 | Java 版来源 | Go 版目标 | 当前状态 | 说明 |
| --- | --- | --- | --- | --- |
| 协作与文档基线 | `AGENTS.md`、`.agents/skills/backend-design-first/SKILL.md`、`docs/templates/*`、Java `docs/ai/*` | `AGENTS.md`、`.agents/skills/backend-design-first/SKILL.md`、`docs/templates/*`、`docs/ai/*`、`.codex/config.toml` | 已对齐 | Go 版已建立设计优先、implementation note、ADR、parity matrix、质量门禁和固定总结结构；本次审计见 `docs/ai/design/006-parity-matrix-audit.md`、`docs/ai/implementation/006-parity-matrix-audit.md`。 |
| 工程入口与 HTTP 运行基线 | `backend/src/main/java/com/eventhub/EventhubApplication.java`、Spring Boot Web、`backend/pom.xml` | `go.mod`、`cmd/eventhub/main.go`、`internal/app`、`internal/http/server.go`、`internal/http/router.go`、`Makefile` | 已对齐 | Go 版使用标准库 HTTP server + chi，不迁移 Spring/Maven 容器模型；已具备最小启动、路由、优雅关闭、`go test`/`go vet`/Makefile 基线。参考 ADR-0002。 |
| 分层与 package layout | Java `controller / service / mapper / entity / config / security` | Go `handler -> service -> repository -> sqlc/database`，以及 `internal/app`、`internal/platform`、`internal/security` 规则 | 已决策 | 当前已落地 `internal/app`、`internal/http/handler/system`、`internal/http/dto/system`、`internal/service/system`、`internal/platform/{clock,idgen,log}`；未开始的 `domain/repository/security` 不创建空 Go package。参考 design-002/005、ADR-0005。 |
| HTTP handler、DTO 与 service contract 边界 | Java Controller、`dto/request`、`vo`、Service 类 | `internal/http/handler/<module>`、`internal/http/dto/<module>`、`internal/service/<domain>/{service,command,query,result,usecase}.go` | 已对齐 | system 样板已落地；Go 版不设置 `internal/http/vo`，HTTP DTO 不进入 service，service Command/Result 不带 JSON tag。参考 design-003/004/005、ADR-0005/0006。 |
| 统一响应、错误码与异常映射 | `common/api/ApiResponse.java`、`ErrorCode.java`、`BusinessException.java`、`GlobalExceptionHandler.java` | `internal/http/response`、`internal/apperror`、`internal/http/validation`、`internal/http/middleware/recover.go` | 已对齐 | 字段保持 `code/message/data/requestId/timestamp`；已初始化 `COMMON-000/400/401/404/500` 和 `AUTH-401/403/409`；JSON/字段校验映射 `COMMON-400`，未预期 panic 映射 `COMMON-500`。auth 专属失败流程仍随 auth 模块迁移。 |
| requestId、配置与结构化日志 | `infra/logging/RequestIdFilter.java`、`application*.yml`、Logback MDC | `internal/platform/idgen`、`internal/http/middleware/request_id.go`、`internal/config`、`internal/platform/log`、`configs/*.env.example` | 已对齐 | 合法 `X-Request-Id` 复用，非法或缺失时重建；写入响应头、context、统一响应体和 `slog` 字段。Go 当前只覆盖 app/env/port/version/log，数据库、Redis、JWT 配置待后续迁移。 |
| system 与 actuator API | `modules/system/controller/SystemController.java`、`SystemService.java`、`EchoRequest.java`、`PingInfo.java`、`EchoInfo.java`、Actuator 配置与测试 | `internal/http/handler/system`、`internal/http/dto/system`、`internal/service/system`、`internal/http/router.go` | 已对齐 | 已实现 `GET /api/v1/system/ping`、`POST /api/v1/system/echo`、`GET/HEAD /actuator/health`、`GET/HEAD /actuator/info`。Go 暂无 Spring Security 默认认证链，未支持方法当前由 router 映射为 `COMMON-404`，待 auth/security 阶段再对齐 Java 的 `AUTH-401` 场景。 |
| 分页契约 | `common/api/PageRequest.java`、`PageResponse.java` | `internal/page` | 已对齐 | 保持 1-based page、默认 1/20、最大 100、offset、`totalPages`、`hasNext`、`hasPrevious` 和 items 浅拷贝语义。 |
| 当前基础测试与质量门禁 | Java `ApiResponseTest`、`PageRequestTest`、`PageResponseTest`、`BusinessExceptionTest`、`SystemControllerTest` | Go `internal/http/*_test.go`、`internal/apperror/error_test.go`、`internal/page/page_test.go`、`internal/platform/idgen/request_id_test.go`、`.golangci.yml` | 已对齐 | 当前 Go foundation 已用 `httptest` 和单元测试覆盖统一响应、错误码、requestId、system、actuator、recover、分页；`gofmt/go test/go vet/make test/make vet` 可运行，lint 配置已存在但工具安装依赖环境。 |
| OpenAPI / Swagger | `infra/openapi/OpenApiConfig.java`、Controller `@Operation/@Tag`、`OpenApiProductionSecurityTest`、Springdoc 配置 | `api/openapi/.gitkeep`、未来 `api/openapi/eventhub.yaml` | 待迁移 | Go 版目前只有 OpenAPI 落点，没有契约文件、生成代码或 validate 命令；后续 API 稳定后迁移 Java 路径、schema、prod hardening 与验证流程。 |
| 数据库迁移与持久化边界 | Flyway `V1__init_backend_foundation.sql`、`V2__stage_1_auth_jwt_rbac.sql`、`V3__create_auth_sessions.sql`、`MyBatisConfig.java`、`mapper/auth/*.xml`、entity | `migrations/.gitkeep`、未来 `internal/repository`、`internal/repository/mysql/{queries,sqlc}`、`sqlc.yaml` | 待迁移 | Java 已有 `system_bootstrap_record`、`users`、`roles`、`user_roles`、`auth_sessions` 和 mapper SQL；Go 当前未建 schema/query/repository。sqlc/database 边界已决策，具体配置和迁移测试待数据库阶段。 |
| 数据库测试策略 | Java `application-test.yml`、H2 profile、mapper tests、auth session concurrency tests | Go migration/sqlc/repository/test database strategy | 待决策 | Go 版不默认采用 H2；需要在数据库迁移设计中决定使用 MySQL 容器、testcontainers、临时库或其他策略。 |
| auth 注册、登录、刷新、登出 API | `modules/auth/controller/AuthController.java`、`RegisterRequest`、`LoginRequest`、`RefreshTokenRequest`、`LoginResponse`、`TokenPairResponse`、`AuthService*` | 未来 `internal/http/handler/auth`、`internal/http/dto/auth`、`internal/service/auth`、`internal/security`、`internal/repository` | 待迁移 | Go 尚未实现 `POST /api/v1/auth/register/login/refresh/logout`、BCrypt 密码、token pair、服务端会话或 auth DTO/Result 映射。迁移时需对齐字段、校验消息、错误码和响应结构。 |
| 当前用户与管理员用户 API | `UserController.java`、`AdminUserController.java`、`AdminUserQueryRequest`、`UpdateUserStatusRequest`、`UserInfo`、`SecurityConfig` | 未来 `internal/http/handler/user` 或 `auth`、`internal/http/dto/user` 或 `auth`、`internal/service/user` 或 `auth` | 待迁移 | Go 尚未实现 `GET /api/v1/me`、`GET /api/v1/admin/users`、`PATCH /api/v1/admin/users/{userId}/status`、用户分页筛选、状态切换、`UserStatus` 或管理员 RBAC。 |
| JWT、认证上下文与 RBAC 安全边界 | `SecurityConfig.java`、`JwtClaims.java`、`JwtCodec.java`、`JwtAuthenticationFilter.java`、`AuthenticatedPrincipal*`、`SecurityUtils.java`、auth/security ADRs | 未来 `internal/security/{jwt,principal}`、HTTP auth middleware、service principal 输入 | 待迁移 | Go 已在规则中初始化 claim 边界：JWT 只放稳定身份与技术 claim，不放角色、邮箱、用户名、用户状态；实际 access token 签发、解析、认证上下文、URL/RBAC 规则仍待实现。 |
| refresh token 与 auth session | `AuthTokenProperties.java`、`AuthSessionService*`、`RefreshTokenHasher.java`、`RefreshTokenParser.java`、`AuthSessionStatus.java`、`auth_sessions` migration/mapper/tests | 未来 `internal/security/refresh`、`internal/repository/auth_session_repository.go`、`internal/repository/mysql`、`internal/service/auth` | 待迁移 | Java 已有 opaque refresh token、哈希存储、ACTIVE/REVOKED、轮换条件更新、重放/过期失败语义和并发测试；Go 当前没有会话表、repository、refresh token 生成/解析/轮换。 |
| 认证错误与安全响应 | `AuthException.java`、`RestAuthenticationEntryPoint.java`、`RestAccessDeniedHandler.java`、`SecurityErrorResponseWriter.java` | `internal/apperror` 已有 auth code，未来 auth/security handler 映射 | 待迁移 | `AUTH-401/403/409` 码值已初始化；`账号或密码错误`、`refresh token 无效或已过期`、`请先登录或重新登录`、`权限不足` 等具体安全失败入口尚未迁移。 |
| 容器化与部署配置 | Java `backend/Dockerfile`、`docker-compose.yml`、`application-dev/prod/test.yml` | Go `configs/*.env.example`，未来 Dockerfile/compose/deploy config | 待迁移 | Go 当前只有环境变量示例和本地运行命令，没有容器镜像、compose、数据库服务编排或 prod profile 等价配置。 |
| 活动、票务、订单、支付、通知、审计 | Java `docs/roadmap/stage-2` 至 `stage-7`，当前 production code 尚未落地 | Go 后续 `event/order/payment/notification/audit` 相关 domain/service/repository/handler | 待决策 | 双端当前 production code 都未进入这些业务模块；Go 后续应以 Java roadmap 和未来 Java 实现为语义来源，重点关注库存扣减、幂等、订单状态机、支付回调、通知和操作日志。 |

## 后续维护规则

1. 每迁移一个 Java 业务模块，必须新增或更新对应矩阵行。
2. 如果 Go 版刻意偏离 Java 版实现方式，但保持业务语义一致，需要在设计文档中说明。
3. 如果 Go 版无法保持接口或错误码兼容，必须新增 ADR 或在设计文档中写明理由。
4. 矩阵只记录对齐状态和索引，详细设计仍放在 `docs/ai/design/`、`docs/ai/implementation/` 和 `docs/ai/adr/`。
