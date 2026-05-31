# Go 项目目录结构规范实现说明

## 1. 本次改动解决了什么问题

本次改动解决了 Go 版 EventHub 后续生成代码时缺少明确 package layout 约束的问题。

在 HTTP foundation 已完成、业务模块和数据库尚未迁移的阶段，先把长期目录结构、分层职责、阶段化落地原则和禁止偏离规则写入 `AGENTS.md`、backend-design-first skill、docs/ai README、ADR 和 parity matrix，避免后续实现 auth/user/event/order/payment 时出现 handler 直连数据库、domain 依赖 sqlc model、业务逻辑散落在入口文件或为了凑结构创建空 Go package 等问题。

## 2. 改动内容
- 新增了本次设计文档：
  - `docs/ai/design/002-project-structure-alignment.md`
- 更新了项目级协作规则：
  - `AGENTS.md`
  - 新增“Go 项目目录结构规范”，包含总原则、规范目录结构、阶段化落地原则、生成代码前结构检查清单和禁止偏离规则。
- 更新了 backend-design-first skill：
  - `.agents/skills/backend-design-first/SKILL.md`
  - 在 Understand and scope 之后新增 `Structure conformance check`，要求设计前列出涉及目录、不涉及目录、目录移动原因、ADR 要求和结构债务说明。
- 更新了 docs/ai 目录说明：
  - `docs/ai/README.md`
  - 新增“目录结构与文档联动”，说明目录结构变化属于非微小修改，并要求 package layout 决策和 Java-Go 目录映射进入 ADR 与 parity matrix。
- 新增了 package layout ADR：
  - `docs/ai/adr/0005-go-project-package-layout.md`
  - 状态为 accepted，记录采用混合式 Go package layout 的原因、备选方案和影响。
- 更新了 Java-Go parity matrix：
  - `docs/ai/parity/java-go-parity-matrix.md`
  - 新增 `已决策` 状态说明。
  - 新增 `Go package layout / 项目目录结构` 行，记录 Java Controller / Service / Mapper / Entity / Config / Security 到 Go `cmd`、`internal/app`、`internal/http`、`internal/service`、`internal/repository`、`internal/domain`、`internal/platform`、`internal/security` 的映射。
- 文件移动和 package 边界变化：
  - 本次没有移动任何 Go 文件。
  - 本次没有新建空 Go package。
  - 本次没有改变现有运行时 package 边界，只新增和更新文档规则。
- 是否更新 Java-Go parity 记录：
  - 已更新。目录结构规范属于 Java 分层到 Go package layout 的刻意差异和长期工程决策，触发 parity matrix 更新条件。

## 3. 为什么这样设计
- 目录规范写入 `AGENTS.md`，是为了让项目级持久规则直接约束后续所有 Codex 任务。
- 结构检查写入 backend-design-first skill，是为了让每次设计和实现前先判断代码应落在哪一层，而不是实现后再补救。
- `docs/ai/README.md` 补充目录结构与文档联动，是为了明确移动 package、拆分层次、引入 repository/sqlc/openapi/migrations 都不是微小修改，必须留下设计和实现记录。
- 单独新增 ADR，是因为 package layout 会长期影响 auth/user/event/order/payment、数据库、OpenAPI 和安全模块的组织方式，属于架构级取舍。
- parity matrix 新增映射行，是为了记录 Go 版不逐行复刻 Spring Boot，而是用 Go package、`internal`、constructor injection、显式 router 和 repository interface 表达同等边界。
- 不创建空 Go package，是为了保持当前仓库可编译、可读、可演进，避免只有目录形状但没有业务含义的 package。

## 4. 替代方案
- 方案 A：只在 `AGENTS.md` 中写目录规范。
  - 没有采用，因为每次执行任务时还需要 skill 级的结构检查步骤，否则规则容易停留在静态文档里。
- 方案 B：直接创建完整目标目录和空 `.go` 文件。
  - 没有采用，因为当前阶段尚未实现这些业务包；空 Go package 会增加无意义编译单元，也违背阶段化落地原则。
