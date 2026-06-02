# 事务边界 ADR

## 标题
事务边界由 service 控制，repository/mysql 只复用上下文中的事务

## 状态
- accepted

## 背景
Java 版通常通过 Spring `@Transactional` 在 service 方法上声明事务边界。Go 版没有 Spring 容器，也不应该让 repository 自行决定业务事务范围。

后续 auth 注册至少需要把创建用户和绑定默认角色放进同一事务；refresh token 轮换依赖单条条件 update，但更复杂的业务如订单、库存、支付回调会需要明确事务范围。

## 决策
Go 版事务边界由 service 控制：

- `internal/platform/db.Transactor` 提供 `WithinTx(ctx, fn)`。
- `WithinTx` 开启 `*sql.Tx` 后写入 context。
- `repository/mysql` 每次执行 sqlc 查询时检查 context，如果存在事务则使用 `*sql.Tx`，否则使用普通 `*sql.DB`。
- repository interface 不暴露 `*sql.Tx`、sqlc generated type 或数据库细节。
- 已有事务上下文中再次调用 `WithinTx` 时复用当前事务。

## 备选方案
- 方案 1：repository 每个方法内部自行开启事务。
- 方案 2：service 直接持有 `*sql.Tx` 并传给 repository 方法。
- 方案 3：使用 context 传递事务，由 repository/mysql 选择执行器。
- 方案 4：引入第三方 transaction manager。

## 决策理由
选择 service 控制 + context 传递事务的原因：

- 业务用例最清楚哪些写入必须属于同一事务。
- repository 保持持久化语义，不决定业务事务范围。
- handler 不接触数据库或 transaction handle。
- 方法签名不被 `*sql.Tx` 污染，repository interface 更稳定。
- 嵌套 service 调用可复用当前事务，避免意外开启独立事务。

## 影响
- 好处
  - 符合 `handler -> service -> repository -> sqlc/database` 分层。
  - 后续注册、订单、库存等跨表写入能由 service 明确包裹。
  - repository/mysql 可以统一包装 sqlc generated code。
- 代价
  - context 中存放事务需要谨慎，只能由 platform/db 管理 key。
  - 如果 service 忘记使用 `Transactor`，跨表写入不会自动事务化。
  - 测试需要覆盖事务内 repository 使用场景。
- 后续可能需要调整的地方
  - auth service 落地时必须对注册事务补 service 测试。
  - 订单和库存阶段需要补并发事务测试和隔离级别设计。
