# Auth Register Login Access Token 设计

## 1. 背景
- 当前 Go 版 EventHub 已具备 HTTP foundation、统一响应、错误码、MySQL migration、sqlc、repository 和事务底座，但尚未实现注册、登录、JWT access token、当前用户查询和认证 middleware。
- Java 版对应语义和契约来源：
  - `docs/ai/parity/java-auth-api-contract.md`
  - Java `AuthController`、`UserController`、`RegisterRequest`、`LoginRequest`、`LoginResponse`、`UserInfo`
  - Java `AuthServiceImpl`、`TokenServiceImpl`、`AuthSessionServiceImpl`
  - Java `JwtClaims`、`JwtCodec`、`JwtAuthenticationFilter`、`AuthenticatedPrincipalService`
  - Java `AuthIntegrationTest` 和 `JwtCodecTest`
- 业务上下文是活动预约与票务平台的身份基础能力：用户可注册、登录后获得短期 access token 和一次性返回的 opaque refresh token，受保护接口通过 Bearer token 建立当前用户上下文。

## 2. 目标
- 实现：
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `GET /api/v1/me`
- 注册流程对齐 Java：
  - 校验 `username/email/password`
  - 创建 `ENABLED` 用户
  - 默认绑定 `USER` 角色
  - username/email 唯一性冲突映射 `AUTH-409`
  - 并发唯一性最终依赖 DB unique constraint
- 登录流程对齐 Java：
  - 支持用户名或邮箱登录
  - 账号不存在和密码错误统一 `AUTH-401` / `账号或密码错误`
  - 禁用用户登录返回 `AUTH-403` / `用户已被禁用`
  - 登录成功创建 `ACTIVE` auth session
  - 返回 access token、refresh token、authorization scheme、过期秒数、session id 和用户摘要
- JWT access token claims 只包含 `iss/sub/iat/exp/jti/sid/typ=access`。
- JWT 不保存角色、权限、邮箱、用户名、用户状态。
- refresh token 明文只返回一次，DB 只保存 `sha256:<hex>`。
- 受保护请求每次按 `sub` 加载最新用户状态和角色。
- 禁用用户持有旧 token 也要被拒绝。
- Go `context.Context` 中传递 `security.Principal`。
- 成功标准：
  - Go 测试覆盖注册、重复注册、登录、错误密码、`/me`、禁用用户旧 token。
  - `go test ./...`、`go vet ./...` 和指定 race 测试可运行。

## 3. 非目标
- 本次不实现 `POST /api/v1/auth/refresh`。
- 本次不实现 `POST /api/v1/auth/logout`。
- 本次不实现管理员用户列表、状态更新或 RBAC 管理端接口。
- 本次不引入 Redis denylist，也不让 access token 立即受 `auth_sessions.status` 吊销影响；保持 Java 当前“access token 解析后按 sub 查用户状态和角色”的语义。
- 本次不新增 OpenAPI YAML；Java OpenAPI 信息只沉淀到 parity contract，Go OpenAPI 生成和 validate 后续单独迁移。
- 本次不逐行迁移 Spring Security 配置、Springdoc 或 Java record/annotation 结构。

## 4. 影响范围
- 本次涉及 Go package / 模块：
  - `internal/config`
  - `internal/app`
  - `internal/http/router`
  - `internal/http/server`
  - `internal/http/middleware`
  - `internal/http/handler/auth`
  - `internal/http/handler/user`
  - `internal/http/dto/auth`
  - `internal/http/dto/user`
  - `internal/http/validation`
  - `internal/apperror`
  - `internal/service/auth`
  - `internal/service/user`
  - `internal/security`
  - `internal/security/password`
  - `internal/security/jwt`
  - `internal/security/refresh`
  - `internal/repository`
  - `internal/repository/mysql`
  - `internal/platform/db`
  - `configs`
  - `docs/ai/design`
  - `docs/ai/implementation`
  - `docs/ai/adr`
  - `docs/ai/parity`
- 本次明确不触及：
  - `cmd/eventhub/main.go`
  - `internal/domain`
  - `internal/page`
  - `internal/platform/redis`
  - `internal/repository/mysql/queries`
  - `internal/repository/mysql/sqlc`
  - `migrations`
  - `api/openapi`
