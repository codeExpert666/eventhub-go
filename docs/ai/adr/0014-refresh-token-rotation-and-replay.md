# ADR：refresh token 轮换与重放检测

## 标题
refresh token 轮换依赖 DB 条件更新并暂不自动吊销已轮换的新会话

## 状态
- accepted

## 背景
Go 版需要迁移 Java 版 `POST /api/v1/auth/refresh`。Java 当前语义是：refresh token 是 opaque random token，服务端只保存 `sha256:<hex>`；refresh 成功后替换 `auth_sessions.refresh_token_hash`，并用旧 hash、旧 version、ACTIVE 状态和未过期时间做条件更新，保证同一旧 token 并发提交时最多一个成功。

关键决策点是：Go 版如何保证并发安全，以及检测到旧 refresh token 重放时是否要自动吊销已经轮换出的新会话。

## 决策
Go 版选择：

- refresh token 继续使用 opaque random token，不使用 JWT。
- DB 只保存 `sha256:<hex>`，不保存明文。
- refresh 成功后立即替换 `auth_sessions.refresh_token_hash`。
- 轮换 SQL 使用 DB 条件更新：

```sql
WHERE session_id = oldSessionId
  AND refresh_token_hash = oldHash
  AND version = oldVersion
  AND status = 'ACTIVE'
  AND refresh_expires_at > now
```

- 轮换成功时 `version = version + 1`，并更新 `refresh_expires_at`、`last_refreshed_at`、`last_seen_at`。
- 同一旧 refresh token 并发提交时，最多一条 UPDATE 影响 1 行。
- 旧 refresh token 重放返回 `AUTH-401`。
- 当前不自动吊销已轮换的新会话。
- 不使用 Go mutex、进程内锁或 Redis 锁保证 refresh 并发正确性。

## 备选方案
- 方案 1：只按 refresh token hash 查询后直接更新，不带 version/status/过期条件。
- 方案 2：使用 DB 条件更新，重放时只返回 `AUTH-401`。
- 方案 3：新增 token family / refresh token 历史表，旧 token 重放时吊销整条会话链。
- 方案 4：使用 Go mutex 或 Redis 分布式锁串行化同一 session refresh。
- 方案 5：refresh token 改为 JWT，携带 sessionId 和 tokenId。

## 决策理由
选择方案 2，原因是：

- 与 Java 当前实现和设计文档保持一致。
- DB 条件更新能覆盖多实例部署，比 Go mutex 更可靠。
- old hash + old version 让并发请求即使都读到旧 session 快照，也只有一个请求能更新成功。
- ACTIVE 和未过期条件让 session 状态、过期时间成为数据库最终防线。
- 不保存历史 hash 时，旧 token 重放无法可靠定位已经轮换后的新会话；贸然吊销可能引入误杀和不完整审计。
- token family/reuse audit 是更完整方案，但需要新增表、状态机、清理策略和审计语义，适合后续安全增强阶段。

## 影响
- 好处
  - 同一旧 refresh token 并发提交最多一个成功。
  - 旧 token 轮换后立即失效。
  - 不依赖进程内状态，适合未来多实例。
  - API 错误语义简单，所有 refresh 失败统一为 `AUTH-401`。
- 代价
  - 当前无法在旧 token 重放时自动吊销已轮换出的新会话。
  - 没有 refresh token reuse audit，服务端缺少安全事件留痕。
  - fake repository 测试只能模拟条件更新语义，真正跨进程正确性仍依赖 MySQL SQL。
- 后续可能需要调整的地方
  - 新增 token family / used token hash 表，重放时吊销会话链。
  - 增加 refresh reuse audit 日志和告警。
  - 增加过期 auth session 清理任务。
  - 如果引入 Redis 缓存，也必须保持 MySQL 作为 refresh token 权威记录。
