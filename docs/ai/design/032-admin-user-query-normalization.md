# 管理员用户列表查询参数规范化

## 1. 背景
- 当前 Go strict handler 使用 `parseAdminUserListQuery` 将 generated `ListAdminUsersParams` 映射为 `usersvc.AdminUserListQuery`。
- 现有实现会在 `validateAdminUserListQuery` 中用 `strings.TrimSpace` 校验 `username`、`email`、`status`，但最终传给 service 的 `AdminUserListQuery` 仍保留原始空白字符。
- service 层当前会在构造 repository criteria 时再次 trim，因此数据库查询行为有防御性兜底；但 handler 输出的 query 不是已经规范化的边界对象，和 strict handler 映射职责不够一致。
- Java 版对应语义来自 `AdminUserQueryRequest` 与 `UserQueryCriteria`：HTTP 查询参数承接分页和筛选字段，后续查询条件使用已经 trim / normalize 的字段。参考 Java `docs/ai/design/2026-05-18-admin-user-pagination-design.md` 和 `docs/ai/implementation/2026-05-18-admin-user-pagination-implementation.md`。

## 2. 目标
- 合并管理员用户列表 query 的解析与校验逻辑，让 `parseAdminUserListQuery` 在一个流程中完成默认值、trim、长度校验、状态校验、时间解析和时间范围校验。
- `username`、`email`、`status` 使用 trim 后的值写入 `usersvc.AdminUserListQuery`。
- 保留 service 层已有 normalize 作为防御边界，不改变 service contract、repository criteria、SQL 或 OpenAPI 契约。
- 成功标准是新增 handler 级测试能证明 trim 后的 query 被交给 service 边界。

## 3. 非目标
- 不调整 `GET /api/v1/admin/users` 的路径、HTTP 方法、请求字段、响应字段、状态码或 OpenAPI generated model。
- 不修改 `usersvc.AdminUserListQuery` 类型结构。
- 不修改 repository 查询、sqlc query、migration 或数据库索引。
- 不引入新的 HTTP DTO、共享 validator 框架或 handler 抽象。

## 4. 影响范围
- 涉及 Go package：
  - `internal/http/handler/user`
- 涉及 API：
  - `GET /api/v1/admin/users`
- 不涉及表、缓存、外部接口、OpenAPI 生成配置或 service/repository package。
- Java-Go parity matrix 需要索引本次 handler 边界收敛；对外契约状态仍为已对齐。

## 5. 领域建模
- `openapigen.ListAdminUsersParams` 是 OpenAPI strict server 生成的 HTTP 查询参数对象。
- `usersvc.AdminUserListQuery` 是 handler 传入 user service 的读操作输入，不承担 HTTP JSON 契约。
- `username`、`email`、`status` 是管理员列表筛选条件，不改变用户领域模型和用户状态枚举。
- 与 Java 版关系：
  - Java `AdminUserQueryRequest` 对应 Go generated query params。
  - Java `UserQueryCriteria` 的规范化查询条件语义对应 Go service 最终构造的 repository criteria。
  - Go handler 先输出 trim 后的 service query，使 handler/service 边界更接近 Java 文档中的规范化意图。

## 6. API 设计
- 接口：`GET /api/v1/admin/users`
- 请求参数不变：
  - `page`：默认 `1`，最小 `1`。
  - `size`：默认 `20`，最小 `1`，最大 `100`。
  - `username`：可选，trim 后最长 `32`。
  - `email`：可选，trim 后最长 `128`。
  - `status`：可选，trim 后只允许 `ENABLED` 或 `DISABLED`。
  - `createdAtFrom` / `createdAtTo` / `updatedAtFrom` / `updatedAtTo`：可选，本地时间格式 `2006-01-02T15:04:05`。
- 响应结构不变，仍返回 generated `ApiResponseAdminUserPage`。
- 错误码 / 异常场景不变：
  - query/path 参数错误继续使用 `COMMON-400` 和 `validation.ParameterValidationError`。
  - 字段错误消息保持现有中文文案。
