# 管理员 RBAC 与用户管理实现说明

## 1. 本次改动解决了什么问题

本次补齐 Go 版阶段 1 的管理员用户管理闭环：

- 新增 USER / ADMIN RBAC middleware，普通 USER 访问管理员接口返回 `AUTH-403`。
- 新增 `GET /api/v1/admin/users`，支持分页、筛选、新注册用户优先排序和分页响应元数据。
- 列表页按当前页用户 ID 批量加载角色，避免逐用户查询造成 N+1。
- 新增 `PATCH /api/v1/admin/users/{userId}/status`，管理员可启用或禁用用户。
- 用户被管理员禁用后，旧 access token 在下一次受保护请求中被 auth middleware 回库校验拒绝。
- 保持 JWT claim 最小边界，不把 roles、email、username、status 写入 JWT。
- 补齐管理员用户列表超大 `page` 的 offset 边界校验，避免 `(page-1)*size` 在 Go `int64` 中溢出，或被截断为 sqlc/repository 使用的 `int32` offset。

## 2. 改动内容
- 新增了什么
  - 设计文档：`docs/ai/design/014-admin-rbac-user-management.md`。
  - ADR：
    - `docs/ai/adr/0016-rbac-user-admin-only.md`
    - `docs/ai/adr/0017-admin-user-list-role-batch-loading.md`
  - `internal/http/middleware/rbac.go`
    - 新增 `RequireRole("ADMIN")` middleware。
  - `internal/http/dto/user/request.go`
    - 新增 `AdminUserListRequest`。
    - 新增 `UpdateUserStatusRequest` 和严格 JSON 状态枚举解码。
  - `internal/http/handler/user/admin_users.go`
    - 新增管理员用户列表和状态更新 handler。
  - `internal/http/handler/user/admin_validation.go`
    - 新增 query/path/body 校验辅助。
  - `internal/service/user/command.go`
    - 新增 `UpdateUserStatusCommand`。
  - `internal/service/user/admin_users.go`
    - 新增 `Service.ListUsers` 和 `Service.UpdateStatus`。
    - 在 `ListUsers` 进入 COUNT 和分页 SQL 前计算并校验 offset，超出 `int32` 可表达范围时返回 `COMMON-400`。
  - 测试：
    - `internal/http/middleware/rbac_test.go`
    - `internal/service/user/admin_users_test.go`
    - 扩展 `internal/http/auth_integration_test.go` 的管理员 API 集成测试。
    - 新增 service 层超大 `page` 在 COUNT 前被拒绝的回归测试，以及 HTTP 层超大 `page` 返回 `COMMON-400` 的集成测试。
- 修改了什么
  - `internal/http/router.go`
    - 在认证保护组中注册管理员用户接口，并叠加 `RequireRole("ADMIN")`。
  - `internal/service/user/service.go`
    - 注入 `platform/db.TxRunner`，使状态更新在 service 层承载事务边界。
  - `internal/service/user/query.go`
    - 新增管理员用户列表 Query。
  - `internal/service/user/current_user.go`
    - 规范化 `roles` 空列表，避免响应中出现 `null`。
  - `internal/http/handler/user/mapping.go`
    - 新增 service 分页结果到 HTTP 分页响应的映射。
  - `internal/repository/user_repository.go`
    - 将用户分页查询方法命名为 `ListUsers`。
  - `internal/repository/mysql/user_repository.go`
    - 使用已有 sqlc `FindUsersPageByCriteria` 实现 `ListUsers`。
  - `internal/app/providers/user.go`
    - 真实运行时为 user service 注入 MySQL transactor。
  - 测试 fake repository 同步实现 `ListUsers`、角色批量加载和 admin seed 用户。
- 删除了什么
  - 未删除生产能力。
  - repository 接口中的旧 `FindPage` 方法改名为 `ListUsers`。
- 是否更新 Java-Go parity 记录
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
  - 本次触发 API、错误码、repository 行为、业务流程、auth/security 和测试策略 parity 更新。

