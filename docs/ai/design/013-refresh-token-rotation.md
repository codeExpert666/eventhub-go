# Refresh Token 轮换、重放检测与 Logout No-op 设计

## 1. 背景
- 当前 Go 版已实现 `POST /api/v1/auth/register`、`POST /api/v1/auth/login`、`GET /api/v1/me`、JWT access token 签发与解析、opaque refresh token 生成、`auth_sessions` 持久化和条件轮换 SQL 底座。
- 登录接口已经返回 `accessToken`、`refreshToken`、`authorizationScheme`、`expiresIn`、`refreshExpiresIn`、`sessionId` 和 `user`，并且 DB 只保存 `sha256:<hex>` refresh token hash。
- 当前缺少 refresh API，access token 过期后客户端只能重新登录；同时缺少 logout API，无法对齐 Java 版“已认证 no-op 登出入口”的协议语义。
- Java 版对应来源：
  - `backend/src/main/java/com/eventhub/modules/auth/controller/AuthController.java`
  - `backend/src/main/java/com/eventhub/modules/auth/dto/request/RefreshTokenRequest.java`
  - `backend/src/main/java/com/eventhub/modules/auth/vo/TokenPairResponse.java`
  - `backend/src/main/java/com/eventhub/modules/auth/service/impl/AuthServiceImpl.java`
  - `backend/src/main/java/com/eventhub/modules/auth/service/RefreshTokenParser.java`
  - `backend/src/main/java/com/eventhub/modules/auth/service/RefreshTokenHasher.java`
  - `backend/src/main/resources/mapper/auth/AuthSessionMapper.xml`
  - `backend/src/main/resources/db/migration/V3__create_auth_sessions.sql`
- 业务上下文：refresh token 是长期认证凭证。每次 refresh 成功后必须轮换新的 refresh token，使旧 token 立即失效；同一个旧 token 在并发提交或重放时最多成功一次。

## 2. 目标
- 实现 `POST /api/v1/auth/refresh`。
- 实现 `POST /api/v1/auth/logout`。
- 对齐 Java 版 refresh request/response 契约：
  - request 字段：`refreshToken`。
  - response 字段：`accessToken`、`refreshToken`、`authorizationScheme`、`expiresIn`、`refreshExpiresIn`、`sessionId`、`user`。
- refresh token 继续使用 opaque random token，不使用 JWT，不从 refresh token 中解析用户、角色、session 或权限。
- DB 只保存 `sha256:<hex>`；服务端不保存 refresh token 明文。
- refresh 成功后旧 token 立即失效，`auth_sessions.version` 执行 `version + 1`。
- 使用数据库条件更新保证并发一致性，不使用 Go mutex、进程内锁或 Redis 锁作为正确性基础。
- 并发提交同一个旧 refresh token 时，最多一个请求成功。
- 旧 refresh token 重放返回 `AUTH-401`。
- 当前检测到旧 token 重放时只返回 `AUTH-401`，不自动吊销已经轮换出的新会话。
- logout 当前 no-op，不修改 DB，仅要求请求已认证。
- 成功标准：
  - refresh 成功可返回新的 token pair。
  - 新 refresh token 可继续 refresh。
  - 旧 refresh token 轮换后再次使用失败。
  - expired、revoked、disabled user 均返回 `AUTH-401`。
  - `go test -race ./internal/service/auth` 中同一旧 token 并发 N 次最多一次成功。

## 3. 非目标
- 不修改 JWT access token claim；仍只包含 `iss/sub/iat/exp/jti/sid/typ=access`。
- 不把角色、邮箱、用户名、用户状态写入 JWT。
- 不把 refresh token 改为 JWT。
- 不新增 refresh token 历史表、token family 表或 reuse audit 表。
- 不在本次实现旧 token 重放后的新会话自动吊销。
- 不让 logout 吊销 `auth_sessions`、写 access token denylist 或修改 DB。
- 不新增 Redis、缓存、分布式锁或 Go mutex 来保证 refresh 并发正确性。
- 不改表结构、索引、唯一约束或 migration；复用现有 `auth_sessions`。
- 不逐行迁移 Java/Spring/MyBatis 结构；Go 版继续使用 `handler -> service -> repository -> sqlc/database`。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `internal/http/dto/auth`
  - `internal/http/handler/auth`
  - `internal/http/router.go`
  - `internal/service/auth`
  - `internal/repository`
  - `internal/repository/mysql`
  - `internal/repository/mysql/queries`
  - `internal/repository/mysql/sqlc`
  - `internal/http` 和 `internal/service/auth` 测试
- 重要的不触碰包：
  - `internal/security/jwt`：不改 claim、签名、解析和 middleware token 语义。
  - `internal/security/refresh`：已有 opaque token 生成、格式校验和 `sha256:<hex>` hash 能力，除非测试发现缺口，否则不改。
  - `migrations`：不改 schema。
  - `internal/domain`：当前阶段没有独立 domain model 需要新增。
