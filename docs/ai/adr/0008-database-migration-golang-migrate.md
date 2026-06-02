# 数据库迁移工具 ADR

## 标题
Go 版 EventHub 使用 golang-migrate 管理数据库 migration

## 状态
- accepted

## 背景
Java 版 EventHub 使用 Flyway，当前 migration 来源是 `backend/src/main/resources/db/migration/V1__init_backend_foundation.sql`、`V2__stage_1_auth_jwt_rbac.sql`、`V3__create_auth_sessions.sql`。

Go 版需要保留版本化 SQL migration 和空库初始化能力，但不适合继续依赖 JVM 工具链。后续 CI、Testcontainers 和 Go 服务部署也需要能直接在 Go 生态内执行 migration 验证。

## 决策
Go 版使用 golang-migrate：

- migration 文件放在 `migrations/`。
- 文件命名使用 `000001_xxx.up.sql` / `000001_xxx.down.sql`。
- 本次 Go 版 `000001_system_bootstrap` 对齐 Java V1。
- 本次 Go 版 `000002_auth_schema` 合并对齐 Java V2 和 V3。
- 集成测试通过 golang-migrate library 在 Testcontainers MySQL 上执行 up/down。

## 备选方案
- 方案 1：继续使用 Flyway。
- 方案 2：使用 goose。
- 方案 3：应用启动时手写 schema 初始化。
- 方案 4：使用 golang-migrate。

## 决策理由
选择 golang-migrate 的原因：

- Go 生态成熟，支持 MySQL、file source 和测试内直接调用。
- 保留纯 SQL migration，方便与 Java Flyway 脚本逐项对齐。
- `up/down` 文件边界清晰，适合集成测试验证迁移可回滚。
- 避免 Go 服务的测试和部署继续依赖 JVM/Flyway。

Go 版合并 Java V2/V3 的原因：

- Go 版当前是从空库起步，没有线上历史版本需要逐号兼容。
- V2 users/roles/user_roles 与 V3 auth_sessions 都属于 auth 持久化底座，同一阶段落地更便于测试和后续 auth 迁移。
- 字段、约束、索引和 seed 仍对齐 Java 版，只是版本拆分不同。

## 影响
- 好处
  - Go 测试可直接运行 migration up/down。
  - 后续 CI 可用同一工具验证 migration。
  - 迁移链路和应用语言栈一致。
- 代价
  - 文件版本号不再与 Java Flyway V1/V2/V3 一一对应。
  - 后续如果需要跨语言共享 migration，需要额外定义同步规则。
- 后续可能需要调整的地方
  - 若 Go 版进入真实部署，应补充 migration CLI 或部署脚本。
  - 若需要支持多数据库，应为每种数据库单独设计 schema 和 migration 测试。
