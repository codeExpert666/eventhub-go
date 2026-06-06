# Current Parity Audit

## 1. 背景
- 当前 Go 版 EventHub 已完成基础 HTTP、auth、refresh token 轮换、管理员用户管理、OpenAPI 生产关闭和 Docker 开发流程等阶段能力，需要对 Java 版当前阶段做一次快照式 parity audit，确认业务语义、接口契约、错误码、数据库模型、安全边界和测试覆盖是否仍然对齐。
- Java 版主要来源：
  - Controller / DTO / VO：`backend/src/main/java/com/eventhub/modules/{auth,system}/**`
  - 统一响应与错误：`backend/src/main/java/com/eventhub/common/{api,exception}/**`
  - migration / mapper：`backend/src/main/resources/db/migration/*.sql`、`backend/src/main/resources/mapper/auth/*.xml`
  - 安全配置：`backend/src/main/java/com/eventhub/infra/security/**`
  - OpenAPI / Swagger：`backend/src/main/java/com/eventhub/infra/openapi/OpenApiConfig.java`、`application*.yml`
  - 测试：`backend/src/test/java/com/eventhub/**`
- Go 版对应来源：
  - HTTP / DTO / response：`internal/http/**`
  - service / repository / security：`internal/service/{auth,user}`、`internal/repository/**`、`internal/security/**`
  - migration / sqlc：`migrations/*.sql`、`internal/repository/mysql/queries/*.sql`
  - OpenAPI：`api/openapi/eventhub.yaml`、`internal/http/handler/openapi`
  - 测试：`internal/**/*_test.go`
- 本设计文档不改变既有模板结构。

## 2. 目标
- 完成 Go 版追平 Java 当前阶段的 parity audit。
- 更新或新增审计文档：
  - `docs/ai/parity/java-go-parity-matrix.md`
  - `docs/ai/parity/current-auth-contract-checklist.md`
  - `docs/ai/parity/test-coverage-comparison.md`
- 审计并分级记录以下内容：
  1. 接口是否齐全。
  2. HTTP method/path 是否一致。
  3. 请求字段/响应字段是否一致。
  4. `ApiResponse` 的 `code/message/data/requestId/timestamp` 是否一致。
  5. 错误码是否一致。
  6. `users/roles/user_roles/auth_sessions` schema 是否一致。
  7. JWT claims 是否一致。
  8. refresh token 轮换语义是否一致。
  9. replay 行为是否一致。
  10. logout no-op 是否一致。
  11. RBAC 是否一致。
  12. prod OpenAPI / Swagger 是否默认关闭。
  13. 测试覆盖类别是否对齐。
- 若发现 P0 差异，必须修复后再完成总结。
- 补充必要 smoke/e2e Go 测试，覆盖 register、login、me、refresh、old refresh replay、logout、admin list、admin disable user、disabled user old token rejected。
- 成功标准：
  - P0 差异为 0 或已修复。
  - 有意偏离均有文档说明，关键架构取舍有既有 ADR 或新增 ADR。
  - `make fmt vet test`、`make test-race`、`make openapi-validate` 可运行并记录结果。
  - 如仓库已有 docker compose smoke 命令则运行；若没有，说明原因。

## 3. 非目标
- 不迁移 Java/Spring 的实现结构，例如 Spring Security FilterChain、Bean Validation 注解、Springdoc 注解扫描、MyBatis XML 或 Flyway 命名方式。
- 不引入新的 auth 业务功能，例如服务端 logout 吊销 access token、refresh token 历史哈希留存、设备会话列表、Redis denylist 或审计日志。
- 不变更已有 API 路径、数据库 schema、错误码或 JWT claim，除非审计发现 P0 差异。
- 不为未开始的 event/order/payment/notification/audit 模块创建空 package 或空文档。

## 4. 影响范围
- Go package / 模块：
  - 预计新增或调整：`internal/http/auth_integration_test.go`、`docs/ai/parity/*`、`docs/ai/design/*`、`docs/ai/implementation/*`
  - 可能只读核验：`internal/http/router.go`、`internal/http/dto/{auth,user}`、`internal/http/response`、`internal/apperror`、`internal/service/{auth,user}`、`internal/security/{jwt,refresh}`、`internal/repository/mysql`、`api/openapi/eventhub.yaml`、`migrations/*.sql`