- 方案 C：采用完全横向分层。
  - 没有采用，因为业务变多后容易出现包过宽，但保留核心横向层仍有利于 Java-Go parity 对照。
- 方案 D：采用完全纵向 modules。
  - 没有采用，因为当前迁移阶段需要清晰对照 Java Controller / Service / Mapper / Entity / Security，完全纵向 modules 会降低学习和审查直观性。
- 方案 E：不新增 ADR，只在设计文档中说明。
  - 没有采用，因为 package layout 是长期工程决策，后续偏离也要有 ADR 索引。

## 5. 测试与验证
- 跑了哪些测试：
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
- 跑了哪些质量门禁：
  - `grep -R "handler -> service -> repository" AGENTS.md .agents/skills docs/ai`：通过，确认分层规则仍可检索。
  - `grep -R "Go 项目目录结构规范" AGENTS.md`：通过，确认 AGENTS 已新增章节。
  - `grep -R "Structure conformance check" .agents/skills/backend-design-first/SKILL.md`：通过，确认 skill 已新增结构检查步骤。
  - `git diff --check`：通过，无空白错误。
  - `gofmt`：不适用，本次没有修改 Go 文件。
  - `sqlc generate`：不适用，本次没有 SQL、schema 或 sqlc 配置变化。
  - migration 测试：不适用，本次没有 migration 变化。
  - OpenAPI validate：不适用，本次没有 API 契约变化。
- 手工验证了哪些场景：
  - 复查 `AGENTS.md`，确认保留原有 Java-Go parity、JWT claim、质量门禁和 7 项总结规则。
  - 复查 `backend-design-first` skill，确认结构规范检查位于 Understand and scope 之后、Design before implementation 之前。
  - 复查 ADR，确认覆盖背景、决策、三类备选方案、决策理由和影响。
- Java-Go parity 如何验证：
  - 对照 Java Controller / Service / Mapper / Entity / Config / Security 分层，确认 Go 目标记录为 `cmd`、`internal/app`、`internal/http`、`internal/service`、`internal/repository`、`internal/domain`、`internal/platform`、`internal/security`。
  - parity matrix 已索引本次设计文档、implementation note 和 ADR。
- 结果如何：
  - 本次为文档和协作规则修改，不改变运行时行为。
  - 当前 Go 测试和 vet 均通过，规则修改没有破坏项目编译和静态检查。

## 6. 已知限制
- 当前实际代码目录尚未完全覆盖长期目标结构，例如 `internal/app`、`internal/domain`、`internal/service`、`internal/repository`、`internal/security`、`api/openapi`、`migrations`、`configs` 仍未落地。
- 这属于阶段性结构债务：目录规范已决策，但未开始的业务和基础设施不创建空 Go package。
- 后续引入 auth/user/event/order、数据库、sqlc、OpenAPI 或 migration 时，需要按本次规则补齐对应目录，并更新设计文档、implementation note 和 parity matrix。
- 如果未来业务复杂度要求偏离当前 package layout，需要先写明原因并新增或更新 ADR。

## 7. 对后续版本的影响
- 对简历可用版的价值：
  - 后续代码结构会更稳定，能清楚展示 Go 版如何对齐 Java 分层但不复制 Spring Boot 目录。
- 对微服务 / 云原生演进的影响：
  - `internal/platform`、`internal/security`、`internal/service`、`internal/repository` 的边界有助于后续拆分服务和抽取基础设施。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响：
  - 新增数据库访问时必须按 `internal/repository/mysql/queries`、`internal/repository/mysql/sqlc`、`internal/repository/mysql` 和根目录 `sqlc.yaml` 落地。
  - 新增 OpenAPI 时必须放在 `api/openapi/eventhub.yaml` 和 `api/openapi/gen/`，且生成代码不能污染 domain model。
  - 每次结构变化都要记录是否存在结构债务。
