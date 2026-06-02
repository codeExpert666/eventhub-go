# ADR：当前用户加载策略

## 标题
受保护请求按 JWT sub 回 DB 加载当前用户状态和角色

## 状态
- accepted

## 背景
Go 版认证 middleware 需要在处理 `GET /api/v1/me` 等受保护请求时建立当前用户上下文。可选策略包括完全信任 JWT 中的角色/状态快照、每次查 DB、或引入 Redis 短缓存。

Java 版 `JwtAuthenticationFilter` 会先解析 JWT，再通过 `AuthenticatedPrincipalService.loadByUserId` 查询最新用户和角色。禁用用户持有未过期旧 token 时，Java 集成测试要求访问 `/api/v1/me` 返回 `AUTH-401`。

## 决策
Go 版选择：

- JWT 只提供稳定用户 ID：`sub`。
- 每次受保护请求根据 `sub` 查询 `users`。
- 用户不存在或 `status != ENABLED` 时返回 `AUTH-401`。
- 每次受保护请求根据 user id 查询角色编码。
- HTTP 用户摘要返回不带 `ROLE_` 前缀的角色编码。
- `security.Principal` 中保存带 `ROLE_` 前缀的 authorities，用于后续 RBAC。
- `Principal` 通过 Go `context.Context` 在 middleware 和 handler/service 间传递。
- 本阶段不使用 Redis 缓存当前用户状态和角色。

## 备选方案
- 方案 1：JWT 内写入角色和状态，middleware 不查 DB。
- 方案 2：middleware 每次按 `sub` 查 DB 加载用户状态和角色。
- 方案 3：middleware 先查 Redis 短缓存，miss 后回 DB。
- 方案 4：middleware 同时校验 `auth_sessions.status`，被 revoke 的 session 立即拒绝 access token。

## 决策理由
选择方案 2，原因是：

- 与 Java 当前实现完全对齐。
- 禁用用户旧 token 能立即失效，满足本次任务的核心安全要求。
- 用户角色变更不会受 access token 过期时间影响。
- 当前项目还没有稳定的 Redis 缓存失效策略，直接引入缓存容易让禁用用户或角色变更出现短暂不一致。
- 方案 4 对后续 logout 和单设备踢下线有价值，但 Java 当前 access token 认证链路尚未检查 session status；本次保持最小对齐，后续单独设计 denylist 或 session 校验。

## 影响
- 好处
  - 当前用户状态和权限始终以 DB 为准。
  - 禁用用户持有旧 token 时会被拒绝。
  - Go context 中的 Principal 来源清晰，handler 不解析 JWT。
- 代价
  - 每个受保护请求增加一次用户查询和一次角色查询。
  - 高并发下需要后续评估缓存、连接池和索引。
  - 当前 session revoke 不会让 access token 立即失效。
- 后续可能需要调整的地方
  - 为用户状态和角色引入短 TTL 缓存时，必须先设计禁用、角色变更和管理员操作后的缓存失效。
  - 实现 logout 后可评估 `jti` denylist 或 `sid` session status 校验。
  - 管理端 RBAC 完成后，复用 `Principal.Authorities` 做 `ROLE_ADMIN` 判断。