- API / 表 / 缓存 / 外部接口：
  - API：system、auth、me、admin users、actuator、OpenAPI / Swagger 文档入口。
  - 表：`system_bootstrap_record`、`users`、`roles`、`user_roles`、`auth_sessions`。
  - 缓存：当前 Redis 不参与认证强一致，本次只审计该取舍是否仍被文档索引。
- 会更新 `docs/ai/parity/java-go-parity-matrix.md`，并新增专题 checklist 和测试覆盖对照。

## 5. 领域建模
- 核心实体：
  - `User`：对应 Java `UserEntity` 与 Go `repository.User`，字段包括 `id/username/email/password_hash/status/created_at/updated_at`。
  - `Role`：对应 Java `RoleEntity` 与 Go `repository.Role`，字段包括 `id/code/name/description/created_at`。
  - `UserRole`：用户和角色多对多绑定，唯一键为 `(user_id, role_id)`。
  - `AuthSession`：服务端 refresh token 权威会话，字段包括 `session_id/user_id/refresh_token_hash/status/issued_at/refresh_expires_at/last_refreshed_at/last_seen_at/revoked_at/revoke_reason/client_ip_hash/user_agent_hash/user_agent_summary/version`。
- 关键状态：
  - `UserStatus`：`ENABLED`、`DISABLED`。
  - `AuthSessionStatus`：`ACTIVE`、`REVOKED`；过期由 `refresh_expires_at` 派生，不额外持久化 `EXPIRED`。
- 与 Java 版领域对象的对应关系：
  - Go 不暴露 sqlc generated model 到 handler；handler 使用 HTTP DTO，service 使用 Command / Query / Result，repository 映射到持久化模型。
  - Java `UserInfo` 在 Go 中对应 `internal/http/dto/user.UserInfoResponse` 和 `internal/service/user.UserResult`。
  - Java `LoginResponse` / `TokenPairResponse` 在 Go 中对应 auth DTO 和 service result。

## 6. API 设计
- 本次预期不新增生产 API，仅补审计文档和 smoke/e2e 测试。
- 当前需对齐的接口列表：
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET|HEAD /actuator/health`
  - `GET|HEAD /actuator/info`
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout`
  - `GET /api/v1/me`
  - `GET /api/v1/admin/users`
  - `PATCH /api/v1/admin/users/{userId}/status`
- 请求参数和响应结构以 Java Controller / DTO / VO 及 Go OpenAPI 为准：
  - register：`username/email/password`，响应 `UserInfo`。
  - login：`usernameOrEmail/password`，响应 `accessToken/refreshToken/authorizationScheme/expiresIn/refreshExpiresIn/sessionId/user`。
  - refresh：`refreshToken`，响应字段与 login token pair 一致。
  - logout：受保护接口，成功 `data=null`。
  - me：受保护接口，响应 `UserInfo`。
  - admin list：`page/size/username/email/status/createdAtFrom/createdAtTo/updatedAtFrom/updatedAtTo`，响应分页 `items/page/size/total/totalPages/hasNext/hasPrevious`。
  - admin update status：path `userId`，body `status`，响应更新后的 `UserInfo`。
- 错误码 / 异常场景：
  - 成功：`COMMON-000`
  - validation：`COMMON-400`
  - not found：`COMMON-404`
  - internal：`COMMON-500`
  - auth unauthorized：`AUTH-401`
  - auth forbidden：`AUTH-403`
  - auth conflict：`AUTH-409`
- 与 Java OpenAPI / controller 契约的已知差异：
  - Java 使用 Springdoc 路径 `/v3/api-docs`、`/swagger-ui.html`；Go 使用 spec-first `/openapi.yaml`、`/swagger/*`。这是既有有意差异，已由 ADR-0018/0019 索引。
  - Java prod 下匿名访问文档路径先落入认证失败，携带管理员 token 后由 springdoc 资源关闭返回 404；Go prod 默认不注册文档路由，匿名和带 token 都返回 `COMMON-404`。这是 Go spec-first 路由注册模型下的有意差异，生产暴露面语义一致。