- 涉及 API：
  - 新增公开匿名入口 `POST /api/v1/auth/refresh`，因为 access token 可能已经过期。
  - 新增受保护入口 `POST /api/v1/auth/logout`，需要通过 access token 认证。
- 涉及表：
  - 复用 `auth_sessions`，更新 `refresh_token_hash`、`refresh_expires_at`、`last_refreshed_at`、`last_seen_at`、`version`、`updated_at`。
- 不涉及缓存或外部接口。
- 影响 `docs/ai/parity/java-go-parity-matrix.md`：是。本次迁移 Java 已有 auth refresh/logout API、错误语义、repository 条件更新和测试策略。

## 5. 领域建模
- 核心实体：
  - `AuthSession`：服务端认证会话，是 refresh token hash、状态、过期时间、乐观锁版本和 sessionId 的权威记录。
  - `User`：会话所属用户，refresh 期间必须回读最新状态；禁用用户不能继续续期。
  - `RefreshToken`：客户端持有的 opaque 长期凭证，服务端只在请求/响应调用栈短暂处理明文，持久化前转换为 `sha256:<hex>`。
- 实体关系：
  - 一个 `AuthSession` 归属一个 `User`。
  - 一个 `AuthSession` 当前只对应一个有效 refresh token hash。
  - JWT access token 的 `sid` claim 指向 `auth_sessions.session_id`，但 refresh 身份来源只来自 refresh token hash 匹配到的服务端会话。
- 关键状态：
  - `ACTIVE`：会话有效，且 `refresh_expires_at > now` 时才允许 refresh。
  - `REVOKED`：会话已吊销，refresh 必须失败。
  - 过期由 `refresh_expires_at` 派生，不新增 `EXPIRED` 状态。
- 与 Java 版领域对象的对应关系：
  - Java `AuthSessionEntity` 对应 Go `repository.AuthSession`。
  - Java `AuthSessionStatus.ACTIVE/REVOKED` 对应 Go `repository.AuthSessionStatusActive/Revoked`。
  - Java `RefreshTokenRequest` 对应 Go `authdto.RefreshTokenRequest`。
  - Java `TokenPairResponse` 对应 Go `authdto.TokenPairResponse`。
  - Java `AuthServiceImpl.refresh/logout` 对应 Go `auth.Service.Refresh/Logout`。
  - Java `AuthSessionMapper.rotateRefreshToken` 对应 Go `AuthSessionRepository.ConditionalRotate` 与 sqlc query。

## 6. API 设计
- `POST /api/v1/auth/refresh`
  - 鉴权：匿名放行。
  - 请求体：

```json
{
  "refreshToken": "..."
}
```

  - 字段规则：
    - `refreshToken` 必填、非空、最大长度 128。
    - service 层继续做防御性格式校验：32 字节随机数的 Base64 URL-safe 无 padding 编码，长度固定 43，只允许 `A-Z`、`a-z`、`0-9`、`-`、`_`。
  - 成功响应 data：

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

- `POST /api/v1/auth/logout`
  - 鉴权：必须已认证。
  - 请求体：无。
  - 成功响应：统一成功 envelope，data 为空或 null，表达客户端应删除本地 token。
- 错误码 / 异常场景：
  - refresh 请求体缺失、JSON 格式错误或字段校验失败：`COMMON-400`。
  - refresh token 格式非法、hash 查不到、session 非 ACTIVE、refresh 过期、条件更新失败、用户不存在或用户禁用：统一返回 `AUTH-401`，消息对齐 Java：`refresh token 无效或已过期`。
  - logout 未认证：`AUTH-401`，沿用 auth middleware 的 `请先登录或重新登录`。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - 字段名、路径、HTTP method 和主要错误码对齐 Java。
  - Go 当前尚未维护 OpenAPI 文件，因此本次不更新 `api/openapi/eventhub.yaml`；契约通过 DTO、handler 测试和 parity matrix 记录。

## 7. 数据设计
- 表结构调整：无，复用现有 `auth_sessions`。
- 索引设计：无新增索引，复用：
  - `uk_auth_sessions_session_id`
  - `uk_auth_sessions_refresh_token_hash`
  - `idx_auth_sessions_user_id`
  - `idx_auth_sessions_status`
  - `idx_auth_sessions_refresh_expires_at`
- 唯一约束：
  - `refresh_token_hash` 唯一，保证一个 refresh token 只能定位一个会话。
  - `session_id` 唯一，支持按服务端会话做条件轮换。
- migration 计划：无。
- sqlc query / generated model 影响：
  - 将现有轮换 SQL 的 repository 语义命名调整为 `ConditionalRotate`，使 service 层表达“条件命中才轮换”的业务含义。
  - sqlc query 保持条件更新：

