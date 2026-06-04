# Java Auth API Contract

本文记录 Go 版迁移 `register/login/me/access token` 时需要对齐的 Java 版认证接口契约。它是 `docs/ai/parity/java-go-parity-matrix.md` 的 auth 细化索引，不替代设计文档和 ADR。

Java 版来源：

- `backend/src/main/java/com/eventhub/modules/auth/controller/AuthController.java`
- `backend/src/main/java/com/eventhub/modules/auth/controller/UserController.java`
- `backend/src/main/java/com/eventhub/modules/auth/dto/request/RegisterRequest.java`
- `backend/src/main/java/com/eventhub/modules/auth/dto/request/LoginRequest.java`
- `backend/src/main/java/com/eventhub/modules/auth/vo/LoginResponse.java`
- `backend/src/main/java/com/eventhub/modules/auth/vo/TokenPairResponse.java`
- `backend/src/main/java/com/eventhub/modules/auth/vo/UserInfo.java`
- `backend/src/main/java/com/eventhub/modules/auth/service/impl/AuthServiceImpl.java`
- `backend/src/main/java/com/eventhub/modules/auth/service/impl/TokenServiceImpl.java`
- `backend/src/main/java/com/eventhub/modules/auth/service/impl/AuthSessionServiceImpl.java`
- `backend/src/main/java/com/eventhub/infra/security/jwt/JwtClaims.java`
- `backend/src/main/java/com/eventhub/infra/security/jwt/JwtCodec.java`
- `backend/src/main/java/com/eventhub/infra/security/jwt/JwtAuthenticationFilter.java`
- `backend/src/main/java/com/eventhub/modules/auth/security/AuthenticatedPrincipalService.java`
- `backend/src/main/java/com/eventhub/infra/security/principal/AuthenticatedPrincipal.java`
- `backend/src/main/java/com/eventhub/infra/security/config/SecurityConfig.java`
- `backend/src/main/java/com/eventhub/common/api/ErrorCode.java`
- `backend/src/main/java/com/eventhub/modules/auth/exception/AuthException.java`
- `backend/src/test/java/com/eventhub/modules/auth/AuthIntegrationTest.java`
- `backend/src/test/java/com/eventhub/infra/security/jwt/JwtCodecTest.java`
- `backend/src/main/resources/db/migration/V2__stage_1_auth_jwt_rbac.sql`
- `backend/src/main/resources/db/migration/V3__create_auth_sessions.sql`

最近核验日期：2026-06-05。

## 1. 统一响应

所有业务接口继续使用统一响应 envelope：

```json
{
  "code": "COMMON-000",
  "message": "成功",
  "data": {},
  "requestId": "请求追踪 ID",
  "timestamp": "响应生成时间"
}
```

错误响应同样使用该 envelope，`data` 可以为 `null` 或字段级错误对象。

## 2. 注册接口

接口：

```text
POST /api/v1/auth/register
```

请求字段：

| 字段 | 类型 | Java 校验 | Go 对齐要求 |
| --- | --- | --- | --- |
| `username` | string | not blank；3-32；`^[A-Za-z0-9_]+$` | 保持字段名和校验语义 |
| `email` | string | not blank；email；max 128 | 注册前 trim + lower-case |
| `password` | string | not blank；8-72；至少字母和数字 | 只在入口短暂存在，落库 BCrypt hash |

成功响应 `data` 为用户摘要：

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
| 并发唯一键兜底无法区分字段 | 409 | `AUTH-409` | `用户名或邮箱已存在` |
| DTO 校验失败 | 400 | `COMMON-400` | `请求体参数校验失败` |
| JSON 请求体格式错误 | 400 | `COMMON-400` | `请求体格式不合法` |

业务语义：

- 注册创建 `ENABLED` 用户。
- 注册成功默认绑定 `USER` 角色。
- 用户创建和角色绑定在同一事务内。
- 并发注册同一账号时，数据库 unique constraint 是最终防线；只能一个成功，其他请求映射为 `AUTH-409`。
- 响应不得暴露 `passwordHash`。

## 3. 登录接口

接口：

```text
POST /api/v1/auth/login
```

请求字段：

| 字段 | 类型 | Java 校验 | Go 对齐要求 |
| --- | --- | --- | --- |
| `usernameOrEmail` | string | not blank；max 128 | trim；包含 `@` 时 lower-case |
| `password` | string | not blank；max 72 | 使用 BCrypt matches 校验 |

