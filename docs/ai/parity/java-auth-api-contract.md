# Java Auth API Contract

本文记录 Go 版迁移 Java Stage 1 auth / RBAC 能力时需要保持的接口契约、错误语义、JWT 边界、认证会话和测试对齐点。它是 `docs/ai/parity/java-go-parity-matrix.md` 的 auth 细化索引。

Java 版来源：

- `backend/src/main/java/com/eventhub/modules/auth/controller/AuthController.java`
- `backend/src/main/java/com/eventhub/modules/auth/controller/UserController.java`
- `backend/src/main/java/com/eventhub/modules/auth/controller/AdminUserController.java`
- `backend/src/main/java/com/eventhub/modules/auth/dto/request/*`
- `backend/src/main/java/com/eventhub/modules/auth/vo/*`
- `backend/src/main/java/com/eventhub/modules/auth/service/impl/*`
- `backend/src/main/java/com/eventhub/modules/auth/service/RefreshTokenHasher.java`
- `backend/src/main/java/com/eventhub/modules/auth/service/RefreshTokenParser.java`
- `backend/src/main/java/com/eventhub/infra/security/config/SecurityConfig.java`
- `backend/src/main/java/com/eventhub/infra/security/jwt/*`
- `backend/src/main/java/com/eventhub/common/api/ErrorCode.java`
- `backend/src/main/java/com/eventhub/modules/auth/exception/AuthException.java`
- `backend/src/main/resources/db/migration/V2__stage_1_auth_jwt_rbac.sql`
- `backend/src/main/resources/db/migration/V3__create_auth_sessions.sql`
- `backend/src/main/resources/mapper/auth/*.xml`
- `backend/src/test/java/com/eventhub/modules/auth/AuthIntegrationTest.java`
- `backend/src/test/java/com/eventhub/modules/auth/service/AuthSessionConcurrencyTest.java`
- `backend/src/test/java/com/eventhub/infra/security/jwt/JwtCodecTest.java`

Go 版目标：

- `api/openapi/eventhub.yaml`
- `internal/http/handler/{auth,user}`
- `internal/http/dto/{auth,user}`
- `internal/service/{auth,user}`
- `internal/http/middleware/{auth,rbac}.go`
- `internal/security/{jwt,password,principal,refresh}`
- `internal/repository`、`internal/repository/mysql`
- `migrations/000002_auth_schema.*.sql`
- `internal/http/auth_integration_test.go`
- `internal/service/{auth,user}/*_test.go`
- `internal/security/{jwt,refresh}/*_test.go`
- `internal/repository/mysql/mysql_repository_integration_test.go`

最近核验日期：2026-06-06。

## 1. 统一响应

所有业务接口使用统一响应 envelope：

```json
{
  "code": "COMMON-000",
  "message": "成功",
  "data": {},
  "requestId": "请求追踪 ID",
  "timestamp": "响应生成时间"
}
```

错误响应同样使用该 envelope；`data` 可以为 `null` 或字段级错误对象。

## 2. 认证与用户接口

### 注册

```text
POST /api/v1/auth/register
```

请求字段：

| 字段 | 类型 | 对齐语义 |
| --- | --- | --- |
| `username` | string | trim 后非空；3-32；`^[A-Za-z0-9_]+$` |
| `email` | string | trim 后非空；email；max 128；注册落库前 lower-case |
| `password` | string | 非空；8-72；至少包含字母和数字；只落 BCrypt hash |

成功 `data`：

```json
{
  "id": 1,
  "username": "alice",
  "email": "alice@example.com",
  "status": "ENABLED",
  "roles": ["USER"]
}
```

错误契约：

| 场景 | HTTP | code | message |
| --- | --- | --- | --- |
| 用户名重复 | 409 | `AUTH-409` | `用户名已存在` |
| 邮箱重复 | 409 | `AUTH-409` | `邮箱已存在` |
| 唯一键兜底无法区分字段 | 409 | `AUTH-409` | `用户名或邮箱已存在` |
| DTO 校验失败 | 400 | `COMMON-400` | `请求体参数校验失败` |
| JSON 请求体格式错误 | 400 | `COMMON-400` | `请求体格式不合法` |

