# Java-Go parity matrix 审计设计

## 1. 背景
- 当前 Go 仓库已经完成 HTTP foundation、项目结构规范化、HTTP DTO/VO 边界、service contract 边界和 HTTP handler/DTO 模块化组织。
- `docs/ai/parity/java-go-parity-matrix.md` 已多次追加记录，出现了基础规则、运行时代码、后续待迁移能力混在一起的问题。
- Java 版对应来源包括：
  - `AGENTS.md`、`.agents/skills/backend-design-first/SKILL.md`、`docs/templates/*`
  - `backend/src/main/java/com/eventhub/common/*`
  - `backend/src/main/java/com/eventhub/modules/system/*`
  - `backend/src/main/java/com/eventhub/modules/auth/*`
  - `backend/src/main/resources/db/migration/*`
  - `backend/src/main/resources/mapper/auth/*`
  - `backend/src/test/java/com/eventhub/**`

## 2. 目标
- 核验 parity matrix 是否与当前 Go 代码、docs/ai 记录和 Java 参考项目进度一致。
- 删除重复表达，例如把“业务错误”和“错误码与业务错误”收敛为单一基础错误行。
- 补齐当前矩阵遗漏的 Java 已有能力，例如 OpenAPI、auth API、JWT/RBAC、refresh token 会话、数据库迁移、MyBatis mapper、H2 数据库测试和容器化配置；对 Java 仅有 roadmap、尚无 production code 的后续业务标为待决策。
- 保持矩阵作为索引表，不把详细设计、实现过程或测试细节堆在说明列。

## 3. 非目标
- 不新增或修改 Go 运行时代码。
- 不实现 auth、user、database、OpenAPI、Docker、event/order/payment 等业务或基础设施能力。
- 不改变任何 API 路径、请求字段、响应字段、错误码或测试语义。
- 不新增 ADR；本次只是文档审计和矩阵收敛，没有新的架构取舍。

## 4. 影响范围
- 本次触及：
  - `docs/ai/design/006-parity-matrix-audit.md`
  - `docs/ai/implementation/006-parity-matrix-audit.md`
  - `docs/ai/parity/java-go-parity-matrix.md`
- 本次明确不触及：
  - `cmd`
  - `internal/app`
  - `internal/config`
  - `internal/platform`
  - `internal/http`
  - `internal/service`
  - `internal/domain`
  - `internal/repository`
  - `internal/security`
  - `api/openapi`
  - `migrations`
  - `configs`
- 本次影响 `docs/ai/parity/java-go-parity-matrix.md`。这是本次工作的核心目标。

## 5. 领域建模
- `Parity Matrix`：Java-Go 对齐索引，只记录来源、Go 目标、状态和差异原因。
- `已对齐`：当前 Go 代码或文档已经能支撑对应语义。
- `已决策`：方向已由文档或 ADR 固化，但代码只按当前阶段部分落地。
- `待迁移`：Java 版已有实现或明确文档，Go 版尚未实现。
- `待决策`：Go 版需要新的设计或 ADR 才能决定迁移方式。

## 6. API 设计
- 本次不新增或修改 API。
- 矩阵需要继续记录当前已对齐 API：
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET/HEAD /actuator/health`
  - `GET/HEAD /actuator/info`
- 矩阵需要明确当前待迁移 API：
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout`
  - `GET /api/v1/me`
  - `GET /api/v1/admin/users`
  - `PATCH /api/v1/admin/users/{userId}/status`

## 7. 数据设计
- 本次不新增数据库表、索引、唯一约束、migration 或 sqlc query。
- 矩阵需要明确 Java 版已有 Flyway V1-V3、users/roles/user_roles/auth_sessions、MyBatis mapper 与 Go 版当前仅有 `migrations/.gitkeep` 的差距。
- sqlc/database + repository 边界已在规则和 ADR 中固化，具体 `sqlc.yaml`、query、generated model 和 migration 测试仍待数据库阶段迁移。

## 8. 关键流程
- 审计流程：
  1. 读取当前 Go 代码和已跟踪文件列表。
  2. 读取当前 parity matrix。
  3. 对照 Go `docs/ai/design`、`implementation`、`adr`。
  4. 对照 Java 参考项目中的 system、common、auth、security、migration、mapper 和测试文件。
  5. 将重复行合并，将过度宣称状态降级或说明边界，将遗漏能力补为待迁移或待决策。
- 分层分工：
  - 本次只改 docs，不影响 `handler -> service -> repository -> sqlc/database`。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及运行时并发、幂等或缓存。
- 矩阵需要保留 refresh token 轮换、auth session 乐观锁和重放检测等 Java 已有并发/安全语义的待迁移状态。

## 10. 权限与安全
- 本次不实现认证或授权。
- 矩阵需要明确：
  - Go 版已在规则中初始化 JWT claim 边界。
  - Java 版 SecurityConfig、JWT filter、AuthSession、RBAC 和管理员接口在 Go 版仍待迁移。
  - 不把角色、邮箱、用户名、用户状态写入 JWT 的约束继续保留。

## 11. 测试策略
- 文档结构验证：
  - `git diff --check`
- Go 质量门禁：
  - `go test ./...`
  - `go vet ./...`
  - `make test`
  - `make vet`
- `gofmt` 不适用，本次不改 Go 文件。
- `sqlc generate` 不适用，本次没有 SQL 或 sqlc 配置变化。
- migration 测试不适用，本次没有 migration 变化。
- OpenAPI validate 不适用，本次没有 OpenAPI 契约文件。

## 12. 风险与替代方案
- 当前方案的风险：
  - 过度合并可能让后续查找单个能力变慢。
  - 过度拆分会重新制造重复行。
- 备选方案：
  - 方案 A：只在原矩阵上追加遗漏行。
  - 方案 B：完全按 Java 文件逐项列出。
  - 方案 C：按当前阶段的语义能力收敛矩阵。
- 为什么不选备选方案：
  - 不选方案 A：原矩阵已有重复表达，继续追加会更难维护。
  - 不选方案 B：Java 文件级清单会让矩阵膨胀，失去索引价值。
  - 选择方案 C：用语义能力做最小完整集合，既能覆盖当前进度，也便于后续按模块追加。
- 后续可演进点：
  - 每迁移一个新模块，只新增或更新对应语义能力行。
  - 如果某个待迁移能力进入设计阶段，再把状态从 `待迁移` 调整为 `已决策` 或 `已对齐`。
