# ADR：管理员用户列表角色批量加载

## 标题
管理员用户列表按当前页用户 ID 批量加载角色

## 状态
- accepted

## 背景
管理员用户列表需要返回 `UserInfo.roles`。Java 版在 `AuthServiceImpl.listUsers` 中先分页查询当前页用户，再调用 `RoleMapper.findRoleCodesByUserIds` 一次性查询当前页所有用户的角色编码，并在 service 层按 userId 分组，避免遍历用户时逐个查询角色造成 N+1。

Go 版已经有 `RoleRepository.FindRoleCodesByUserID` 用于当前用户和登录响应，也已有 sqlc query `FindRoleCodesByUserIDs`。本次需要决定列表路径是否复用单用户角色查询，或显式提供批量角色加载语义。

## 决策
Go 版选择：

- 管理员列表先通过 `UserRepository.CountByCriteria` 和 `UserRepository.ListUsers` 查询分页用户。
- service 提取当前页用户 ID。
- 调用 `RoleRepository.FindRoleCodesByUserIDs(ctx, userIDs)` 一次性加载角色编码。
- service 将 `repository.UserRoleCode` 扁平行按 userId 分组。
- 用户没有角色记录时，响应中的 `roles` 返回空数组。
- 批量角色查询继续归属 `RoleRepository`，不塞入 `UserRepository` 的用户列表结果中。
- 不用 JOIN 直接拼出最终 HTTP DTO，避免 repository 泄漏 HTTP 响应形态。

## 备选方案
- 方案 1：列表遍历用户并逐个调用 `FindRoleCodesByUserID`。
- 方案 2：按当前页用户 ID 批量查询角色，在 service 层分组。
- 方案 3：用一个大 JOIN 查询用户和角色，repository 返回聚合后的用户摘要。
- 方案 4：缓存用户角色，列表接口从缓存读取。

## 决策理由
选择方案 2，原因是：

- 与 Java 版管理员用户分页查询实现和文档对齐。
- 当前页最多 100 个用户，如果逐个查角色，查询次数会从固定 3 次退化为 `2 + N` 次。
- 批量查询返回扁平结果，service 分组逻辑简单，且能清楚表达“列表组装角色”的业务意图。
- `RoleRepository` 继续表达 `roles/user_roles` 的持久化语义，避免用户仓储跨职责聚合角色关系。
- repository 不返回 HTTP DTO，也不暴露 sqlc generated model，保持 `handler -> service -> repository -> sqlc/database`。
- 当前没有稳定的角色缓存失效策略，直接缓存会让角色变更或用户禁用语义变复杂。

## 影响
- 好处
  - 消除管理员用户列表路径上的 N+1 查询风险。
  - 角色查询成本与当前页用户数量解耦为一次批量 SQL。
  - service 能统一处理无角色用户的空数组响应。
  - Go 版和 Java 版在列表查询策略上保持 parity。
- 代价
  - service 需要维护一段按 userId 分组的组装逻辑。
  - 列表总数、当前页用户和角色批量查询不是同一事务快照，数据变化时可能有轻微读偏差。
  - 当前没有针对 `user_roles.user_id` 的单独索引；唯一约束 `uk_user_roles_user_role` 以 user_id 开头，可支撑按用户 ID 查询，后续仍应结合查询计划复核。
- 后续可能需要调整的地方
  - 如果列表增加角色筛选，需要重新评估 SQL 结构和索引。
  - 如果角色数量和用户量显著增长，可以评估角色缓存，但必须先设计失效策略。
  - 如果用户列表成为高频管理接口，可评估 `(created_at, id)`、`(status, created_at, id)` 等用户表索引。
