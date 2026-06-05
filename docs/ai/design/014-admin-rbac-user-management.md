# 管理员 RBAC 与用户管理设计

## 1. 背景
- Go 版 EventHub 已实现注册、登录、refresh/logout、`GET /api/v1/me`、JWT access token 最小 claim 和按 `sub` 回库加载当前用户状态与角色。
- Java 版阶段 1 已提供管理员用户管理能力：
  - `GET /api/v1/admin/users`
  - `PATCH /api/v1/admin/users/{userId}/status`
  - USER / ADMIN RBAC，普通 USER 访问管理端返回 403。
  - 管理员用户列表分页、筛选、按当前页用户 ID 批量查角色，避免 N+1。
  - 管理员禁用用户后，该用户已签发但未过期的旧 access token 在下一次受保护请求中被拒绝。
- Java 版对应来源：
  - `backend/src/main/java/com/eventhub/modules/auth/controller/AdminUserController.java`
  - `backend/src/main/java/com/eventhub/modules/auth/dto/request/AdminUserQueryRequest.java`
  - `backend/src/main/java/com/eventhub/modules/auth/dto/request/UpdateUserStatusRequest.java`
  - `backend/src/main/java/com/eventhub/modules/auth/service/impl/AuthServiceImpl.java`
  - `backend/src/main/java/com/eventhub/modules/auth/mapper/UserMapper.java`
  - `backend/src/main/java/com/eventhub/modules/auth/mapper/RoleMapper.java`
  - `backend/src/main/resources/mapper/auth/UserMapper.xml`
  - `backend/src/main/resources/mapper/auth/RoleMapper.xml`
  - `backend/src/test/java/com/eventhub/modules/auth/AuthIntegrationTest.java`
  - Java 文档：`docs/ai/design/2026-05-18-admin-user-pagination-design.md`、`docs/ai/design/2026-05-11-explicit-security-matchers-design.md`、`docs/interview/stage-1-auth-jwt-rbac/decisions-and-tradeoffs.md`
- 业务上下文：当前平台需要建立普通用户与管理员后台的最小权限边界，为后续活动、场次、票种、订单、审计等管理能力提供可复用的 ADMIN 门禁。

## 2. 目标
- 实现 USER / ADMIN RBAC，新增 `RequireRole("ADMIN")` middleware。
- 实现管理员用户列表：
  - `GET /api/v1/admin/users`
  - 1-based page，默认 `page=1`、`size=20`、最大 `size=100`。
  - 支持 `username`、`email`、`status`、`createdAtFrom`、`createdAtTo`、`updatedAtFrom`、`updatedAtTo` 筛选。
  - 默认按 `created_at DESC, id DESC` 返回新注册用户优先。
  - 在 service 层进入 COUNT 和 SQL 查询前校验 `offset=(page-1)*size` 可安全放入 sqlc 使用的 `int32`，避免超大页码溢出或被截断。
  - 列表响应使用 Go 端已有 `page.Response`，字段对齐 Java `PageResponse`。
  - 当前页用户角色通过一次批量查询加载，避免 N+1。
- 实现管理员更新用户状态：
  - `PATCH /api/v1/admin/users/{userId}/status`
  - 请求体字段 `status` 仅允许 `ENABLED` / `DISABLED`。
  - 更新成功后回读用户摘要与角色并返回。
  - 用户不存在返回 `COMMON-404` / `用户不存在`。
- 保持 JWT claim 边界：
  - JWT 仍只保存 `iss/sub/iat/exp/jti/sid/typ=access`。
  - 不写 roles、email、username、status。
  - 禁用用户旧 access token 继续由认证 middleware 的回库加载机制拒绝。
- 补充测试覆盖：
  - USER 被管理员接口拒绝。
  - ADMIN 成功访问用户列表。
  - 分页与筛选。
  - 状态更新。
  - 禁用用户旧 access token 被拒绝。
- 成功标准：
  - Go API 路径、字段、分页语义、错误码和核心安全行为与 Java 版一致。
  - `go test ./...` 通过。

