# Test Coverage Comparison

最近核验日期：2026-06-06。

本文对照 Java 当前阶段测试目录和 Go 当前测试覆盖，重点记录 auth / RBAC / OpenAPI / 数据库迁移与基础响应契约的测试策略 parity。

## 总览

| 覆盖类别 | Java 测试来源 | Go 测试来源 | 状态 | 说明 |
| --- | --- | --- | --- | --- |
| 统一响应 envelope | `common/api/ApiResponseTest.java` | `internal/http/response/response_test.go`、`internal/http/router_test.go` | 已对齐 | 成功/失败均覆盖 `code/message/data/requestId/timestamp`。 |
| 分页请求与响应 | `PageRequestTest.java`、`PageResponseTest.java` | `internal/page/page_test.go` | 已对齐 | 1-based page、默认值、最大 size、`hasNext/hasPrevious` 已覆盖。 |
| 业务异常与错误码 | `BusinessExceptionTest.java`、`GlobalExceptionHandlerTest.java` | `internal/apperror/error_test.go`、`internal/http/router_test.go`、handler integration tests | 已对齐 | Go 用 `apperror` + response writer 承接 Java `BusinessException` / `GlobalExceptionHandler` 语义。 |
| system / actuator | `SystemControllerTest.java` | `internal/http/router_test.go`、`internal/service/system/*_test.go` | 已对齐 | ping、echo、health/info、HEAD、requestId、404 和 JSON validation 均覆盖。 |
| register | `AuthIntegrationTest.register*` | `internal/http/auth_integration_test.go`、`internal/service/auth/service_test.go` | 已对齐 | 成功、默认 USER 角色、重复账号、唯一键兜底、密码 hash 均覆盖。 |
| login | `AuthIntegrationTest.login*` | `internal/http/auth_integration_test.go`、`internal/service/auth/service_test.go` | 已对齐 | token pair、ACTIVE session、错误密码、禁用用户、TTL 和 `Bearer` scheme 均覆盖。 |
| `/api/v1/me` | `AuthIntegrationTest.currentUser*` | `internal/http/auth_integration_test.go` | 已对齐 | 合法 token 返回用户摘要；缺失、篡改、禁用用户旧 token 返回 `AUTH-401`。 |
| refresh token rotation | `AuthIntegrationTest.refresh*` | `internal/http/auth_integration_test.go`、`internal/service/auth/service_test.go` | 已对齐 | 成功轮换、新 token 可继续刷新、旧 token replay、过期、revoked、禁用用户和非法格式均覆盖。 |
| refresh 并发 | `AuthSessionConcurrencyTest.java`、`AuthIntegrationTest.concurrentRefresh*` | `internal/service/auth/service_test.go`、`internal/repository/mysql/mysql_repository_integration_test.go` | 已对齐 | Go service fake 覆盖并发旧 token 最多一次成功，MySQL integration 覆盖条件更新和 version。 |
| refresh token parser / hasher | `RefreshTokenParserTest.java`、`RefreshTokenHasherTest.java` | `internal/security/refresh/refresh_test.go` | 已对齐 | 长度 43、URL-safe 无 padding、`sha256:<hex>`、稳定 hash 和非法输入均覆盖。 |
| auth_sessions mapper / repository | `AuthSessionMapperTest.java` | `internal/repository/mysql/mysql_repository_integration_test.go` | 已对齐 | Go 使用 Testcontainers MySQL 而不是 H2，覆盖 migration、seed、唯一约束、insert/find/revoke/rotate。 |
| JWT codec | `JwtCodecTest.java` | `internal/security/jwt/jwt_test.go` | 已对齐 | `sub/jti/sid/typ/iss/iat/exp`、错误 typ、缺失 claim、issuer、签名和过期均覆盖。 |
| JWT middleware / disabled old token | `JwtAuthenticationFilter` 由 `AuthIntegrationTest` 间接覆盖 | `internal/http/middleware/auth_test.go`、`internal/http/auth_integration_test.go` | 已对齐 | Bearer 解析、无效 token、主体加载、禁用用户旧 token 均覆盖。 |
| RBAC | `SecurityConfig.java` + `AdminUserController` integration tests | `internal/http/middleware/rbac_test.go`、`internal/http/auth_integration_test.go` | 已对齐 | USER 访问 admin 返回 `AUTH-403`，ADMIN 成功。 |
| admin user list | `AuthIntegrationTest.adminUsers*` | `internal/http/auth_integration_test.go`、`internal/service/user/admin_users_test.go` | 已对齐 | 分页、排序、username/email/status/time range 筛选、参数校验和角色批量加载均覆盖。 |
| admin disable user | `AuthIntegrationTest.disabledUserOldTokenShouldReturnUnauthorized` | `internal/http/auth_integration_test.go`、`internal/service/user/admin_users_test.go` | 已对齐 | 更新状态、目标不存在、禁用后旧 token 失效均覆盖。 |
| logout no-op | `AuthIntegrationTest.logout*` | `internal/http/auth_integration_test.go`、`internal/service/auth/service_test.go` | 已对齐 | 未认证返回 `AUTH-401`；已认证成功且 session 不变。 |
| prod OpenAPI / Swagger 默认关闭 | `OpenApiProductionSecurityTest.java` | `internal/config/config_test.go`、`internal/app/providers/http_test.go`、`internal/http/router_test.go` | 已对齐 | Go prod 默认不注册 `/openapi.yaml` / `/swagger/*`，返回 `COMMON-404`。 |
| OpenAPI contract validation | Springdoc 运行时生成 + integration smoke | `make openapi-validate`、`api/openapi/eventhub.yaml`、generated code | 已对齐 | Go 采用 spec-first，使用 kin-openapi 校验。 |
| Docker / compose smoke | Java 主要由 SpringBootTest 和 profile 覆盖 | 本次新增 in-process HTTP smoke；Makefile 暂无独立 smoke target | 待演进 | 当前无 `make smoke` 或 `docker compose smoke` target；后续可增加外部 HTTP smoke 脚本。 |