业务语义：

- 创建 `ENABLED` 用户。
- 默认绑定 `USER` 角色。
- 用户创建和角色绑定在同一事务内。
- 数据库 unique constraint 是并发注册的最终防线。
- 响应不得暴露 `passwordHash`。

### 登录

```text
POST /api/v1/auth/login
```

请求字段：

| 字段 | 类型 | 对齐语义 |
| --- | --- | --- |
| `usernameOrEmail` | string | trim 后非空；max 128；包含 `@` 时 lower-case 查询 |
| `password` | string | 非空；max 72；使用 BCrypt matches 校验 |

成功 `data`：

```json
{
  "accessToken": "jwt-token",
  "refreshToken": "opaque-refresh-token",
  "authorizationScheme": "Bearer",
  "expiresIn": 1800,
  "refreshExpiresIn": 2592000,
  "sessionId": "server-session-id",
  "user": {
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "status": "ENABLED",
    "roles": ["USER"]
  }
}
```

错误契约：

| 场景 | HTTP | code | message |
| --- | --- | --- | --- |
| 账号不存在 | 401 | `AUTH-401` | `账号或密码错误` |
| 密码错误 | 401 | `AUTH-401` | `账号或密码错误` |
| 用户禁用后登录 | 403 | `AUTH-403` | `用户已被禁用` |
| DTO 校验失败 | 400 | `COMMON-400` | `请求体参数校验失败` |

业务语义：

- 登录失败不创建 `auth_sessions`。
- 登录成功每次创建新的 `ACTIVE` auth session，支持多设备 / 多次登录。
- `refreshToken` 明文只返回一次，数据库只保存 `sha256:<hex>`。

### Refresh

```text
POST /api/v1/auth/refresh
```

请求字段：

| 字段 | 类型 | 对齐语义 |
| --- | --- | --- |
| `refreshToken` | string | HTTP 层校验非空和 max 128；service 层校验 opaque token 固定 43 长度、URL-safe Base64 无 padding |

成功 `data` 与登录 token pair 字段一致。

错误契约：

| 场景 | HTTP | code | message |
| --- | --- | --- | --- |
| DTO 校验失败 | 400 | `COMMON-400` | `请求体参数校验失败` |
| token 格式非法、hash 查不到、重放、过期、session revoked、用户不存在或禁用 | 401 | `AUTH-401` | `refresh token 无效或已过期` |

业务语义：

- refresh endpoint 匿名放行，因为 access token 可能已经过期。
- refresh token 是 opaque token，不是 JWT。
- refresh 身份来源只来自 `auth_sessions.refresh_token_hash` 匹配到的服务端 session。
- 成功后旧 token 立即失效，新 token hash 替换旧 hash。
- 条件更新必须匹配 sessionId、old hash、old version、`ACTIVE` 状态和未过期时间，并将 `version + 1`。
- 同一旧 refresh token 并发提交时最多一个成功。
- 当前不保存历史 hash，也不自动吊销已轮换的新会话；旧 token 重放统一返回 `AUTH-401`。

### Logout

```text
POST /api/v1/auth/logout
Authorization: Bearer <access_token>
```

错误契约：

| 场景 | HTTP | code | message |
| --- | --- | --- | --- |
| 缺失 Bearer token | 401 | `AUTH-401` | `请先登录或重新登录` |
| access token 无效或过期 | 401 | `AUTH-401` | `请先登录或重新登录` |

业务语义：

- 必须已认证。
- 当前 no-op，不修改 `auth_sessions`，不写 access token denylist。
- 客户端收到成功后删除本地 token。

### 当前用户

```text
GET /api/v1/me
Authorization: Bearer <access_token>
```

成功 `data` 与注册用户摘要一致。

错误契约：

| 场景 | HTTP | code | message |
| --- | --- | --- | --- |
| 缺失、过期、签名非法或 claim 不完整的 access token | 401 | `AUTH-401` | `请先登录或重新登录` |
| 用户不存在或已禁用但持有旧 token | 401 | `AUTH-401` | `请先登录或重新登录` |

