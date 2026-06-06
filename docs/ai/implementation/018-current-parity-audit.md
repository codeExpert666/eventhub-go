# Current Parity Audit

## 1. 本次改动解决了什么问题

本次完成 Go 版 EventHub 对 Java 当前阶段的 parity audit，重新核验 Java 当前接口、错误码、DTO/VO、migration、MyBatis mapper、测试目录、OpenAPI/Swagger 配置和安全配置，并与 Go 当前实现、OpenAPI、migration、sqlc query 和测试覆盖对照。

审计结论：

- 未发现需要修改生产代码的 P0 差异。
- 发现一个 P1 测试可读性缺口：Go 已有分散测试覆盖 auth/admin 核心场景，但缺少一条串联 register/login/me/refresh/replay/logout/admin/disabled-old-token 的 smoke flow。
- P1 已通过新增 Go HTTP 集成测试补齐。
- P2 有意差异已记录在 parity checklist：Go spec-first OpenAPI 路径、prod 文档关闭状态码、sqlc offset 溢出保护。

## 2. 改动内容
- 新增了什么
  - 新增 `docs/ai/design/018-current-parity-audit.md`，按设计模板记录当前审计目标、范围、API/data/security/test 策略和风险。
  - 新增 `docs/ai/parity/current-auth-contract-checklist.md`，逐项记录接口、字段、响应 envelope、错误码、schema、JWT、refresh、logout、RBAC、prod OpenAPI 和测试覆盖的 P0/P1/P2 状态。
  - 新增 `docs/ai/parity/test-coverage-comparison.md`，把 Java 当前测试类别映射到 Go 对应测试文件，并记录剩余自动化边界。
  - 新增 `internal/http/auth_integration_test.go` 的 `TestAuthParitySmokeFlow`，串联覆盖 register、login、me、refresh、old refresh replay、logout、admin list、admin disable user、disabled user old token rejected。
- 修改了什么
  - 更新 `docs/ai/parity/java-go-parity-matrix.md`，增加本次 current parity audit 索引，并把 OpenAPI、数据库测试、auth API 和安全响应行链接到新 checklist / test comparison。
- 删除了什么
  - 未删除文件或生产逻辑。
- 是否更新 Java-Go parity 记录
  - 已更新主矩阵，并新增两个专题 parity 文档。

## 3. 为什么这样设计
- 关键设计原因
  - 用户要求的是当前阶段 parity audit，核心价值是把“已对齐、未对齐、有意差异”的判断沉淀成可追溯文档，而不是为了审计而改动生产逻辑。
  - 新增 smoke 测试选择 HTTP 集成层，因为它能一次穿过 router、auth middleware、RBAC middleware、handler、service 和 repository interface fake，最贴近接口契约。
  - 真实 MySQL 行为仍交给 `internal/repository/mysql/mysql_repository_integration_test.go`，避免把一个 smoke 测试变成笨重的外部环境依赖。
- 与 Go 项目当前阶段的匹配点
  - 保持 `handler -> service -> repository -> sqlc/database` 分层，不让 handler 访问数据库。
  - 不引入新的接口抽象或外部依赖。
  - 不变更 JWT claim 边界，仍只使用稳定身份和技术 claim。
- 与 Java 版业务语义的对齐方式
  - Java register/login/refresh/logout/me/admin list/admin status 的路径、字段、错误码和状态语义在 checklist 中逐项映射。
  - Java refresh token 轮换、旧 token replay、logout no-op、禁用用户旧 token 失效和 RBAC 语义已在 Go 测试中覆盖。

## 4. 替代方案
- 方案 A
  - 只更新 parity 文档，不新增测试。
  - 未采用原因：用户明确要求实现必要 smoke/e2e 覆盖；Go 原有测试分散，缺少一条读起来就能确认当前 auth/admin 闭环的用例。
