# MySQL 集成测试 ADR

## 标题
Go 版数据库测试使用 Testcontainers MySQL

## 状态
- accepted

## 背景
Java 版测试使用 H2 MySQL mode 来降低本地依赖，并通过 Flyway 初始化测试库。H2 能覆盖一部分 SQL 语法，但不完全等价于 MySQL。

Go 版本次工作直接引入 MySQL schema、外键、唯一约束、`LAST_INSERT_ID()`、时间戳默认值和 auth session 条件更新。使用替代数据库容易让 migration 和约束测试出现假阳性。

## 决策
Go 版数据库集成测试使用 Testcontainers MySQL：

- 测试容器镜像使用 `mysql:8.0.36`。
- 测试连接启用 `parseTime=true` 和 `multiStatements=true`。
- migration up/down 通过 golang-migrate 在容器内真实 MySQL 执行。
- repository 集成测试在 migration 后验证 seed、唯一约束、事务上下文和 auth session 条件更新。
- 测试启动前调用 Testcontainers provider health 检查；Docker 不可用时跳过容器测试。

## 备选方案
- 方案 1：H2 MySQL mode。
- 方案 2：SQLite 内存数据库。
- 方案 3：依赖本机固定 MySQL。
- 方案 4：Docker Compose 手工启动 MySQL。
- 方案 5：Testcontainers MySQL。

## 决策理由
选择 Testcontainers MySQL 的原因：

- 能验证真实 MySQL 行为，包括唯一约束错误号、外键、timestamp、multi statements 和条件 update。
- 测试环境自包含，不要求开发者提前创建数据库或保持固定端口。
- 与 Go 测试框架自然集成，CI 后续可直接复用。
- 比 H2/SQLite 更适合数据库底座阶段的 parity 验证。

## 影响
- 好处
  - migration up/down 和 repository SQL 的可信度更高。
  - 后续库存扣减、订单幂等、refresh token 并发测试可以沿用同一策略。
- 代价
  - 需要 Docker 或兼容 container provider。
  - 首次拉取镜像耗时较长。
  - 普通 `go test ./...` 会包含容器测试；Docker 不可用时会 skip。
- 后续可能需要调整的地方
  - CI 需要启用 Docker 服务。
  - 若测试耗时过高，可按 package 或 build tag 拆分慢速集成测试，但当前阶段先保持默认可运行。