```sql
UPDATE auth_sessions
SET refresh_token_hash = ?,
    refresh_expires_at = ?,
    last_refreshed_at = ?,
    last_seen_at = ?,
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE session_id = ?
  AND refresh_token_hash = ?
  AND version = ?
  AND status = 'ACTIVE'
  AND refresh_expires_at > ?;
```

  - 语义对应用户要求的条件：

```sql
WHERE session_id = oldSessionId
  AND refresh_token_hash = oldHash
  AND version = oldVersion
  AND status = 'ACTIVE'
  AND refresh_expires_at > now
```

- 数据一致性考虑：
  - service 先按旧 hash 查询 session 快照，再用旧 sessionId、旧 hash、旧 version、ACTIVE、未过期条件执行单条 UPDATE。
  - 只有 UPDATE 受影响行数为 1 时才视为轮换成功。
  - 成功后旧 hash 被新 hash 替换，旧 token 立即无法再次查到当前 ACTIVE 会话。

## 8. 关键流程
- 正常流程：
  1. Handler 解码 `RefreshTokenRequest` 并做 HTTP 字段校验。
  2. Handler 映射为 `auth.RefreshCommand`。
  3. Service 对 refresh token 做 opaque 格式校验。
  4. Service 计算旧 refresh token hash。
  5. Repository 按旧 hash 查询 `AuthSession`。
  6. Service 校验 session 存在、状态为 `ACTIVE`、`refresh_expires_at > now`。
  7. Service 按 session.user_id 回读用户，并要求用户状态为 `ENABLED`。
  8. Service 读取用户摘要和角色，用于响应。
  9. Service 生成新 refresh token，计算新 hash 和新的 `refresh_expires_at`。
  10. Repository 执行 `ConditionalRotate` 条件更新。
  11. 受影响行数为 1 后，Service 签发新的 access token，并返回 token pair。
- 异常流程：
  - 格式非法：不查询 DB，返回 `AUTH-401`。
  - 旧 hash 查不到：返回 `AUTH-401`，覆盖篡改 token 和旧 token 重放。
  - 状态为 `REVOKED` 或已过期：返回 `AUTH-401`。
  - 用户不存在或禁用：返回 `AUTH-401`。
  - 条件更新受影响行数为 0：返回 `AUTH-401`，覆盖并发失败和旧快照失效。
- 状态流转：
  - refresh 成功：`ACTIVE -> ACTIVE`，`version + 1`，`refresh_token_hash` 替换为新 hash。
  - refresh 失败：不修改 DB。
  - logout：不修改 DB。
- handler / service / repository / sqlc/database 分工：
  - handler：HTTP DTO decode/validate，映射 Command，写统一响应。
  - service：业务校验、事务边界、token 生成、条件轮换成功判定、错误语义收敛。
  - repository：表达 `FindByRefreshTokenHash`、`ConditionalRotate` 等持久化语义，隐藏 sqlc 参数细节。
  - sqlc/database：执行参数化 SQL，不承载业务判断。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：不涉及库存。
- 如何防重复提交：
  - 同一旧 refresh token 的重复提交依赖 DB 条件更新和唯一 hash 约束保证。
  - 并发请求可能都先读到旧 session 快照，但只有第一条 UPDATE 能命中旧 hash + 旧 version；之后 version 和 hash 已改变，其他请求更新 0 行。
- 事务边界：
  - `Refresh` 在 auth service 事务中执行“查询旧 session、回读用户、生成新 token、条件更新”。
  - 新 access token 在条件更新成功后签发，避免客户端拿到和 DB 状态不一致的 token pair。
- 缓存：
  - 不引入缓存。refresh token 是强一致认证凭证，当前以 MySQL 作为权威记录。
- 不使用 Go mutex：
  - 进程内 mutex 无法覆盖多实例部署，也无法防止跨进程并发。
  - 单条条件 UPDATE 是最终一致性边界，和 Java 版 MyBatis SQL 语义对齐。

## 10. 权限与安全
- 访问权限：
  - `/api/v1/auth/refresh` 匿名放行。
  - `/api/v1/auth/logout` 必须通过 Bearer access token 认证。
- 鉴权与鉴别约束：
  - refresh 身份来源只来自服务端 DB 中旧 refresh token hash 匹配到的 session。
  - 不从 refresh token 明文解析用户 ID、sessionId、角色或状态。
  - refresh 期间回读用户状态，禁用用户统一返回 `AUTH-401`。
- JWT claim 边界：
  - 不改 JWT claim。
  - 不把角色、邮箱、用户名、用户状态写入 JWT。
- 敏感信息：
  - refresh token 明文不落库、不写日志、不进入错误消息。
  - DB 只保存 `sha256:<hex>`。