## 3. 为什么这样设计
- 关键设计原因
  - `RequireRole("ADMIN")` 只读取 `security.Principal`，角色来源仍是 auth middleware 每次按 JWT `sub` 回库加载的最新状态。
  - handler 只负责 HTTP 参数解析和 DTO 映射；分页筛选规范化、角色批量组装、状态更新事务边界放在 service。
  - offset 校验放在 service 层并早于 COUNT，是因为 repository/sqlc 的 `limit/offset` 参数边界属于业务查询下推前的持久化调用约束；提前校验可以避免 `total == 0` 时绕过，也避免先乘法再比较导致溢出。
  - offset 上界使用除法判断 `pageIndex > MaxInt32/size`，先判定再相乘，确保不会在 Go `int64` 中溢出。
  - 状态更新使用 `TxRunner` 包住 `UPDATE + 回读用户摘要`，对齐 Java `@Transactional` 语义，也为后续审计、通知、缓存失效预留边界。
  - 角色批量查询继续放在 `RoleRepository`，因为角色关系属于 `roles/user_roles` 持久化语义，不混入用户仓储返回 DTO。
  - PATCH 状态请求使用自定义 JSON 解码，拒绝 `LOCKED`、`disabled` 和数字 ordinal，尽量贴近 Java 枚举反序列化失败语义。
- 与 Go 项目当前阶段的匹配点
  - 路由层只绑定 URL、method 和 middleware，不创建业务对象。
  - handler 直接依赖具体 `*usersvc.Service`，不新增重复接口。
  - repository interface 保留为 service 到 sqlc/database 的持久化边界。
  - 没有创建空 domain/admin package；管理员用户能力仍归属 user 模块。
  - 没有引入重量级依赖、Redis 缓存、权限点系统或 token denylist。
- 与 Java 版业务语义的对齐方式
  - API 路径、方法、字段名、分页元数据、状态值和核心错误码对齐 Java。
  - 列表查询沿用 `COUNT + LIMIT/OFFSET + ORDER BY created_at DESC, id DESC`。
  - 角色批量加载对齐 Java `RoleMapper.findRoleCodesByUserIds`。
  - 禁用用户旧 access token 被拒绝继续依赖请求期回库校验。

## 4. 替代方案
- 方案 A：把 roles 写入 JWT，让 RBAC middleware 只验 token。
  - 没有采用。角色和用户状态会变化，旧 token 会保留过期权限；这违反 ADR-0011 和禁用用户旧 token 即时拒绝目标。
- 方案 B：提前引入 permissions / role_permissions / 资源级权限。
  - 没有采用。当前资源模型尚未稳定，USER / ADMIN 已足够支撑阶段 1 管理端边界；详细原因见 ADR-0016。
- 方案 C：在每个 admin handler 中手写角色判断。
  - 没有采用。后续管理接口会增加，集中 middleware 更可复用，也更接近 Java URL 级 `hasRole("ADMIN")`。
- 方案 D：用户列表逐个调用 `FindRoleCodesByUserID`。
  - 没有采用。会产生 N+1 查询，详细原因见 ADR-0017。
- 方案 E：让 UserRepository 直接返回用户和角色聚合 DTO。
  - 没有采用。repository 不应返回 HTTP DTO；角色关系属于 `RoleRepository`，service 聚合更符合当前分层。
- 方案 F：为状态更新引入 access token denylist 或 token version。
  - 没有采用。当前通过回库检查用户状态已经满足禁用用户旧 token 被拒绝；denylist/token version 需要额外 schema、缓存和清理策略。
- 方案 G：保留在 `page.Request.Offset()` 之后再比较 `math.MaxInt32`。
  - 没有采用。先乘法再比较会在极大 `page` 下先发生 `int64` 溢出；并且原有实现会在 `total == 0` 时提前返回，导致超大页码没有被拒绝。