## 7. 数据设计
- 本次预期不调整表结构、索引、唯一约束或 migration。
- 审计需核对：
  - `users` 字段、唯一约束 `uk_users_username` / `uk_users_email`、状态值。
  - `roles` 字段、唯一约束 `uk_roles_code`、`USER` / `ADMIN` seed。
  - `user_roles` 唯一约束、外键和 `idx_user_roles_role_id`。
  - `auth_sessions` 字段、唯一约束、外键、`idx_auth_sessions_user_id`、`idx_auth_sessions_status`、`idx_auth_sessions_refresh_expires_at`。
  - Java V2/V3 与 Go `000002_auth_schema` 的合并迁移是否仍保持空库最终 schema 一致。
- sqlc query / generated model 影响：
  - 如无 schema 或 query 差异，不运行 `sqlc generate`。
  - 如发现 P0 query 行为差异，需更新 `internal/repository/mysql/queries/*.sql` 并运行 `make sqlc` 或记录无法运行原因。
- 数据一致性考虑：
  - 注册使用用户唯一键作为并发最终防线。
  - refresh 轮换依赖 `session_id + old_refresh_token_hash + version + status + refresh_expires_at` 条件更新，保证旧 token 最多成功一次。

## 8. 关键流程
- 正常流程：
  - register：handler decode/validate -> service 归一化账号、检查唯一性、hash 密码、创建用户、绑定 USER 角色 -> repository/sqlc/migration schema。
  - login：handler -> service 校验用户名或邮箱、密码和用户状态 -> 创建 ACTIVE auth session -> 签发 access token 与 opaque refresh token。
  - me：auth middleware 解析 Bearer token -> service 加载最新用户状态与角色 -> handler 返回 `UserInfo`。
  - refresh：handler -> service 解析和 hash 旧 refresh token -> 查 ACTIVE 未过期 session -> 查用户仍 ENABLED -> 条件更新为新 refresh token hash -> 签发新 access token。
  - logout：受保护路由进入 handler/service，当前 no-op，不修改 session。
  - admin list / disable：auth middleware + RBAC -> handler -> service -> repository；禁用用户后旧 access token 请求期会重新查库并被拒绝。
- 异常流程：
  - 账号重复：`AUTH-409`。
  - 账号或密码错误：`AUTH-401`。
  - 禁用用户登录：`AUTH-403`。
  - refresh 无效、过期、重放、用户禁用或会话吊销：统一 `AUTH-401`。
  - 普通用户访问 admin：`AUTH-403`。
  - admin 更新不存在用户：`COMMON-404`。
- handler / service / repository / sqlc/database 分工：
  - handler 只处理 HTTP 入参、认证上下文、响应映射和错误映射。
  - service 承载业务规则、事务、幂等与状态流转。
  - repository 表达持久化语义。
  - sqlc/database 只承载 schema 对应查询和生成代码。

## 9. 并发 / 幂等 / 缓存
- 当前认证域没有库存超卖风险。
- 防重复提交和并发：
  - 注册并发依赖数据库唯一约束兜底。
  - refresh replay 和并发刷新依赖条件更新与 version，旧 refresh token 最多成功一次。
  - replay 后当前会话保持 `ACTIVE` 且保留最新 refresh token hash，不因无法定位历史 token 而吊销已轮换会话；该取舍已有 ADR-0014。
- 事务边界：
  - register：创建用户和绑定角色同一事务。
  - login：会话创建和 token 返回同一事务语义，确保返回 token 的 `sid` 已有权威 session。
  - refresh：查 session、查用户、条件更新和签发响应在 service 事务边界内组织。
  - admin update status：状态更新和回读响应在同一事务语义内。
- 缓存：
  - 当前 Redis 不参与 auth 强一致，用户禁用、角色和 refresh 仍以 MySQL 为准。

## 10. 权限与安全
- 匿名接口：register、login、refresh、system ping/echo、actuator health/info、非生产文档入口。
- 已认证接口：logout、me。
- ADMIN 接口：admin list、admin update status。
- 鉴权与鉴别约束：
  - HTTP 只接受 `Authorization: Bearer <access-token>`。
  - 认证 middleware 解析 JWT 后必须重新加载最新用户状态和角色。
  - RBAC 使用 `ROLE_ADMIN` 权限语义；Go 可接受未带前缀的业务角色并在 middleware 内规范化。