成功响应 `data`：

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
- 登录成功每次创建一条新的 `ACTIVE` auth session，支持多设备/多次登录。
- 登录响应返回 access token、refresh token、authorization scheme、两个过期秒数、session id 和用户摘要。
- `refreshToken` 明文只在响应中返回一次。
- DB 只保存 refresh token hash，不保存明文。

## 4. 当前用户接口

接口：

```text
GET /api/v1/me
Authorization: Bearer <access_token>
```

成功响应 `data` 与注册接口的用户摘要一致。

错误契约：

| 场景 | HTTP | code | message |
| --- | --- | --- | --- |
| 缺失 Bearer token | 401 | `AUTH-401` | `请先登录或重新登录` |
| access token 过期 | 401 | `AUTH-401` | `请先登录或重新登录` |
| access token 签名非法/篡改 | 401 | `AUTH-401` | `请先登录或重新登录` |
| JWT claim 缺失或 `typ != access` | 401 | `AUTH-401` | `请先登录或重新登录` |
| 用户不存在 | 401 | `AUTH-401` | `请先登录或重新登录` |
| 用户已禁用且持有旧 token | 401 | `AUTH-401` | `请先登录或重新登录` |

业务语义：

- 认证 middleware 只从 Bearer token 中解析最小 claim。
- 每次受保护请求必须根据 JWT `sub` 回 DB 加载最新用户状态和角色。
- 禁用用户持有未过期旧 token 也必须被拒绝。
- 当前用户响应不得暴露 `passwordHash`。

## 5. Refresh 与 Logout 接口

Refresh 接口：

```text
POST /api/v1/auth/refresh
```

请求字段：

| 字段 | 类型 | Java 校验 | Go 对齐要求 |
| --- | --- | --- | --- |
| `refreshToken` | string | not blank；max 128 | HTTP 层校验非空和最大长度；service 层校验 opaque token 固定 43 长度和 URL-safe Base64 无 padding 格式 |

成功响应 `data` 与登录 token pair 字段保持一致：

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

Refresh 错误契约：

| 场景 | HTTP | code | message |
| --- | --- | --- | --- |
| DTO 校验失败 | 400 | `COMMON-400` | `请求体参数校验失败` |
| refresh token 格式非法 | 401 | `AUTH-401` | `refresh token 无效或已过期` |
| refresh token hash 查不到 | 401 | `AUTH-401` | `refresh token 无效或已过期` |
| 旧 refresh token 重放 | 401 | `AUTH-401` | `refresh token 无效或已过期` |
| session 已过期 | 401 | `AUTH-401` | `refresh token 无效或已过期` |
| session 已 `REVOKED` | 401 | `AUTH-401` | `refresh token 无效或已过期` |
| 用户不存在或已禁用 | 401 | `AUTH-401` | `refresh token 无效或已过期` |

Refresh 业务语义：

- refresh endpoint 匿名放行，因为 access token 可能已经过期。
- refresh token 是 opaque token，不是 JWT。
- refresh 身份来源只来自 `auth_sessions.refresh_token_hash` 匹配到的服务端 session。
- DB 只保存 `sha256:<hex>`。
- refresh 成功后旧 token 立即失效，新 token hash 替换旧 hash。
- 条件更新必须同时匹配 sessionId、old hash、old version、`ACTIVE` 状态和未过期时间。
- 轮换成功时 `version + 1`。
- 同一旧 refresh token 并发提交时最多一个成功。
- 当前不自动吊销已轮换的新会话。

Logout 接口：

```text
POST /api/v1/auth/logout
Authorization: Bearer <access_token>
```

Logout 错误契约：

| 场景 | HTTP | code | message |
| --- | --- | --- | --- |
| 缺失 Bearer token | 401 | `AUTH-401` | `请先登录或重新登录` |
| access token 无效或过期 | 401 | `AUTH-401` | `请先登录或重新登录` |

Logout 业务语义：

- 必须已认证。
- 当前 no-op，不修改 `auth_sessions`。
- 当前不写 access token denylist。
- 客户端收到成功后删除本地 token。

## 6. JWT access token

Java `JwtClaims` / `JwtCodec` 当前要求 access token 包含：

| Claim | 来源 | 要求 |
| --- | --- | --- |
| `iss` | auth token 配置 | 生成和解析时校验 issuer |
| `sub` | `users.id` | 必须是可解析为整数的用户 ID |
| `iat` | 签发时间 | 标准 JWT claim |
| `exp` | 过期时间 | 标准 JWT claim |
| `jti` | 随机 UUID | 必填，唯一标识 access token |
| `sid` | `auth_sessions.session_id` | 必填，关联服务端认证会话 |
| `typ` | 固定字符串 | 必须等于 `access` |