## 3. 非目标
- 本次不引入 permissions、role_permissions、菜单权限、按钮权限或资源级数据权限。
- 本次不新增角色管理接口，不支持管理员动态给用户授予或移除角色。
- 本次不把角色或权限写入 JWT。
- 本次不让 access token 校验 auth session revoke 状态；该边界已在 ADR-0013 中说明。
- 本次不新增 Redis 缓存、权限短缓存或 token denylist。
- 本次不新增数据库表、字段、索引或 migration；复用现有 `users`、`roles`、`user_roles`。
- 本次不迁移 Spring Security 的双层注解机制；Go 版用 router 受保护分组 + RBAC middleware 表达等价业务语义。
- 本次不逐行迁移 Java/Spring/MyBatis 类结构；Go 版继续保持 `handler -> service -> repository -> sqlc/database`。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `internal/http/middleware`
    - 新增 RBAC middleware。
  - `internal/http/router.go`
    - 在已认证路由组中注册管理员用户接口，并叠加 `RequireRole("ADMIN")`。
  - `internal/http/dto/user`
    - 新增管理员用户查询请求和状态更新请求 DTO。
  - `internal/http/handler/user`
    - 新增管理员用户列表、状态更新 handler 和映射/校验逻辑。
  - `internal/service/user`
    - 新增管理员列表 Query、状态更新 Command、分页 Result 和业务方法。
    - 为状态更新注入事务运行能力，保持 service 承载事务边界。
  - `internal/repository`
    - 调整用户分页 repository 语义命名，暴露用户列表查询与状态更新。
    - 角色批量加载继续放在 `RoleRepository`，因为角色关系属于 `roles/user_roles` 持久化边界。
  - `internal/repository/mysql`
    - 使用现有 sqlc 查询实现分页、状态更新与角色批量查询。
  - `internal/app/providers`
    - 装配 user service 的事务依赖。
  - `internal/http`、`internal/service/user`、`internal/http/middleware` 测试。
- 重要的不触碰包：
  - `internal/security/jwt`：不改变 claim、签名和解析。
  - `migrations`：不改 schema。
  - `internal/domain`：当前阶段用户持久化模型已由 repository 表达，不新增空 domain package。
  - `api/openapi`：当前 Go 仓库尚未维护 OpenAPI 契约文件，本次不生成。
- 涉及 API：
  - 新增 `GET /api/v1/admin/users`。
  - 新增 `PATCH /api/v1/admin/users/{userId}/status`。
- 涉及表：
  - 读取 `users`、`roles`、`user_roles`。
  - 更新 `users.status` 和 `users.updated_at`。
- 影响 `docs/ai/parity/java-go-parity-matrix.md`：是。本次迁移 Java 管理员用户 API、RBAC、分页筛选、角色批量查询、状态更新和测试策略。

## 5. 领域建模
- `User`
  - Go 端对应 `repository.User`，字段对齐 `users` 表。
  - 对外响应只暴露 `id`、`username`、`email`、`status`、`roles`。
  - 不暴露 `passwordHash`。
- `UserStatus`
  - 当前状态集合：
    - `ENABLED`：允许登录、refresh、访问受保护接口。
    - `DISABLED`：不能登录，旧 access token 在认证 middleware 回库加载时被拒绝。
  - 状态流转：
    - `ENABLED -> DISABLED`
    - `DISABLED -> ENABLED`
  - 本阶段不增加 `LOCKED`、`DELETED`、`PENDING` 等状态。
- `Role`
  - 当前角色集合：
    - `USER`：普通登录用户。
    - `ADMIN`：管理员后台用户。
  - 数据库存储角色编码不带 `ROLE_` 前缀。
  - `security.Principal.Authorities` 保存带 `ROLE_` 前缀的 authority，供 RBAC middleware 判断。
  - HTTP 响应中的 `roles` 仍返回不带 `ROLE_` 前缀的业务角色编码。
- `Principal`
  - 来源于认证 middleware 根据 JWT `sub` 回库加载的最新用户状态和角色。
  - JWT 不承担角色或用户状态快照职责。