- 审计与操作日志：
  - 当前不新增审计表。
  - 后续如引入 token family/reuse audit，可记录重放检测和安全响应。
- Logout 安全语义：
  - 当前 access token 无状态，不落库；logout no-op 只表达“客户端删除本地 token”的协议入口。
  - 本次不通过 logout 吊销 refresh session，避免引入尚未设计的多端退出和 access token denylist 边界。

## 11. 测试策略
- 单元测试：
  - `AuthService.Refresh` 成功返回新 token pair，响应字段和 TTL 对齐登录。
  - 旧 refresh token refresh 成功后再次使用返回 `AUTH-401`。
  - 新 refresh token 可以再次 refresh。
  - refresh token 格式非法返回 `AUTH-401`。
  - expired session 返回 `AUTH-401`。
  - revoked session 返回 `AUTH-401`。
  - disabled user 返回 `AUTH-401`。
  - `AuthService.Logout` 要求 principal 存在；存在时 no-op 成功。
- service / repository 测试：
  - service fake repository 实现 `ConditionalRotate`，模拟 DB 条件更新：只有 sessionId、old hash、old version、ACTIVE、未过期全部命中时更新。
  - 并发测试：同一旧 refresh token 并发 N 次调用 `Refresh`，最多一个成功，其余为 `AUTH-401`。
  - MySQL repository integration test 可保留或补充现有条件更新覆盖，确保 sqlc 查询受影响行数语义正确。
- migration / sqlc 验证：
  - 不改 migration。
  - 如调整 query 名称或参数，运行 `make sqlc` 或 `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate`。
- 接口验证：
  - HTTP 集成测试覆盖 `POST /api/v1/auth/refresh` 成功、旧 token 重放失败、`POST /api/v1/auth/logout` 未认证失败和已认证成功。
- OpenAPI validate：
  - 不适用，Go 当前尚无 OpenAPI 契约文件。
- 异常场景验证：
  - JSON/字段校验失败映射 `COMMON-400`。
  - refresh 业务失败收敛 `AUTH-401`。
  - logout 未认证由 middleware 映射 `AUTH-401`。
- Java-Go parity 验证：
  - 对照 Java `AuthController.refresh/logout`、`RefreshTokenRequest`、`TokenPairResponse`、`AuthServiceImpl.refresh/logout`、`AuthSessionMapper.xml`。
  - 更新 `docs/ai/parity/java-go-parity-matrix.md` 的 auth refresh/logout 与 refresh token/session 流程行。
- 需要运行的命令：
  - `gofmt`
  - `go test ./internal/service/auth`
  - `go test -race ./internal/service/auth`
  - `go test ./...`
  - `go vet ./...`
  - `make test`
  - `make vet`
  - `make sqlc`，如果 SQL query 名称或生成代码变化
  - `golangci-lint run`，如果工具可用

## 12. 风险与替代方案
- 当前方案的风险：
  - 不保存历史 hash 时，旧 refresh token 重放只能识别为无效凭证，不能定位并吊销已经轮换后的会话。
  - refresh endpoint 匿名放行，如果客户端同时携带过期 Authorization header，当前 Go router 不会对该路由使用 auth middleware，因此不会被过期 access token 阻断；这与“refresh 依赖 refresh token 而不是 access token”一致。
  - fake repository 并发测试只能验证 service 对 `ConditionalRotate` 结果的处理；真正跨进程一致性仍依赖 MySQL integration test 和 SQL 条件。
- 备选方案：
  - 方案 A：refresh token 使用 JWT，携带 sessionId 和 tokenId。
  - 方案 B：新增 refresh token 历史表或 token family 表，重放时吊销整条 token family。
  - 方案 C：使用 Redis 分布式锁或 Go mutex 串行化同一 session refresh。
  - 方案 D：logout 立即吊销当前 `auth_sessions`，并新增 access token denylist。
- 为什么不选备选方案：
  - 不选 A：长期凭证会携带可解析元数据，偏离 Java 当前 opaque refresh token 设计，也扩大泄漏后的可观察信息。
  - 不选 B：安全性更强，但需要新增模型、迁移、清理策略和审计语义；本次明确当前不自动吊销已轮换的新会话。
  - 不选 C：锁不是跨数据库权威状态；Go mutex 无法覆盖多实例，Redis 锁增加外部依赖且仍需要 DB 条件更新兜底。
  - 不选 D：logout 吊销与 access token denylist 会引入新的安全边界、缓存一致性和多端设备语义；Java 当前 logout no-op，本次优先对齐。
- 后续可演进点：
  - 增加 refresh token family / reuse audit 表，重放时吊销当前会话链。
  - 增加用户禁用时批量吊销 ACTIVE sessions。
  - 增加 logout session revoke 与 access token denylist，并设计 Redis / DB 权威边界。
  - 增加过期 auth session 清理任务。