业务语义：

- middleware 只从 Bearer access token 解析最小 claim。
- 每次受保护请求必须按 JWT `sub` 回 DB 加载最新用户状态和角色。
- 响应不得暴露 `passwordHash`。

## 3. 管理员用户接口

两个接口都要求 Bearer token 且当前用户拥有 `ADMIN` 角色。Java 使用 URL 规则 + `@PreAuthorize("hasRole('ADMIN')")`，Go 使用受保护 route group + `RequireRole("ADMIN")` middleware；业务语义一致。

### 分页查询用户

```text
GET /api/v1/admin/users
```

查询参数：

| 字段 | 类型 | 默认 / 约束 | 对齐语义 |
| --- | --- | --- | --- |
| `page` | int | 默认 1；min 1 | 1-based page |
| `size` | int | 默认 20；min 1；max 100 | 每页条数 |
| `username` | string | max 32 | trim 后为空忽略，包含匹配 |
| `email` | string | max 128 | trim + lower-case 后为空忽略，包含匹配 |
| `status` | string | 空 / `ENABLED` / `DISABLED` | 非法状态返回字段级校验错误 |
| `createdAtFrom` / `createdAtTo` | string | `yyyy-MM-dd'T'HH:mm:ss` | from 晚于 to 返回 400 |
| `updatedAtFrom` / `updatedAtTo` | string | `yyyy-MM-dd'T'HH:mm:ss` | from 晚于 to 返回 400 |

成功 `data` 为 `PageResponse<UserInfo>`：

```json
{
  "items": [],
  "page": 1,
  "size": 20,
  "total": 0,
  "totalPages": 0,
  "hasNext": false,
  "hasPrevious": false
}
```

业务语义：

- 排序为 `created_at DESC, id DESC`。
- 当前页用户角色批量加载，避免 N+1。
- Go 在进入 COUNT / SQL 前额外校验超大 page 对应 offset 能安全放入 sqlc `int32` 参数；超出返回 `COMMON-400`。这是 Go 生成代码参数边界保护，不改变常规 Java 分页契约。

### 更新用户状态

```text
PATCH /api/v1/admin/users/{userId}/status
```

请求字段：

| 字段 | 类型 | 对齐语义 |
| --- | --- | --- |
| `status` | string | 必填；只能是 `ENABLED` 或 `DISABLED`；不接受数字 ordinal |

错误契约：

| 场景 | HTTP | code | message |
| --- | --- | --- | --- |
| 未认证或 access token 无效 | 401 | `AUTH-401` | `请先登录或重新登录` |
| 非 ADMIN 用户访问 | 403 | `AUTH-403` | `权限不足` |
| query/body/path 参数校验失败 | 400 | `COMMON-400` | `请求参数校验失败` 或 `请求体参数校验失败` |
| 目标用户不存在 | 404 | `COMMON-404` | `用户不存在` |

业务语义：

- 状态更新在事务内执行，并回读更新后的用户摘要。
- 禁用用户后，该用户未过期旧 access token 访问受保护接口必须返回 `AUTH-401`。

## 4. JWT access token

access token 必须包含：

| Claim | 来源 | 要求 |
| --- | --- | --- |
| `iss` | auth token 配置 | 签发和解析时校验 issuer |
| `sub` | `users.id` | 必须可解析为整数用户 ID |
| `iat` | 签发时间 | 标准 JWT claim |
| `exp` | 过期时间 | 标准 JWT claim |
| `jti` | 随机 UUID | 必填 |
| `sid` | `auth_sessions.session_id` | 必填 |
| `typ` | 固定字符串 | 必须为 `access` |

JWT 禁止保存：

- 角色
- 权限
- 邮箱
- 用户名
- 用户状态
- 密码哈希或任何敏感凭证

## 5. Refresh Token 与 auth_sessions

refresh token：

- opaque token，不是 JWT。
- 由 32 字节随机数生成，经 URL-safe Base64 无 padding 编码，长度固定 43。
- 明文只返回给客户端一次。
- DB 保存格式为 `sha256:<hex>`，SHA-256 hex 小写。
- 默认 TTL 为 30 天，即 `2592000` 秒。