- `AdminUserListQuery`
  - service 层查询输入，不带 HTTP `json` tag。
  - 包含分页、账号字段、状态和时间范围筛选。
- `UpdateUserStatusCommand`
  - service 层写操作输入，不带 HTTP `json` tag。
  - 包含目标用户 ID 和目标状态。
- 与 Java 版领域对象对应关系：
  - Java `UserEntity` 对应 Go `repository.User`。
  - Java `UserStatus` 对应 Go `repository.UserStatus`。
  - Java `UserInfo` 对应 Go `userdto.UserInfoResponse` / `usersvc.UserResult`。
  - Java `AdminUserQueryRequest` 对应 Go `userdto.AdminUserListRequest` 和 `usersvc.AdminUserListQuery`。
  - Java `UpdateUserStatusRequest` 对应 Go `userdto.UpdateUserStatusRequest` 和 `usersvc.UpdateUserStatusCommand`。
  - Java `UserRoleCodeResult` 对应 Go `repository.UserRoleCode`。

## 6. API 设计
- `GET /api/v1/admin/users`
  - 鉴权：必须携带有效 Bearer access token。
  - 授权：必须拥有 `ADMIN` 角色，对应 `Principal.Authorities` 包含 `ROLE_ADMIN`。
  - 查询参数：
    - `page`：可选，默认 `1`，最小 `1`。
    - `size`：可选，默认 `20`，最小 `1`，最大 `100`。
    - `username`：可选，最长 32，trim 后为空忽略，包含匹配。
    - `email`：可选，最长 128，trim 后为空忽略，转小写后包含匹配。
    - `status`：可选，只允许 `ENABLED` 或 `DISABLED`。
    - `createdAtFrom` / `createdAtTo`：可选，ISO 本地时间，例如 `2026-05-01T00:00:00`。
    - `updatedAtFrom` / `updatedAtTo`：可选，ISO 本地时间。
  - 成功响应 data：

```json
{
  "items": [
    {
      "id": 1,
      "username": "admin",
      "email": "admin@eventhub.local",
      "status": "ENABLED",
      "roles": ["ADMIN", "USER"]
    }
  ],
  "page": 1,
  "size": 20,
  "total": 1,
  "totalPages": 1,
  "hasNext": false,
  "hasPrevious": false
}
```

- `PATCH /api/v1/admin/users/{userId}/status`
  - 鉴权：必须携带有效 Bearer access token。
  - 授权：必须拥有 `ADMIN` 角色。
  - path 参数：
    - `userId`：正整数。
  - 请求体：

```json
{
  "status": "DISABLED"
}
```

  - 成功响应 data：

```json
{
  "id": 2,
  "username": "alice",
  "email": "alice@example.com",
  "status": "DISABLED",
  "roles": ["USER"]
}
```

- 错误码 / 异常场景：
  - 未登录、access token 缺失、过期、篡改、用户不存在或用户已禁用：`AUTH-401`，消息 `请先登录或重新登录`。
  - 已登录但缺少 `ADMIN` 角色：`AUTH-403`，消息 `权限不足`。
  - `page < 1`、`size < 1`、`size > 100`、超大 `page` 导致 offset 无法安全下推到 SQL、筛选字段超长、非法 status、非法时间格式、时间 from 晚于 to：`COMMON-400`。
  - PATCH 请求体缺失、JSON 格式错误、`status` 非字符串、`status` 非 `ENABLED` / `DISABLED`、大小写不匹配：`COMMON-400`，消息 `请求体格式不合法` 或 `请求体参数校验失败`。
  - PATCH `status` 缺失或 `null`：`COMMON-400`，字段错误 `status 不能为空`。
  - PATCH `userId` 不是正整数：`COMMON-400`。
  - 目标用户不存在：`COMMON-404`，消息 `用户不存在`。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - 路径、方法、字段名、分页字段、状态值和核心错误码对齐 Java。
  - Go 当前没有 OpenAPI 文件，因此本次不更新 OpenAPI；通过 handler 集成测试和 parity matrix 记录契约。
  - Go 版不使用 `@PreAuthorize`，用 `RequireRole("ADMIN")` middleware 表达管理员角色门禁。