- 目录结构检查：
  - 新增业务 handler 使用 `internal/http/handler/auth` 和 `internal/http/handler/user` 子包，不把具体业务 handler 放在 `internal/http/handler` 根目录。
  - 新增 HTTP DTO 使用 `internal/http/dto/auth` 和 `internal/http/dto/user` 子包，不创建 `vo` 包。
  - `internal/http/dto/user` 承载可复用的 `UserInfoResponse`；`internal/http/dto/auth` 的 `LoginResponse` 引用用户响应 DTO，以避免复制响应结构。
  - service contract 使用 `internal/service/auth/{service,command,result,register,login}.go` 和 `internal/service/user/{service,query,result,current_user,principal}.go`。
  - 不创建空 domain package；当前 repository model 到 service result 的映射足够支撑本阶段。
  - 不改 `repository/mysql/queries` 或 sqlc generated code，因为现有查询已覆盖本次用例。
- 本次影响 `docs/ai/parity/java-go-parity-matrix.md`，因为 API 契约、错误码、JWT、安全上下文、auth session、refresh token hash 和测试策略发生变化。

## 5. 领域建模
- `User`
  - 当前由 repository 层模型承接持久化字段：`id/username/email/password_hash/status/created_at/updated_at`。
  - service 输出使用 `UserResult`，只暴露 `id/username/email/status/roles`。
  - 状态保持 Java 字符串：`ENABLED` / `DISABLED`。
- `Role`
  - 使用 `roles.code` 作为业务角色编码，当前至少有 `USER` / `ADMIN`。
  - HTTP 用户摘要返回原始角色编码，例如 `USER`。
  - `Principal.Authorities` 使用 `ROLE_` 前缀，例如 `ROLE_USER`，对齐 Java Spring Security 授权语义。
- `AuthSession`
  - 登录成功创建 `ACTIVE` 会话。
  - `session_id` 写入登录响应和 access token `sid`。
  - `refresh_token_hash` 保存 `sha256:<hex>`。
  - `refresh_expires_at` 由登录时间加 refresh token TTL 得出。
- `AccessToken`
  - JWT，只保存稳定身份和技术 claim。
  - `sub` 对应 `users.id`。
  - `sid` 对应 `auth_sessions.session_id`。
- `RefreshToken`
  - opaque token，32 字节随机数 Base64 URL-safe 无 padding 编码。
  - 不从 token 中解析用户、角色、状态或会话。
- `Principal`
  - Go `context.Context` 中的当前认证主体。
  - 字段为 `UserID`、`Username`、`Authorities`；它来自 DB 实时加载，不来自 JWT payload。

## 6. API 设计
- `POST /api/v1/auth/register`
  - 请求：

```json
{
  "username": "alice",
  "email": "alice@example.com",
  "password": "Password123"
}
```

  - 成功响应 `data`：

```json
{
  "id": 1,
  "username": "alice",
  "email": "alice@example.com",
  "status": "ENABLED",
  "roles": ["USER"]
}
```

- `POST /api/v1/auth/login`
  - 请求：

```json
{
  "usernameOrEmail": "alice",
  "password": "Password123"
}
```

  - 成功响应 `data`：

```json
{
  "accessToken": "...",
  "refreshToken": "...",
  "authorizationScheme": "Bearer",
  "expiresIn": 1800,
  "refreshExpiresIn": 2592000,
  "sessionId": "...",
  "user": {
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "status": "ENABLED",
    "roles": ["USER"]
  }
}
```

- `GET /api/v1/me`
  - Header：`Authorization: Bearer <access_token>`
  - 成功响应 `data` 与 `UserInfoResponse` 一致。
- 错误码 / 异常场景：
  - 注册参数错误：`400 COMMON-400`
  - 注册用户名重复：`409 AUTH-409` / `用户名已存在`
  - 注册邮箱重复：`409 AUTH-409` / `邮箱已存在`
  - 注册并发 unique constraint 兜底：`409 AUTH-409` / `用户名或邮箱已存在`
  - 登录账号不存在或密码错误：`401 AUTH-401` / `账号或密码错误`
  - 登录禁用用户：`403 AUTH-403` / `用户已被禁用`
  - `/me` 缺失、过期、篡改、claim 错误 token：`401 AUTH-401` / `请先登录或重新登录`
  - `/me` 用户不存在或已禁用：`401 AUTH-401` / `请先登录或重新登录`
- 与 Java OpenAPI / controller 契约差异：
  - Go 暂不生成 OpenAPI 文档；字段名、路径、方法、状态码和响应 envelope 先通过测试和 parity contract 锁定。

## 7. 数据设计
- 表结构调整：
  - 本次不新增 migration。
  - 复用 `migrations/000002_auth_schema.up.sql` 中的 `users`、`roles`、`user_roles`、`auth_sessions`。