- JWT claim 边界：
  - 允许：`iss/sub/iat/exp/jti/sid/typ`。
  - 禁止：角色、邮箱、用户名、用户状态。
- refresh token：
  - opaque、32 字节随机、Base64 URL-safe 无 padding。
  - 服务端只保存带算法前缀的 SHA-256 hash，不保存明文。
- OpenAPI / Swagger：
  - prod 默认关闭，Go 不注册路由并返回 `COMMON-404`，避免暴露接口契约。

## 11. 测试策略
- 单元测试：
  - `internal/security/jwt`：claim、签名、issuer、过期、typ、sid、jti。
  - `internal/security/refresh`：token 生成、格式、hash。
  - `internal/apperror`、`internal/http/response`、`internal/page`：统一响应和分页元数据。
- service / repository 测试：
  - auth service：register/login/refresh/replay/logout。
  - user service：admin list、status update。
  - repository/mysql：migration、seed、唯一约束、条件更新和事务上下文。
- 接口验证：
  - 新增或调整一个 auth parity smoke/e2e Go 测试，串联 register -> login -> me -> refresh -> old refresh replay -> logout -> admin list -> admin disable user -> disabled user old token rejected。
  - 保留现有细粒度 handler/service 测试，避免 smoke 失败时定位困难。
- OpenAPI validate：
  - 运行 `make openapi-validate`。
- 异常场景验证：
  - refresh replay、禁用用户 old token、RBAC 403、prod OpenAPI 默认关闭。
- Java-Go parity 验证：
  - 新增 checklist 文档逐项标记 P0/P1/P2、状态、Java 来源、Go 证据和差异说明。
  - 新增测试覆盖对照文档，按 Java 测试类别映射 Go 测试文件与剩余缺口。
- 需要运行的命令：
  - `make fmt vet test`
  - `make test-race`
  - `make openapi-validate`
  - 如仓库存在 smoke compose 命令则运行；当前 Makefile 只有 `compose-up/compose-down`，没有独立 smoke target，最终需说明。

## 12. 风险与替代方案
- 当前方案风险：
  - 审计主要依赖源码、测试和 OpenAPI 静态对照，若 Java 运行时由 Spring 自动生成的 OpenAPI 与注解存在细小差异，需要以后通过实际 `/v3/api-docs` 导出再做机器对比。
  - Go 的 `/openapi.yaml` 与 Java `/v3/api-docs` 路径不同，属于既有有意差异；需要在 checklist 中避免误判为业务契约缺失。
  - 新增 smoke 测试若过度依赖测试 fake store，可能覆盖不到真实 MySQL 行为；因此真实 schema/SQL 行为仍由 repository/mysql 集成测试承担。
- 备选方案：
  - 方案 A：只更新文档，不新增测试。缺点是用户明确要求必要 smoke/e2e 覆盖，且端到端链路缺少单个可读用例。
  - 方案 B：新增 Docker Compose 外部 smoke 脚本。缺点是当前 Makefile 未定义应用启动后的 smoke target，且端口、迁移时机和环境依赖会让本次审计变重；适合后续独立演进。
  - 方案 C：生成并比较 Java `/v3/api-docs` 与 Go `/openapi.yaml`。缺点是需要启动 Java 应用与测试数据库/Redis，本次手工源码审计已足够定位当前阶段 P0。
- 选择当前方案的原因：
  - 最小改动即可固化当前 parity 结论，并补上用户指定的 smoke/e2e 覆盖。
  - 不改变生产 API、schema 或安全边界，降低回归风险。
  - 保持 Go idiom 和现有分层，不引入 Java/Spring 结构。
- 后续可演进点：
  - 增加真实 Docker Compose smoke target，启动 MySQL/Redis/migrate/app 后通过 HTTP 客户端跑同一条 auth/admin 场景。
  - 增加 OpenAPI 机器对比工具，允许记录路径差异映射后比较 schema 字段。
  - 后续 event/order/payment 模块开始迁移时，按本次 checklist 模式新增专题 parity 文档。