登录成功写入 `auth_sessions`：

| 字段 | 语义 |
| --- | --- |
| `session_id` | 服务端会话标识，返回给客户端并写入 JWT `sid` |
| `user_id` | 登录用户 ID |
| `refresh_token_hash` | `sha256:<hex>` |
| `status` | `ACTIVE` |
| `issued_at` | 登录签发时间 |
| `refresh_expires_at` | refresh token 过期时间 |
| `last_seen_at` | 可用 `issued_at` 初始化 |
| `version` | 初始为 0，轮换成功时递增 |

Go migration 已对齐 Java V3 的唯一约束、索引和状态值；Go 将 Java V2/V3 合并为从空库起步的 `000002_auth_schema`。

## 6. OpenAPI 信息

Java 通过 Springdoc 从 Controller 注解和 DTO schema 生成 OpenAPI：

- `AuthController` tag: `Auth`
- `UserController` tag: `User`
- `AdminUserController` tag: `Admin User`
- 标题：`EventHub Backend API`
- 版本：`v1`
- dev/test 默认开启 `/v3/api-docs` 和 Swagger UI。
- prod profile 关闭 OpenAPI JSON 和 Swagger UI，安全白名单跟随开关。

Go 当前已建立 spec-first OpenAPI：

- 契约源：`api/openapi/eventhub.yaml`
- 生成代码：`api/openapi/gen/{models.gen.go,server.gen.go}`
- 文档入口：`GET /openapi.yaml`、`GET /swagger/*`
- dev/test 默认 `OPENAPI_ENABLED=true`，prod 默认 `OPENAPI_ENABLED=false`
- 禁用时不注册文档入口，统一返回 `COMMON-404`
- `make openapi-validate`、`make openapi-generate`、`make openapi-check` 负责验证和漂移检查

Go 不复刻 Springdoc 注解扫描、`/v3/api-docs` 和 `/swagger-ui.html` 路径；安全目标与接口契约一致。

## 7. 测试对齐点

Java `AuthIntegrationTest` / `JwtCodecTest` / auth session concurrency tests 覆盖的核心断言，Go 当前已通过 HTTP/service/security/repository 测试分层承接：

- 注册成功返回 `COMMON-000`、`ENABLED`、`USER`，不暴露 `passwordHash`。
- 重复用户名、唯一键兜底冲突返回 `AUTH-409`；重复邮箱契约保留为 `AUTH-409` / `邮箱已存在`。
- 登录成功返回 access token、refresh token、`authorizationScheme=Bearer`、`expiresIn`、`refreshExpiresIn=2592000`、`sessionId` 和用户摘要。
- 错误密码、账号不存在返回 `AUTH-401` / `账号或密码错误`，且不创建 session。
- 用户禁用后不能登录，返回 `AUTH-403` / `用户已被禁用`。
- refresh 成功轮换 token pair，旧 token 不能再次使用，新 token 可以继续 refresh。
- 过期、revoked、禁用用户、非法格式或重放 refresh token 返回 `AUTH-401` / `refresh token 无效或已过期`。
- 同一旧 refresh token 并发提交时最多一个成功。
- logout 未认证返回 `AUTH-401`；已认证成功但当前不修改 DB。
- `/api/v1/me` 对缺失、过期、篡改、claim 不完整、禁用用户旧 token 返回 `AUTH-401`；合法 token 返回当前用户摘要和角色。
- 管理员用户列表支持 ADMIN 访问、USER 返回 `AUTH-403`、分页元数据、username/email/status/time-range 筛选、`created_at DESC, id DESC` 排序和角色批量加载。
- 管理员更新用户状态支持状态切换、目标不存在返回 `COMMON-404`，禁用后旧 access token 立即失效。
- JWT `typ != access`、缺失 `jti`、缺失 `sid`、过期和篡改 token 均被拒绝。
- refresh token 生成长度、格式、hash 前缀和稳定 hash 已覆盖。
