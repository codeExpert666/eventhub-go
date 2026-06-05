# ADR：管理员用户接口 RBAC 边界

## 标题
管理员用户接口当前只要求 ADMIN 角色

## 状态
- accepted

## 背景
Go 版需要迁移 Java 版阶段 1 的 USER / ADMIN RBAC。Java 版通过 Spring Security URL 规则和 `@PreAuthorize("hasRole('ADMIN')")` 保护 `/api/v1/admin/users` 与 `/api/v1/admin/users/{userId}/status`。Go 版已经采用 JWT 最小 claim 和请求期回库加载当前用户状态、角色的策略，`security.Principal.Authorities` 中保存 `ROLE_` 前缀权限。

当前业务仍处在账号体系和管理端边界阶段，活动、场次、票种、订单、操作日志等资源模型尚未在 Go 版落地。此时需要决定管理员接口是否只做角色级 RBAC，还是提前引入 permissions、role_permissions 或资源级数据权限。

## 决策
Go 版选择：

- `GET /api/v1/admin/users` 只允许拥有 `ADMIN` 角色的已认证用户访问。
- `PATCH /api/v1/admin/users/{userId}/status` 只允许拥有 `ADMIN` 角色的已认证用户访问。
- 新增 `RequireRole("ADMIN")` middleware，根据 `security.Principal.Authorities` 判断 `ROLE_ADMIN`。
- JWT 不保存 roles、permissions、email、username 或 user status。
- 用户状态和角色仍由认证 middleware 每次根据 JWT `sub` 回库加载。
- 本次不引入 permissions、role_permissions、菜单权限、按钮权限或资源级数据权限。
- 未来如活动、订单等资源模型稳定，再独立设计资源级授权和数据范围。

## 备选方案
- 方案 1：只做 USER / ADMIN 角色级 RBAC。
- 方案 2：新增 permissions 与 role_permissions，管理员接口按权限点判断。
- 方案 3：新增资源级 ACL 或数据范围权限。
- 方案 4：把 roles 写入 JWT，RBAC middleware 只读 token。

## 决策理由
选择方案 1，原因是：

- 与 Java 版阶段 1 的业务语义对齐，当前管理端用户接口只需要验证普通用户和管理员边界。
- USER / ADMIN 足够支撑后续活动、票种、订单管理接口的第一层后台门禁。
- `roles` / `user_roles` 多对多表已经为后续扩展更多角色或权限关系预留基础。
- 当前资源模型尚未稳定，提前做 permissions 或资源级权限会引入空转设计。
- JWT 不保存 roles 可以保证用户禁用或角色变更后，不需要等待 access token 过期才生效。
- Go 版 `RequireRole` middleware 可复用到未来 `/api/v1/admin/**` 管理接口，不需要每个 handler 手写角色判断。

## 影响
- 好处
  - 权限边界清楚，USER 访问管理员接口稳定返回 `AUTH-403`。
  - 管理端路由能复用同一个 middleware，避免 handler 散落授权判断。
  - 保持 JWT claim 最小化，延续 ADR-0011 和 ADR-0013 的安全边界。
  - 与 Java-Go parity 中阶段 1 auth/RBAC 目标一致。
- 代价
  - 当前不能表达“某管理员只能管理某类活动”或“只能查看不可修改”等细粒度权限。
  - 管理员状态更新暂不写审计日志，后续需要补操作记录。
  - 所有受保护请求仍会回库加载用户状态和角色，高并发下需要后续评估缓存。
- 后续可能需要调整的地方
  - 活动、订单和票务资源稳定后，可引入 permissions、role_permissions 或资源级策略。
  - 增加管理员操作日志，记录操作者、目标用户、动作、旧状态、新状态、requestId 和时间。
  - 如引入权限缓存，必须设计用户禁用、角色变更和权限变更后的失效机制。
