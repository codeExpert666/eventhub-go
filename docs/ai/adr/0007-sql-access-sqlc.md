# SQL 访问方式 ADR

## 标题
Go 版 EventHub 使用 sqlc + database/sql 访问 MySQL

## 状态
- accepted

## 背景
Java 版 EventHub 当前使用 MyBatis XML 管理 SQL 和 resultMap。其优势是 SQL 显式、字段映射可审查、查询语义稳定，适合持续对齐 API、错误码和数据库模型。

Go 版需要复现这些业务语义，但不应逐行迁移 MyBatis 或引入 Spring 风格 mapper。数据库访问还必须遵守 `handler -> service -> repository -> sqlc/database`，不能让 handler 或 service 直接依赖 generated row，也不能让 ORM model 变成 domain model。

## 决策
Go 版数据库访问采用 sqlc + `database/sql`：

- SQL 文件放在 `internal/repository/mysql/queries/`。
- sqlc generated code 放在 `internal/repository/mysql/sqlc/`。
- repository interface 放在 `internal/repository/`。
- MySQL repository wrapper 放在 `internal/repository/mysql/`。
- wrapper 负责 sqlc row 与 repository model 的映射，不向 HTTP 或 service 暴露 generated model。

## 备选方案
- 方案 1：GORM。
- 方案 2：sqlx + 手写 scan。
- 方案 3：ent。
- 方案 4：sqlc + `database/sql`。

## 决策理由
选择 sqlc + `database/sql` 的原因：

- 保留 Java MyBatis XML 的显式 SQL 可审查性。
- 生成类型安全的查询参数和结果结构，减少手写 scan 样板。
- `database/sql` 是 Go 标准库抽象，配合 MySQL driver 可满足当前需求。
- generated code 被限制在 sqlc 包内，repository/mysql 显式映射，能防止 sqlc model 泄漏到 HTTP 或 domain。
- 不引入 ORM 运行时行为，避免隐藏 SQL、自动迁移或懒加载等不利于 Java-Go parity 的复杂性。

## 影响
- 好处
  - SQL 与 Java mapper 更容易逐项对照。
  - 编译期能发现查询参数或结果字段变化。
  - 后续 repository 测试可以直接覆盖真实 SQL。
- 代价
  - 每次修改 SQL 或 schema 后必须运行 `make sqlc`。
  - 动态筛选查询需要用 sqlc macro 或拆查询，写法比 ORM 更显式。
  - generated code 文件数量增加。
- 后续可能需要调整的地方
  - 如果未来需要复杂动态查询，可在 service/repository 设计中评估是否拆分 query 或局部使用 query builder，但不得让 ORM 侵入 domain/handler。