- 与 Java 版 OpenAPI / controller 契约差异：无新的对外契约差异。

## 7. 数据设计
- 不调整表结构、索引、唯一约束或 migration。
- 不新增或修改 sqlc query、generated model 或 repository model。
- 数据一致性边界不变；本次只在 HTTP handler 入口收敛查询参数值。

## 8. 关键流程
- 正常流程：
  1. strict server 解码 query params 为 `openapigen.ListAdminUsersParams`。
  2. `parseAdminUserListQuery` 设置分页默认值。
  3. `parseAdminUserListQuery` trim `username`、`email`、`status`，并用 trim 后的值做校验和赋值。
  4. `parseTimeParam` 继续负责单个时间字段的 trim、空值忽略和格式解析。
  5. 时间范围校验在 query 组装完成后执行。
  6. handler 将规范化后的 `usersvc.AdminUserListQuery` 交给 service。
- 异常流程：
  - 任意字段校验失败时收集 `validation.FieldErrors` 并返回 `ParameterValidationError`。
  - 非法时间格式保留字段级错误，不进入 service。
- 分层分工：
  - handler 负责 HTTP 参数解析、校验和 generated model 到 service query 的映射。
  - service 保留业务查询规则和防御性 normalize。
  - repository / sqlc/database 不参与本次改动。

## 9. 并发 / 幂等 / 缓存
- 查询参数规范化不涉及写操作、库存、订单、支付或状态流转。
- 不引入新的并发、幂等或缓存问题。
- 不调整事务边界。

## 10. 权限与安全
- `GET /api/v1/admin/users` 仍由现有认证和 RBAC middleware 保护。
- 本次不改变 JWT claim，不把角色、邮箱、用户名、用户状态写入 JWT。
- trim 后的筛选值仍通过 repository/sqlc 参数化查询传递，不拼接 SQL。
- 不新增敏感信息输出。

## 11. 测试策略
- 单元测试：
  - 新增 `internal/http/handler/user` 包测试，覆盖 `parseAdminUserListQuery` 会把带空白的 `username`、`email`、`status` 规范化后写入 service query。
  - 保留时间解析和范围校验现有行为。
- service / repository 测试：
  - 不涉及。
- migration / sqlc 验证：
  - 不涉及。
- 接口验证：
  - 运行 handler user 包测试和全量 Go 测试。
- OpenAPI validate：
  - 不涉及 API 契约变化。
- 异常场景验证：
  - 长度校验继续基于 trim 后的值。
  - 状态非法值继续返回 `COMMON-400` 参数校验错误。
- Java-Go parity 验证：
  - 更新 parity matrix 的 Auth、当前用户与管理员用户 API 行，索引本次 handler 边界收敛。
- 需要运行：
  - `gofmt`
  - `go test ./internal/http/handler/user`
  - `go test ./...`
  - `go vet ./...`
  - 如工具可用，运行 `golangci-lint run`。

## 12. 风险与替代方案
- 风险：
  - 工作区已有 strict-server 迁移相关未提交改动，需要避免扩大本次 diff。
  - handler 与 service 都保留 trim，可能看起来重复；但 service 的 normalize 仍是跨调用源防御边界。
- 备选方案：
  - 保持 parse/validate 两个函数，仅在 parse 阶段 trim 后赋值。
    - 未采用原因：当前校验与赋值都围绕同一组 query 字段，拆开会继续保留“校验值”和“赋值值”可能分叉的风险。
  - 只依赖 service normalize。
    - 未采用原因：service 能兜底查询行为，但 handler 输出的 service query 仍不够清楚。
  - 抽通用 query normalizer。
    - 未采用原因：当前只有管理员用户列表存在该局部需求，抽象会早于实际重复。
- 后续可演进点：
  - 如果更多列表查询出现同类规则，再评估是否抽取字段级 trim helper 或 query builder。
