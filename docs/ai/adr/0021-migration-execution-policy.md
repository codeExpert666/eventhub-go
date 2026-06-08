# ADR：数据库 migration 显式执行，不绑定应用启动

## 标题
Go 版 migration 由 Compose job 或 Makefile 显式执行，应用启动不自动迁移数据库

## 状态
- accepted

## 背景
Java 版 EventHub 使用 Flyway，Spring Boot 应用启动时可以自动执行 classpath 下的 migration。这在单体学习项目早期很方便，也让 `docker compose up` 能从空库启动到可用状态。

Go 版已经通过 ADR-0008 选择 golang-migrate 管理 `migrations/`。本次需要把本地 Docker Compose、Makefile 和运行镜像补齐，同时决定 migration 应该由 app 自动执行，还是由部署/开发流程显式执行。

活动预约与票务平台后续会涉及订单、库存、支付回调等状态敏感表。随着应用进入多副本或云原生部署，多个 app 实例在启动时同时尝试迁移数据库会带来锁、失败重试和权限边界问题。因此需要尽早把 schema 变更职责从业务进程启动中分离出来。

## 决策
Go 版采用显式 migration 策略：

- 应用进程不在 `Bootstrap` 或 HTTP server 启动中自动执行 migration。
- Docker Compose 新增一次性 `migrate` service：
  - 使用固定镜像 `migrate/migrate:v4.19.0`。
  - 挂载仓库 `migrations/`。
  - 等待 MySQL healthy 后执行 `up`。
  - app 依赖 `migrate` service `service_completed_successfully`。
- Makefile 提供：
  - `make migrate-up`
  - `make migrate-down`
  - `make compose-up`
- `migrate-down` 默认回退 1 个版本，可通过 `MIGRATE_STEPS` 覆盖。
- `make compose-up` 在执行 `docker compose up --build` 前先执行 `docker compose rm -sf migrate`，确保一次性 migration job 从新容器执行；启动已退出的旧容器也会重新运行 `up`，但会沿用旧容器创建时保存的 command/env/image 配置。
- 本地和 CI 可通过 `MIGRATE_DATABASE_URL` 指向目标数据库。

## 备选方案
- 方案 1：应用启动时自动执行 golang-migrate。
- 方案 2：只提供 Makefile migration，不在 Compose 中加入 migrate service。
- 方案 3：只依赖 Testcontainers 集成测试执行 migration。
- 方案 4：Compose job + Makefile 显式 migration。
- 方案 5：每次完整启动前只移除 `migrate` 容器，再复用现有 Compose 编排。

## 决策理由
选择方案 4：

- 保留 Java Compose 从空库启动的本地闭环体验。
- 不把 schema 变更职责塞进业务应用进程，避免后续多副本部署时 app 启动竞争 migration。
- Makefile 命令适合本地开发、CI 和手工排障。
- Compose `migrate` job 让 `docker compose up --build` 在空库上可重复启动，不要求开发者手工先运行迁移。
- `make compose-up` 先移除旧 `migrate` 容器，可以避免裸 Compose 复用已完成 job 时带来的状态和配置歧义；bind mount 中新增的 migration SQL 在旧容器重启时仍可见。
- `migrate-down` 默认 1 步更保守，避免误把本地 schema 全部回滚。

没有选择其他方案：

- 不选应用自动 migration：部署职责和业务启动职责混在一起，后续生产权限也更难收敛。
- 不选只提供 Makefile：`docker compose up --build` 无法保证空库可用，不符合本次完整本地闭环目标。
- 不选只依赖测试：测试能验证 migration，但不能提供开发运行时数据库初始化。
- 不选强制重建全部服务：只移除 `migrate` 容器已经能解决重复执行 migration 的问题，避免无谓重建 MySQL、Redis 或 app 容器。

## 影响
- 好处
  - 本地 Compose 可从空库启动到 app healthy。
  - migration 失败会阻止 app 启动，问题暴露更早。
  - 迁移职责清晰，可自然映射到后续 CI/CD 发布步骤。
  - 应用运行镜像不需要内置 migration CLI。
- 代价
  - Compose 中会出现一个正常退出的一次性 `migrate` 容器。
  - 本机手动运行应用前需要先执行 `make migrate-up`。
  - 本地完整启动应优先使用 `make compose-up`；裸 `docker compose up --build` 可能复用旧的 `migrate` 容器，虽然旧容器启动会重跑 command，但不会吸收后续修改过的 command/env/image 配置。
  - 生产部署需要明确 migration job 或发布阶段命令，不能只启动 app。
- 后续可能需要调整的地方
  - CI 可增加专门的 migration up/down 验证。
  - 生产部署文档需要定义 migration 权限、执行顺序和失败回滚策略。
  - 如果进入多服务拆分，需要定义各服务 migration ownership 和版本发布顺序。