- 索引设计：
  - 注册唯一性依赖 `uk_users_username`、`uk_users_email`。
  - 默认角色依赖 `uk_roles_code`。
  - 登录创建 auth session 依赖 `uk_auth_sessions_session_id` 和 `uk_auth_sessions_refresh_token_hash` 防碰撞。
- migration 计划：
  - 无新增 migration；实现后运行现有 repository/migration 测试。
- sqlc query / generated model 影响：
  - 使用现有 `CreateUser`、`FindUserByUsernameOrEmail`、`FindUserByID`、`FindRoleByCode`、`FindRoleCodesByUserID`、`AddRoleToUser`、`CreateAuthSession`。
  - 不修改 sqlc 查询，因此不需要 `sqlc generate`。
- 数据一致性考虑：
  - 注册使用 service transaction 包住 user insert 和 role binding。
  - 登录使用 service transaction 包住密码校验后的 session 创建；session 创建失败不返回 token pair。
  - refresh token 明文只在登录调用栈和 HTTP 响应中短暂存在。
  - 密码落库只保存 BCrypt hash。

## 8. 关键流程
- 注册正常流程：
  1. handler decode/validate `RegisterRequest`。
  2. handler 映射到 `RegisterCommand`。
  3. service trim username、lower-case email。
  4. service 预检查 username/email 是否存在，用于返回更具体消息。
  5. service 使用 BCrypt 生成密码 hash。
  6. service 在事务内创建用户并绑定 `USER` 角色。
  7. service 回读用户和角色，返回 `UserResult`。
  8. handler 映射为 `UserInfoResponse`。
- 注册异常流程：
  - 预检查发现 username/email 重复，直接返回 `AUTH-409`。
  - 并发场景下两个请求都通过预检查时，DB unique constraint 兜底，service 映射为 `用户名或邮箱已存在`。
  - 默认 `USER` 角色缺失属于服务端不变量失败，映射 `COMMON-500`。
- 登录正常流程：
  1. handler decode/validate `LoginRequest`。
  2. service trim 登录标识，包含 `@` 时 lower-case。
  3. service 按 username 或 email 查用户。
  4. service 使用 BCrypt 校验密码。
  5. service 校验用户状态必须为 `ENABLED`。
  6. service 生成 `sessionId`、refresh token、`jti`。
  7. service 计算 refresh token hash 和过期时间。
  8. service 创建 `ACTIVE` auth session。
  9. service 签发 access token。
  10. service 返回 `LoginResult`。
- 登录异常流程：
  - 用户不存在和密码错误统一 `AUTH-401`。
  - 禁用用户在密码通过后返回 `AUTH-403`，不创建 session。
  - session 插入失败时不返回 token。
- `/me` 正常流程：
  1. `Auth` middleware 读取 `Authorization`。
  2. JWT codec 校验签名、issuer、exp、`sub/jti/sid/typ`。
  3. middleware 调用 user service 按 `sub` 查询用户和角色。
  4. user service 拒绝不存在或禁用用户。
  5. middleware 将 `Principal` 写入 context。
  6. handler 从 context 读取 `Principal`，调用 user service 查询当前用户摘要。
- 分工：
  - handler：HTTP decode/validate、service command/query 映射、response 映射。
  - middleware：Bearer token 解析、JWT 技术校验、DB 加载 Principal、写 context。
  - service/auth：注册、登录业务规则、事务、错误码映射、token/session 创建。
  - service/user：当前用户查询和 principal 加载。
  - repository：持久化接口。
  - repository/mysql：sqlc row 与 repository model 映射。
  - security：密码、JWT、refresh token、Principal context。

## 9. 并发 / 幂等 / 缓存
- 并发：
  - 注册并发唯一性依赖 DB unique constraint；预检查只用于更友好的字段级错误消息。
  - 登录并发可创建多条 ACTIVE session，符合多设备登录语义。
  - `session_id` / `refresh_token_hash` 碰撞概率极低，但仍由唯一约束兜底。
- 幂等：
  - 注册不是幂等接口；重复账号返回冲突。
  - 登录不是幂等接口；每次成功登录都创建新的 session 和 token pair。
- 缓存：
  - 本次不引入 Redis。
  - 受保护请求每次回 DB 加载用户状态和角色，确保禁用用户旧 token 即时失效。
  - 后续如引入短 TTL 缓存，需要单独设计用户禁用、角色变更和缓存失效策略。