## 7. 数据设计
- 表结构调整：
  - 无。
- 索引设计：
  - 继续使用现有约束和索引：
    - `uk_users_username`
    - `uk_users_email`
    - `uk_roles_code`
    - `uk_user_roles_user_role`
    - `idx_user_roles_role_id`
  - `ORDER BY created_at DESC, id DESC` 当前没有组合索引，沿用 Java 版阶段取舍，后续按数据量评估 `(created_at, id)` 或 `(status, created_at, id)`。
  - `username` / `email` 包含匹配使用 `LIKE '%xxx%'`，普通 BTree 索引不能充分利用，当前阶段作为低频管理后台查询可接受。
- 唯一约束：
  - 不变。
- migration 计划：
  - 无新增 migration。
- sqlc query / generated model 影响：
  - 复用当前已有 SQL：
    - `CountUsersByCriteria`
    - `FindUsersPageByCriteria`
    - `UpdateUserStatus`
    - `FindRoleCodesByUserIDs`
  - 若 sqlc generated code 与查询文件不一致，运行 `sqlc generate` 刷新。
- 数据一致性考虑：
  - 列表读取使用 `COUNT + page query + role batch query`，不包事务快照；数据变化时可能出现轻微读偏差，管理端列表可接受。
  - 状态更新在 service 层使用事务包裹 `UPDATE users.status` 与回读用户摘要，减少更新后回读期间的响应不一致，并为后续审计日志、通知、会话失效或缓存清理预留同一事务边界。

## 8. 关键流程
- `GET /api/v1/admin/users` 正常流程：
  1. 请求进入全局 requestId / recover middleware。
  2. 进入认证路由组，Auth middleware 解析 Bearer access token。
  3. Auth middleware 根据 JWT `sub` 回库加载 `users` 和角色编码；用户不存在或非 `ENABLED` 返回 `AUTH-401`。
  4. Auth middleware 将 `security.Principal` 写入 context。
  5. `RequireRole("ADMIN")` 检查 `Principal.Authorities` 是否包含 `ROLE_ADMIN`；缺少则返回 `AUTH-403`。
  6. Handler 解析 query 参数，校验分页、字段长度、状态和时间范围。
  7. Handler 映射为 `usersvc.AdminUserListQuery`。
  8. Service 规范化筛选条件，创建 `page.Request`，并在查询数据库前校验 offset 不会溢出或超过 repository/sqlc 的 `int32` 参数边界。
  9. Service 调用 `UserRepository.CountByCriteria`。
  10. 总数为 0 时直接返回空 `page.Response`。
  11. Service 调用 `UserRepository.ListUsers` 查询当前页用户。
  12. Service 提取当前页用户 ID，调用 `RoleRepository.FindRoleCodesByUserIDs` 批量加载角色。
  13. Service 按 userID 分组角色，组装 `usersvc.UserResult` 列表和 `page.Response`。
  14. Handler 映射为 HTTP DTO 并写出统一成功响应。
- `PATCH /api/v1/admin/users/{userId}/status` 正常流程：
  1. 前置认证和 `RequireRole("ADMIN")` 同列表接口。
  2. Handler 解析正整数 `userId`。
  3. Handler 解码 JSON 请求体，严格校验 `status` 只能是 `ENABLED` / `DISABLED`。
  4. Handler 映射为 `usersvc.UpdateUserStatusCommand`。
  5. Service 在事务内调用 `UserRepository.UpdateStatus`。
  6. 受影响行数为 0 时返回 `COMMON-404` / `用户不存在`。
  7. Service 在同一事务内回读用户和角色，返回用户摘要。
  8. Handler 写出统一成功响应。
- 异常流程：
  - 无 token 或 token 无效：认证 middleware 返回 `AUTH-401`。
  - USER 访问 admin API：RBAC middleware 返回 `AUTH-403`。
  - 禁用用户旧 token 访问任意受保护接口：认证 middleware 回库发现 `DISABLED` 后返回 `AUTH-401`。
  - query 或 body 校验失败：handler 返回 `COMMON-400`。
  - 目标用户不存在：service 返回 `COMMON-404`。
