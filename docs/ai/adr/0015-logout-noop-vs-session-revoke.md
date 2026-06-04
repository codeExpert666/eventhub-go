# ADR：logout no-op 与 session revoke 边界

## 标题
当前 logout 只要求已认证并保持服务端 no-op

## 状态
- accepted

## 背景
Go 版需要迁移 Java 版 `POST /api/v1/auth/logout`。Java 当前实现中，logout 要求请求已认证，但 service 方法本身不修改数据库；它表达的是客户端删除本地 token 的协议语义。当前 access token 是无状态 JWT，服务端没有 access token denylist，也没有在受保护请求中校验 `auth_sessions.status` 来即时拒绝已吊销 session 的 access token。

关键决策点是：本次是否借实现 logout 之机，把当前 auth session 立即置为 `REVOKED`，或者引入 access token denylist。

## 决策
Go 版选择：

- `POST /api/v1/auth/logout` 挂在 auth middleware 保护组内，必须携带有效 Bearer access token。
- handler 从 context 读取 `security.Principal` 并传给 `AuthService.Logout`。
- service 校验 principal 有效后直接返回 nil。
- logout 当前不修改 `auth_sessions`。
- logout 当前不写 access token denylist。
- logout 当前不吊销 refresh token。
- 客户端收到成功响应后删除本地 access token 和 refresh token。

## 备选方案
- 方案 1：logout no-op，只要求已认证。
- 方案 2：logout 将当前 `auth_sessions.status` 更新为 `REVOKED`。
- 方案 3：logout revoke session，并在 auth middleware 每次校验 `sid` 对应 session 状态。
- 方案 4：logout 写入 access token `jti` denylist，middleware 每次查 denylist。
- 方案 5：logout 同时 revoke refresh session、写 access token denylist、记录审计日志。

## 决策理由
选择方案 1，原因是：

- 与 Java 当前 `AuthServiceImpl.logout` 和 controller 注释完全对齐。
- 当前 access token 无状态，单独 revoke auth session 并不能让已签发 access token 立即失效，容易给使用者造成“服务端已完全登出”的错觉。
- middleware 当前只按 JWT `sub` 回 DB 加载用户状态和角色，不校验 `sid` session status；改变这一点需要独立设计性能、缓存和兼容性。
- access token denylist 需要 Redis/DB 权威边界、过期清理和故障降级策略，不适合混入 refresh rotation 本次任务。
- no-op 仍保留清晰的协议入口，后续可以在同一 service 方法中扩展审计、session revoke 或 denylist。

## 影响
- 好处
  - 对齐 Java API 和当前安全语义。
  - 不引入半套吊销能力，避免状态边界误导。
  - handler/service 分层清晰，后续扩展点稳定。
- 代价
  - 客户端必须主动删除本地 token；服务端不会即时失效已签发 access token。
  - refresh token 在服务端仍保持原状态，除非客户端删除后不再提交。
  - 无 logout 审计记录。
- 后续可能需要调整的地方
  - 实现 session revoke 后，middleware 可评估按 `sid` 校验 session 状态。
  - 引入 access token denylist 时，需要设计 Redis/DB 权威边界和过期清理。
  - 增加设备会话列表后，可支持单设备 logout、全端 logout 和管理员踢下线。