## 5. 测试与验证
- 跑了哪些测试
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `make test`：通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w`：已对改动 Go 文件运行。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `make test`：通过。
  - `golangci-lint run`：已尝试，当前环境未安装 `golangci-lint`，返回 `command not found`。
  - `sqlc generate`：未运行；本次没有修改 SQL query、schema 或 sqlc 配置，复用的 `FindUsersPageByCriteria`、`UpdateUserStatus`、`FindRoleCodesByUserIDs` 已存在且生成代码可编译。
  - migration 测试：本次未修改 migration；`go test ./...` 已覆盖 `internal/repository/mysql` Testcontainers 集成测试。
  - OpenAPI validate：未运行；Go 版当前没有 OpenAPI 契约文件，本次未修改 `api/openapi`。
- 手工验证了哪些场景
  - `rg` 确认生产代码已使用 `UserRepository.ListUsers`，旧 `FindPage` 不再存在。
  - 检查 router 中 admin 路由位于 auth middleware 保护组内，并叠加 `RequireRole("ADMIN")`。
  - 检查 JWT codec 未修改，仍不写 roles、email、username、status。
- Java-Go parity 如何验证
  - 对照 Java `AdminUserController`、`AdminUserQueryRequest`、`UpdateUserStatusRequest`、`AuthServiceImpl.listUsers/updateStatus`、`UserMapper.xml`、`RoleMapper.xml` 和 `AuthIntegrationTest`。
  - 更新 parity matrix 的管理员用户 API、JWT/RBAC 安全边界、认证错误与安全响应行。
- 结果如何
  - HTTP 集成测试覆盖 USER 403、ADMIN 列表成功、分页、筛选、状态更新和禁用用户旧 token 拒绝。
  - HTTP 集成测试覆盖超大 `page` 返回 `COMMON-400`。
  - service 测试覆盖批量角色加载、空页跳过角色查询、超大 `page` 在 COUNT 前返回 `COMMON-400`、状态更新事务和用户不存在错误。
  - middleware 测试覆盖 ADMIN 放行、USER 403、缺少 principal 401。

## 6. 已知限制
- 当前版本还缺什么
  - 没有角色管理接口，不能动态授予或收回 ADMIN。
  - 没有 permissions、资源级权限和数据范围权限。
  - 管理员状态更新没有写审计日志。
  - 状态更新后不主动吊销 auth session，也不写 access token denylist。
  - 当前 MySQL 分页 offset 通过 sqlc 生成的 `int32` 参数下推，超过该边界的深分页请求会返回 `COMMON-400`；如后续确需更深分页，应重新设计 repository/sqlc 参数类型或改用游标分页。
  - Go 版仍没有 OpenAPI 契约文件。
- 哪些地方后面需要继续演进
  - 活动、订单、票种等资源模型稳定后，单独设计 permissions 或资源级授权。
  - 增加管理员操作日志。
  - 评估用户列表索引，例如 `(created_at, id)` 或 `(status, created_at, id)`。
  - 如引入 Redis 权限缓存，需要设计禁用用户和角色变更后的失效策略。
- 与 Java 版仍有哪些差距
  - Java 版有 Spring Security URL 规则和 `@PreAuthorize` 双层保护；Go 版当前用 auth route group + `RequireRole` 表达等价业务语义。
  - Java 版 OpenAPI 注解尚未迁移到 Go OpenAPI 文件。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 补齐账号体系中的管理员后台边界，展示 RBAC、分页筛选、N+1 优化和禁用用户旧 token 拒绝的完整链路。
- 对微服务 / 云原生演进的影响
  - 当前用户状态和角色以数据库为权威来源，后续拆分服务时可把 Principal 加载策略抽到认证服务或网关侧。
  - 未把 roles 写入 JWT，避免跨服务传播过期权限快照。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续 `/api/v1/admin/**` 管理接口可复用 `RequireRole("ADMIN")`。
  - 用户列表筛选和 `page.Response` 可复用于活动、订单、票种等管理端列表。
  - 若新增权限表、审计表或 token version，需要同步 migration、sqlc、repository、service 测试和 parity matrix。