- handler / service / repository / sqlc/database 分工：
  - handler：HTTP query/path/body 解析、HTTP DTO 校验、DTO 与 service Command/Query/Result 映射、写响应。
  - service：业务规则、筛选规范化、分页响应组装、批量角色分组、状态更新事务边界和业务错误语义。
  - repository：表达持久化语义，隐藏 sqlc 参数和结果映射。
  - sqlc/database：执行参数化 SQL，不承载业务判断。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：
  - 不涉及库存、订单或票务扣减。
- 如何防重复提交：
  - `GET` 查询天然幂等。
  - `PATCH status` 对同一用户设置同一状态是结果幂等；当前不额外做幂等键。
- 事务边界：
  - 列表查询不启用事务快照，避免管理端读接口占用不必要的事务资源。
  - 状态更新启用 service 层事务，覆盖 `UPDATE + 回读响应`。
- 缓存：
  - 本次不缓存用户状态、角色或管理员列表。
  - 认证 middleware 每次受保护请求回库加载最新状态和角色，保证禁用用户旧 token 被拒绝。
  - 后续如引入 Redis 短缓存，必须设计管理员更新状态和角色变更后的失效策略。

## 10. 权限与安全
- 哪些角色能访问：
  - `GET /api/v1/admin/users`：仅 ADMIN。
  - `PATCH /api/v1/admin/users/{userId}/status`：仅 ADMIN。
- 鉴权与鉴别约束：
  - 认证仍由 Bearer access token 完成。
  - 授权仍以服务端实时加载的 `Principal.Authorities` 为准。
  - `RequireRole("ADMIN")` 接收业务角色名 `ADMIN`，内部规范化为 `ROLE_ADMIN` 对比。
  - 缺少认证主体时返回 `AUTH-401`；主体存在但缺少角色时返回 `AUTH-403`。
- JWT claim 边界：
  - 不把角色、权限、邮箱、用户名、用户状态写入 JWT。
  - JWT 只作为身份与 token 技术声明；角色和状态来自服务端权威数据。
- 是否涉及敏感信息、审计或操作日志：
  - 响应不暴露 `passwordHash`、refresh token hash、session 状态或内部角色 ID。
  - 本次暂不写审计日志；后续管理员状态变更应补操作日志，记录操作者、目标用户、旧状态、新状态、requestId 和时间。
- 为什么暂不做 permissions 或资源级权限：
  - 当前阶段只有账号和管理员用户列表，业务资源如活动、票种、订单、场次尚未稳定。
  - USER / ADMIN 足以支撑阶段 1 的普通用户和管理端边界。
  - 过早设计权限点、菜单、按钮或数据范围容易空转，并且会牵动前端、审计和资源归属模型。
  - `roles` / `user_roles` 的多对多结构已经为后续扩展 permissions 或资源级授权预留数据基础。

## 11. 测试策略
- 单元测试：
  - RBAC middleware：
    - Principal 包含 `ROLE_ADMIN` 时放行。
    - Principal 只包含 `ROLE_USER` 时返回 `AUTH-403`。
    - 缺少 Principal 时返回 `AUTH-401`。
  - user service：
    - `ListUsers` 分页筛选时调用用户列表查询和角色批量查询。
    - 空页不发起角色批量查询。
    - 超大 `page` 在 COUNT 和 SQL 查询前返回 `COMMON-400`，避免 offset 溢出或截断。
    - `UpdateStatus` 不存在用户返回 `COMMON-404`。
- service / repository 测试：
  - MySQL repository 集成测试继续覆盖用户筛选分页、状态更新、角色批量查询。
  - 如 repository 方法命名调整，同步更新 fake repository 与集成测试。
- migration / sqlc 验证：
  - 本次不改 migration。
  - 如果 query 文件或 sqlc generated code 变化，运行 `sqlc generate`。