## 10. 权限与安全
- `POST /api/v1/auth/register` 和 `POST /api/v1/auth/login` 匿名可访问。
- `GET /api/v1/me` 需要合法 Bearer access token。
- JWT claim 边界：
  - 写入：`iss/sub/iat/exp/jti/sid/typ=access`。
  - 不写入：角色、权限、邮箱、用户名、用户状态、密码哈希。
- refresh token：
  - 明文只返回一次。
  - DB 保存 `sha256:<hex>`。
  - 默认 TTL 30 天。
  - refresh endpoint 下一阶段实现。
- 密码：
  - 使用 BCrypt。
  - 明文密码不写日志、不进入响应、不落库。
- Principal：
  - 从 DB 当前状态加载，写入 request context。
  - handler/service 通过显式参数或 context 读取，不从 JWT 中读取动态用户属性。
- 风险：
  - 当前 `/me` 不检查 `auth_sessions.status`，对齐 Java 当前 access token 认证模型；服务端单设备吊销需要后续 denylist 或 session 校验设计。

## 11. 测试策略
- 单元测试：
  - `security/password`：hash/matches。
  - `security/refresh`：生成长度、格式、hash 前缀和稳定性。
  - `security/jwt`：生成/解析 required claims，拒绝错误 `typ`、缺失 `jti`、缺失 `sid`、过期 token、篡改 token。
- service / repository 测试：
  - `internal/service/auth` 使用 fake repository 和 no-op transactor 覆盖注册、重复注册、登录、错误密码、禁用用户登录、登录创建 ACTIVE session 和 refresh token hash。
  - repository/mysql 现有 Testcontainers 测试继续覆盖 migration、unique constraint 和 auth session 持久化。
- migration / sqlc 验证：
  - 本次不改 SQL 和 migration，不运行 `sqlc generate`；保留 `go test ./...` 中现有 migration 集成测试。
- 接口验证：
  - HTTP 集成测试使用真实 handler/middleware/service 和 in-memory repository 覆盖注册、重复注册、登录、错误密码、`/me`、禁用用户旧 token。
- OpenAPI validate：
  - Go 版暂无 OpenAPI 文件，本次不运行。
- 异常场景验证：
  - 缺 token、坏 token、禁用旧 token 均返回 `AUTH-401`。
  - 登录失败不创建 session。
- Java-Go parity 验证：
  - 对照 `docs/ai/parity/java-auth-api-contract.md`。
- 需要运行的命令：
  - `gofmt`
  - `go test ./...`
  - `go vet ./...`
  - `go test -race ./internal/service/auth ./internal/http/middleware`
  - `make test` 如时间和环境允许

## 12. 风险与替代方案
- 当前方案的风险：
  - 每次受保护请求都查 DB，会增加延迟；这是为了换取用户禁用和角色变更的即时生效。
  - Go app 启动装配接入 DB 后，如果未配置 MySQL DSN，auth 路由无法在真实进程中工作；配置示例必须同步补齐。
  - 当前不实现 refresh endpoint，登录返回的 refresh token 只能为下一阶段预备。
  - 当前不检查 `auth_sessions.status`，旧 access token 在用户仍启用时不会因 session revoke 立即失效。
- 备选方案：
  - 方案 A：把角色、用户名、邮箱、状态写入 JWT，middleware 不查 DB。
  - 方案 B：JWT 只放 `sub`，不放 `sid/jti/typ`。
  - 方案 C：refresh token 明文或可逆加密后保存到 DB。
  - 方案 D：所有 auth 能力集中在一个 service 文件和 root handler 包。
  - 方案 E：本次直接实现 refresh/logout/admin users。
- 为什么不选备选方案：
  - 不选 A：角色和状态会变成 token 快照，禁用用户和权限变更无法及时生效，也违反 Java ADR 和项目约束。
  - 不选 B：缺少 `jti/sid/typ` 会削弱审计、会话关联和 token 类型边界。
  - 不选 C：refresh token 是长期凭证，数据库泄漏后明文或可逆密文风险过高。
  - 不选 D：会破坏当前 Go package layout 和 service contract 边界，后续扩展 refresh/admin 会迅速拥挤。
  - 不选 E：本次任务明确 refresh endpoint 下一阶段实现；保持最小闭环能更清楚验证 access token 和当前用户安全边界。
- 后续可演进点：
  - 实现 refresh token 轮换和重放检测。
  - logout 吊销当前 session，并评估 Redis denylist。
  - 管理员禁用用户时批量吊销 ACTIVE sessions。
  - 增加 OpenAPI YAML 和 validate。
  - 为用户状态和角色变更引入短 TTL 缓存时补失效策略。
