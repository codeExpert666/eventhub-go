# Current Auth Contract Checklist

最近核验日期：2026-06-06。

本文是 Go 版对 Java 当前 auth / RBAC 阶段的快照审计清单，索引 `docs/ai/design/018-current-parity-audit.md` 和 `docs/ai/implementation/018-current-parity-audit.md`。

## 分级规则

- P0：影响生产契约、认证安全、数据一致性或 Java-Go 核心语义，必须修复。
- P1：当前语义可用，但测试、文档或自动化覆盖不足，应在本次或近期补齐。
- P2：有意偏离实现方式、路径或工程组织，但业务目标一致，需要文档化。

## 结论

- P0 差异：0。
- P1 差异：1，已通过 `internal/http/auth_integration_test.go` 的 `TestAuthParitySmokeFlow` 补齐。
- P2 有意差异：3，均有既有设计或 ADR 索引。

## 契约清单

| 审计项 | 优先级 | Java 来源 | Go 证据 | 状态 | 说明 |
| --- | --- | --- | --- | --- | --- |
| 接口是否齐全 | P0 | `AuthController.java`、`UserController.java`、`AdminUserController.java`、`SystemController.java` | `internal/http/router.go`、`api/openapi/eventhub.yaml` | 已对齐 | auth、me、admin users、system、actuator 均已注册；OpenAPI 文档入口采用 Go spec-first 路径。 |
| HTTP method/path | P0 | Java controller `@RequestMapping` / `@PostMapping` / `@GetMapping` / `@PatchMapping` | `internal/http/router.go`、`api/openapi/eventhub.yaml` | 已对齐 | 业务 API 路径一致：`/api/v1/auth/*`、`/api/v1/me`、`/api/v1/admin/users`。 |
| 文档入口路径 | P2 | Springdoc `/v3/api-docs`、`/swagger-ui.html` | Go `/openapi.yaml`、`/swagger/*` | 有意差异 | Go 使用 spec-first，不迁移 Springdoc 注解扫描路径；参考 ADR-0018、ADR-0019。 |
| register 请求字段 | P0 | `RegisterRequest.java` | `internal/http/dto/auth/request.go`、auth validation | 已对齐 | `username/email/password`；用户名、邮箱、密码约束和归一化语义一致。 |
| register 响应字段 | P0 | `UserInfo.java` | `internal/http/dto/user/response.go` | 已对齐 | `id/username/email/status/roles`，不暴露 `passwordHash`。 |
| login 请求字段 | P0 | `LoginRequest.java` | `internal/http/dto/auth/request.go` | 已对齐 | `usernameOrEmail/password`。 |
| login 响应字段 | P0 | `LoginResponse.java` | `internal/http/dto/auth/response.go` | 已对齐 | `accessToken/refreshToken/authorizationScheme/expiresIn/refreshExpiresIn/sessionId/user`。 |
| refresh 请求/响应字段 | P0 | `RefreshTokenRequest.java`、`TokenPairResponse.java` | `internal/http/dto/auth/{request,response}.go` | 已对齐 | refresh 请求只接收 `refreshToken`；响应字段与 Java token pair 一致。 |
| logout 响应 | P0 | `AuthController.logout` | `internal/http/handler/auth/logout.go` | 已对齐 | 受保护 POST，成功返回 `COMMON-000` 且 `data=null`。 |
| admin list 请求字段 | P0 | `AdminUserQueryRequest.java` | `internal/http/dto/user/request.go`、`api/openapi/eventhub.yaml` | 已对齐 | `page/size/username/email/status/createdAtFrom/createdAtTo/updatedAtFrom/updatedAtTo`。 |
| admin list 响应字段 | P0 | `PageResponse.java`、`UserInfo.java` | `internal/page`、`internal/http/dto/user/response.go` | 已对齐 | `items/page/size/total/totalPages/hasNext/hasPrevious`。 |
| admin status 请求/响应字段 | P0 | `UpdateUserStatusRequest.java` | `internal/http/dto/user/request.go` | 已对齐 | `status` 必填，只允许 `ENABLED` / `DISABLED`，响应返回更新后的 `UserInfo`。 |
| ApiResponse envelope | P0 | `ApiResponse.java`、`SecurityErrorResponseWriter.java` | `internal/http/response` | 已对齐 | 统一包含 `code/message/data/requestId/timestamp`，成功码 `COMMON-000`。 |
| 错误码集合 | P0 | `ErrorCode.java`、`AuthException.java` | `internal/apperror/code.go`、`internal/service/auth/errors.go` | 已对齐 | `COMMON-000/400/401/404/500`、`AUTH-401/403/409` 均已覆盖。 |
| validation 错误语义 | P0 | `GlobalExceptionHandler.java`、DTO validation | `internal/http/validation`、handler validation | 已对齐 | 请求体格式、字段校验、查询参数校验均映射 `COMMON-400`。 |
| users schema | P0 | `V2__stage_1_auth_jwt_rbac.sql` | `migrations/000002_auth_schema.up.sql` | 已对齐 | 字段、唯一约束和 `ENABLED/DISABLED` 状态值一致。 |
| roles schema / seed | P0 | `V2__stage_1_auth_jwt_rbac.sql` | `migrations/000002_auth_schema.up.sql` | 已对齐 | `USER` / `ADMIN` seed、`uk_roles_code` 一致。 |
| user_roles schema | P0 | `V2__stage_1_auth_jwt_rbac.sql` | `migrations/000002_auth_schema.up.sql` | 已对齐 | 唯一约束、外键和角色反查索引一致。 |
| auth_sessions schema | P0 | `V3__create_auth_sessions.sql` | `migrations/000002_auth_schema.up.sql` | 已对齐 | 字段、唯一约束、外键、状态和索引一致；Go 从空库合并 V2/V3。 |
| JWT claims | P0 | `JwtClaims.java`、`JwtCodec.java` | `internal/security/jwt/jwt.go` | 已对齐 | token 只包含 `iss/sub/iat/exp/jti/sid/typ`；不写角色、邮箱、用户名、状态。 |
| JWT 解析失败语义 | P0 | `JwtAuthenticationFilter.java` | `internal/http/middleware/auth.go` | 已对齐 | 过期、篡改、缺失 claim、错误 typ、用户禁用均返回 `AUTH-401`。 |
| refresh token 格式 | P0 | `RefreshTokenParser.java`、`TokenServiceImpl.java` | `internal/security/refresh/refresh.go` | 已对齐 | 32 字节随机，Base64 URL-safe 无 padding，长度 43。 |
| refresh hash 格式 | P0 | `RefreshTokenHasher.java` | `internal/security/refresh/refresh.go` | 已对齐 | `sha256:<hex>`，不保存明文。 |
| refresh token 轮换 | P0 | `AuthServiceImpl.refresh`、`AuthSessionMapper.xml` | `internal/service/auth/refresh_token.go`、`queries/auth_sessions.sql` | 已对齐 | 条件更新 old hash / version / ACTIVE / 未过期，成功后替换新 hash 并 version+1。 |
| old refresh replay | P0 | `AuthIntegrationTest.replayedOldRefreshTokenShouldReturnUnauthorizedAndKeepRotatedSessionActive` | `TestAuthRefreshEndpointRejectsReplay`、`TestAuthParitySmokeFlow` | 已对齐 | replay 返回 `AUTH-401`；不保存历史 hash，不吊销已轮换的新会话，ADR-0014 已记录。 |
| logout no-op | P0 | `AuthServiceImpl.logout` | `internal/service/auth/logout.go`、`TestAuthLogoutEndpointIsNoopForAuthenticatedUser` | 已对齐 | 必须认证，但当前不修改 `auth_sessions`。 |
| RBAC | P0 | `SecurityConfig.java`、`@PreAuthorize("hasRole('ADMIN')")` | `internal/http/middleware/rbac.go` | 已对齐 | admin list/status 需要 `ROLE_ADMIN`，普通 USER 返回 `AUTH-403`。 |
| 禁用用户旧 token | P0 | `AuthenticatedPrincipalService.java`、Java integration test | `internal/service/user.LoadPrincipal`、`TestAdminUpdateUserStatusEndpointDisablesOldAccessToken`、`TestAuthParitySmokeFlow` | 已对齐 | 每次受保护请求回 DB 加载最新状态；禁用后旧 access token 返回 `AUTH-401`。 |
| prod OpenAPI 默认关闭 | P0 | `application-prod.yml`、`SecurityConfig.java`、`OpenApiProductionSecurityTest.java` | `internal/config/config.go`、`internal/app/providers/http.go`、router/provider/config tests | 已对齐 | prod 默认 `OPENAPI_ENABLED=false`，Go 不注册文档路由并返回 `COMMON-404`。 |
| prod 文档关闭时的未认证状态码 | P2 | Java 匿名 `/v3/api-docs` 返回 `AUTH-401`，带 token 后返回 `COMMON-404` | Go `/openapi.yaml` 禁用时统一 `COMMON-404` | 有意差异 | Go 不复刻 Spring Security 对未注册文档资源的认证拦截；安全暴露面一致。 |
| admin 超大 page | P2 | Java `PageRequest` 使用 int + Bean Validation | Go `pageOffset` 防止 sqlc `int32` offset 溢出 | 有意差异 | Go 额外保护生成代码参数边界，正常分页契约不变。 |
| 测试覆盖类别 | P1 | Java `AuthIntegrationTest` 等 | Go `internal/**/*_test.go` | 已对齐 | 本次新增 `TestAuthParitySmokeFlow`，补齐单条 smoke 链路可读性。 |

## P0 审计结果

本次未发现需要改生产代码的 P0 差异。

## P1 修复记录

- 缺口：Go 已有分散测试覆盖 register/login/me/refresh/replay/logout/admin/disabled old token，但缺少一条集中展示当前 auth/admin 契约的 smoke/e2e flow。
- 修复：新增 `internal/http/auth_integration_test.go` 的 `TestAuthParitySmokeFlow`。
- 覆盖链路：register -> login -> me -> refresh -> old refresh replay -> logout -> admin list -> admin disable user -> disabled user old token rejected。

## P2 有意差异记录

- OpenAPI / Swagger 路径：Go 使用 `/openapi.yaml` 和 `/swagger/*`，不使用 Java Springdoc `/v3/api-docs` 和 `/swagger-ui.html`。
- prod 文档关闭状态码：Go 默认不注册文档路由，统一返回 `COMMON-404`；Java 匿名请求先进入认证边界返回 `AUTH-401`。
- Go admin list 对超大 page 做 offset 溢出保护，避免 sqlc `int32` 参数越界。