- 接口验证：
  - `USER` 访问 `GET /api/v1/admin/users` 返回 `AUTH-403`。
  - `ADMIN` 访问列表成功并返回分页字段。
  - `page=1&size=1` 限制当前页数量，并返回 `hasNext/hasPrevious`。
  - `username/email/status` 筛选命中目标用户。
  - `createdAtFrom/createdAtTo/updatedAtFrom/updatedAtTo` 时间范围可用。
  - `PATCH status` 成功返回更新后的用户摘要。
  - 管理员禁用用户后，该用户旧 access token 请求 `/api/v1/me` 返回 `AUTH-401`。
- OpenAPI validate：
  - 当前 Go 仓库没有 OpenAPI 文件和 validate 命令，本次不运行。
- 异常场景验证：
  - 未登录访问 admin API：`AUTH-401`。
  - USER 访问 admin API：`AUTH-403`。
  - `page=0`、`size=101`、超大 `page`、`status=LOCKED`、时间范围反向：`COMMON-400`。
  - PATCH `status=null`：`COMMON-400`，字段错误 `status 不能为空`。
  - PATCH `status=LOCKED`、`status=disabled` 或数字：`COMMON-400`。
  - PATCH 不存在用户：`COMMON-404`。
- Java-Go parity 验证：
  - 对照 Java controller、request、service、mapper 和 integration test。
  - 更新 `docs/ai/parity/java-go-parity-matrix.md` 的管理员用户 API 与 RBAC 行。
- 需要运行的命令：
  - `gofmt`。
  - `go test ./...`。
  - 如 sqlc generated code 变化：`sqlc generate`。

## 12. 风险与替代方案
- 当前方案风险：
  - 每个受保护请求都回库加载用户状态和角色，高并发下会增加 DB 压力。
  - 管理员列表的 `COUNT(*)`、`LIKE '%xxx%'` 和缺少组合索引在大数据量下可能变慢。
  - 列表总数、当前页和角色批量查询不是同一个快照，数据变化时可能有轻微读偏差。
  - 状态更新后当前不主动吊销 auth session 或 access token denylist；但下一次 access token 认证会因用户状态回库检查被拒绝。
- 备选方案 A：把 roles 写入 JWT，RBAC middleware 只验 token。
  - 优点：每个请求少查角色。
  - 未采用原因：用户禁用或角色变更后旧 token 会带着旧权限继续生效，不符合 Java-Go 对齐和 ADR-0011。
- 备选方案 B：新增 permissions / role_permissions / resource_acl。
  - 优点：可表达更细粒度权限。
  - 未采用原因：当前活动、订单、票种等资源模型尚未展开，USER / ADMIN 已满足阶段目标；提前做资源级权限会放大复杂度。
- 备选方案 C：只在每个 admin handler 内手写角色判断。
  - 优点：改动更小。
  - 未采用原因：后续管理接口会持续增加，集中 middleware 更容易复用和测试，也更接近 Java URL 级 `hasRole("ADMIN")` 语义。
- 备选方案 D：在 UserRepository 中返回用户和角色聚合结果。
  - 优点：service 少一次角色仓储调用。
  - 未采用原因：角色关系属于 `RoleRepository` 持久化边界，Go 版保留用户查询与角色查询的仓储职责分离；service 聚合当前页角色可以清楚表达避免 N+1 的业务意图。
- 备选方案 E：为管理员列表新增缓存。
  - 优点：降低重复查询成本。
  - 未采用原因：用户状态和角色变化会影响列表，当前缺少可靠失效策略；管理端列表不是高频读热点。
- 备选方案 F：为状态更新新增用户级 token version 或 access token denylist。
  - 优点：可更明确地失效旧 access token。
  - 未采用原因：当前已通过每次受保护请求回库检查用户状态满足禁用用户旧 token 被拒绝；token version / denylist 需要额外 schema、缓存或清理策略，留待会话管理强化阶段。
- 后续可演进点：
  - 为用户列表增加 `(created_at, id)`、`(status, created_at, id)` 索引评估。
  - 增加管理员操作日志。
  - 设计角色管理接口和权限点模型。
  - 引入受控短缓存时同步设计禁用用户与角色变更失效。