JWT 禁止保存：

- 角色
- 权限
- 邮箱
- 用户名
- 用户状态
- 密码哈希或任何敏感凭证

## 7. refresh token

Go 版已实现登录返回 refresh token 和 refresh endpoint 轮换。

Java 当前语义：

- refresh token 是 opaque token，不是 JWT。
- 由 32 字节随机数生成，经 Base64 URL-safe 无 padding 编码，长度固定 43。
- 明文只返回给客户端一次。
- DB 保存格式为 `sha256:<hex>`。
- 哈希算法为 SHA-256，hex 小写，前缀为后续算法升级预留。
- 默认 refresh token TTL 为 30 天，即 `2592000` 秒。

## 8. auth_sessions

登录成功写入：

| 字段 | 语义 |
| --- | --- |
| `session_id` | 服务端会话标识，返回给客户端并写入 JWT `sid` |
| `user_id` | 登录用户 ID |
| `refresh_token_hash` | `sha256:<hex>` |
| `status` | `ACTIVE` |
| `issued_at` | 登录签发时间 |
| `refresh_expires_at` | refresh token 过期时间 |
| `last_seen_at` | 可用 `issued_at` 初始化 |
| `version` | 初始为 0 |

Go 版当前 migration 已对齐 Java V3 的唯一约束、索引和状态值。

## 9. OpenAPI 信息

Java 版通过 Springdoc 从 Controller 注解和 DTO schema 自动生成 OpenAPI：

- `AuthController` tag: `Auth`
- `UserController` tag: `User`
- 全局文档标题：`EventHub Backend API`
- 文档版本：`v1`
- 开发/测试默认开启 `/v3/api-docs` 和 Swagger UI。
- 生产 profile 关闭 OpenAPI JSON 和 Swagger UI，并且安全白名单跟随开关。

Go 版当前尚未建立 `api/openapi/eventhub.yaml`，本次实现只沉淀 Java contract 文档；OpenAPI YAML 和 validate 流程后续单独迁移。

## 10. 测试对齐点

Java `AuthIntegrationTest` 中与本次 Go 任务直接相关的断言：

- 注册成功返回 `COMMON-000`，用户状态 `ENABLED`，角色 `USER`，不返回 `passwordHash`。
- 重复用户名返回 409 / `AUTH-409` / `用户名已存在`。
- 重复邮箱返回 409 / `AUTH-409` / `邮箱已存在`。
- 并发注册同一账号时结果为一个 200 和一个 409。
- 登录成功返回 access token、refresh token、`authorizationScheme=Bearer`、`expiresIn>0`、`refreshExpiresIn=2592000`、`sessionId` 和用户摘要。
- 登录成功后创建 `ACTIVE` auth session；DB 中 refresh token hash 不等于明文，按 hash 可查询。
- refresh 成功返回新的 access token、refresh token、`authorizationScheme=Bearer`、`expiresIn`、`refreshExpiresIn`、`sessionId` 和用户摘要。
- refresh 成功后旧 refresh token 不能再次使用。
- 新 refresh token 可以继续 refresh。
- 过期 refresh token、revoked session、禁用用户、篡改 token 均返回 401 / `AUTH-401` / `refresh token 无效或已过期`。
- 同一旧 refresh token 并发提交时最多一个成功。
- logout 未认证返回 401 / `AUTH-401` / `请先登录或重新登录`。
- logout 已认证返回成功，当前不修改 DB。
- 密码错误返回 401 / `AUTH-401` / `账号或密码错误`，且不创建 session。
- 账号不存在返回 401 / `AUTH-401` / `账号或密码错误`，且不创建 session。
- 用户禁用后不能登录，返回 403 / `AUTH-403` / `用户已被禁用`。
- 未带 token 访问 `/api/v1/me` 返回 401 / `AUTH-401`。
- 过期 token、篡改 token 访问 `/api/v1/me` 返回 401 / `AUTH-401`。
- 用户被禁用后，旧 access token 访问 `/api/v1/me` 返回 401 / `AUTH-401`。
- 合法 token 访问 `/api/v1/me` 返回当前用户摘要和 `USER` 角色。
- `JwtCodecTest` 要求 `typ != access`、缺失 `jti`、缺失 `sid` 都被拒绝。

Go 版本次测试至少覆盖：

- 注册
- 重复注册
- 登录
- 错误密码
- `/api/v1/me`
- 禁用用户旧 token
- JWT claim 边界
- refresh token hash 格式
- refresh token 轮换与重放检测
- refresh 并发最多一个成功
- logout no-op
