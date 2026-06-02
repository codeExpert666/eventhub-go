# ADR：JWT claim 边界

## 标题
JWT access token 只保存最小认证声明

## 状态
- accepted

## 背景
Go 版需要实现登录后签发 JWT access token，并对齐 Java 版 `JwtClaims`、`JwtCodec`、`JwtAuthenticationFilter` 和 `2026-05-24-access-token-claim-boundary.md` ADR。关键问题是 access token 是否应该携带角色、权限、邮箱、用户名、用户状态等动态信息。

如果 JWT 过度自包含，受保护请求可以少查一次数据库，但用户禁用、角色调整和账号状态变更会变成 token 过期前无法及时生效的快照。EventHub 后续还需要支持会话管理、refresh token 轮换、logout 和安全审计，因此 access token 也需要保留稳定的 token id 和 session id。

## 决策
Go 版 JWT access token 写入：

- `iss`
- `sub`
- `iat`
- `exp`
- `jti`
- `sid`
- `typ=access`

Go 版 JWT access token 不写入：

- 角色
- 权限
- 邮箱
- 用户名
- 用户状态
- 密码哈希
- refresh token 或任何长期凭证

认证 middleware 解析 JWT 后，必须根据 `sub` 回 DB 加载最新用户状态和角色，再构造 request context 中的 `Principal`。

## 备选方案
- 方案 1：只写 `sub/iat/exp/iss`，不写 `jti/sid/typ`。
- 方案 2：写 `iss/sub/iat/exp/jti/sid/typ=access`，动态用户信息每次回 DB 加载。
- 方案 3：把角色、权限、用户名、邮箱和状态一起写入 JWT。

## 决策理由
选择方案 2，原因是：

- `jti` 可以唯一标识某个 access token，便于后续审计、denylist 和排障。
- `sid` 可以把 access token 与 `auth_sessions.session_id` 关联，为 refresh、logout 和设备会话管理预留上下文。
- `typ=access` 能防止 refresh token 或其他 token 类型被误用于 access token 认证链路。
- 不写动态用户信息，可以保证禁用用户持有旧 token 也会在下一次受保护请求时被拒绝。
- 该方案与 Java 版 ADR 和 `JwtCodecTest` 保持一致，同时符合 Go 生态中显式加载业务状态的实现方式。

## 影响
- 好处
  - JWT 保持轻量，claim 边界清楚。
  - 用户禁用和角色变更能及时影响受保护请求。
  - 后续 denylist、审计和会话管理有稳定扩展点。
- 代价
  - 每次受保护请求需要回 DB 或未来受控缓存加载用户状态和角色。
  - 当前没有用户级 token version，不能仅靠 JWT claim 实现全端失效。
- 后续可能需要调整的地方
  - 如果引入 Redis 用户权限短缓存，需要设计禁用用户和角色变更的失效策略。
  - 如果新增 `users.token_version`，可评估把版本 claim 加入 access token。
  - 如果服务拆分到网关/资源服务，需要重新评估哪些 claim 可以安全下沉。