## 本次新增 Go 覆盖

新增 `internal/http/auth_integration_test.go` 的 `TestAuthParitySmokeFlow`，用单条 HTTP 集成链路覆盖：

1. register
2. login
3. me
4. refresh
5. old refresh replay
6. logout
7. admin list
8. admin disable user
9. disabled user old token rejected

该测试使用现有 in-memory fake repository 运行完整 router、auth middleware、RBAC middleware、handler、service 和 repository interface 边界；真实 MySQL 行为仍由 repository/mysql integration tests 覆盖。

## Java-Go 测试策略差异

| 差异 | 状态 | 原因 |
| --- | --- | --- |
| Java 使用 SpringBootTest + H2 MySQL mode；Go repository integration 使用 Testcontainers MySQL | 有意差异 | Go 避免 H2 和 MySQL 方言差异，直接验证真实 MySQL migration、索引和 sqlc query。 |
| Java OpenAPI 由 Springdoc 运行时生成；Go OpenAPI 使用 spec-first YAML | 有意差异 | Go 以 `api/openapi/eventhub.yaml` 为契约源，并用 `make openapi-validate` 校验。 |
| Java prod 文档测试覆盖匿名 401 与带 token 404；Go 禁用文档路由后统一 404 | 有意差异 | Go 不复刻 Spring Security 对未注册文档资源的拦截顺序；安全暴露面一致。 |
| Go 暂无独立 Docker Compose smoke target | 待演进 | 当前 Makefile 只有 `compose-up/compose-down`，本次用 Go HTTP integration smoke 覆盖契约链路。 |

## 剩余风险

- 尚未做 Java `/v3/api-docs` 与 Go `/openapi.yaml` 的机器级 schema diff；当前通过 Java Controller/DTO/VO 源码和 Go spec 静态对照完成审计。
- Go smoke 测试是进程内 HTTP 测试，不能替代真实容器网络、MySQL、Redis 和 migration job 的端到端 smoke。
- 当后续新增 event/order/payment 等业务模块时，需要按本表模式新增模块级测试覆盖对照。