- 方案 B
  - 新增 Docker Compose 外部 smoke 脚本并启动真实 app。
  - 未采用原因：当前 Makefile 和 compose 文件没有独立 smoke target 或 smoke 服务；直接把本次审计扩展为容器编排任务会引入端口、迁移时机和环境依赖噪声。后续可以独立补 `make smoke`。
- 方案 C
  - 启动 Java 应用导出 `/v3/api-docs`，再和 Go `/openapi.yaml` 做机器 diff。
  - 未采用原因：当前 Java-Go 主要差异在 Springdoc runtime path 与 Go spec-first path；源码、DTO/VO、测试断言和 Go OpenAPI 静态对照已经足以完成当前阶段 P0 审计。机器 diff 更适合后续独立工具化。

## 5. 测试与验证
- 跑了哪些测试
  - `make fmt vet test`
  - `make test-race`
  - `make openapi-validate`
  - `docker compose config --services`
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `make fmt vet test` 已执行，其中包含 `gofmt -w .`、`go vet ./...`、`go test ./...`。
  - 本次没有修改 SQL、migration 或 sqlc 配置，因此未运行 `sqlc generate`。
- 手工验证了哪些场景
  - 通过源码对照确认 Java Controller/DTO/VO、ErrorCode、migration、mapper、安全配置、OpenAPI 配置和测试目录。
  - 通过新增 `TestAuthParitySmokeFlow` 验证 auth/admin 关键链路。
  - 通过 `docker compose config --services` 确认当前 compose 服务只有 `redis/mysql/migrate/app`，没有独立 smoke 服务。
- Java-Go parity 如何验证
  - `current-auth-contract-checklist.md` 按 P0/P1/P2 逐项记录 Java 来源、Go 证据和状态。
  - `test-coverage-comparison.md` 按 Java 测试类别映射 Go 测试文件。
- 结果如何
  - `make fmt vet test`：通过。
  - `make test-race`：通过。
  - `make openapi-validate`：通过。
  - docker compose smoke：当前仓库没有独立 smoke 服务或 Makefile target，未运行；已记录为后续演进点。

## 6. 已知限制
- 当前版本还缺什么
  - 没有独立 Docker Compose smoke target；本次用 Go in-process HTTP integration smoke 覆盖契约链路。
  - 没有自动导出 Java `/v3/api-docs` 并与 Go `/openapi.yaml` 做 schema diff。
- 哪些地方后面需要继续演进
  - 可新增 `make smoke`，在 MySQL/Redis/migrate/app 全部启动后运行外部 HTTP smoke。
  - 可新增 OpenAPI diff 工具，允许配置 Java `/v3/api-docs` 与 Go `/openapi.yaml` 的路径映射后比较 schema 字段。
  - 后续 event/order/payment/notification/audit 模块开始迁移时，需要新增模块级 parity checklist。
- 与 Java 版仍有哪些差距
  - Go 不复刻 Springdoc 路径 `/v3/api-docs` / `/swagger-ui.html`，使用 `/openapi.yaml` / `/swagger/*`。
  - Go prod 文档关闭时不注册文档路由，匿名和带 token 都返回 `COMMON-404`；Java 匿名请求先进入认证边界返回 `AUTH-401`，带 token 后资源关闭返回 `COMMON-404`。
  - 这些差异不影响当前业务契约和安全目标，已记录为 P2 有意差异。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 当前 auth / RBAC / refresh token / prod OpenAPI hardening 具备可展示的 Java-Go parity 证据链。
  - 新增 smoke flow 让核心认证闭环更容易被 reviewer 快速理解。
- 对微服务 / 云原生演进的影响
  - JWT claim 最小化、MySQL auth session 权威记录和 Redis 不参与强一致的边界继续保持清晰，方便后续拆分认证服务或接入缓存/denylist。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续业务模块迁移可复用 `current-auth-contract-checklist.md` 的审计格式。
  - 新增 API 或 schema 时仍需同步 OpenAPI、migration/sqlc、parity matrix、设计文档和 implementation note。
