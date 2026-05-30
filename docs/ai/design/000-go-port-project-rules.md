# Go 版 EventHub AI 协作规则与质量规范设计文档

## 1. 背景
- 当前 Go 仓库还没有 AI 协作规则、文档模板、`docs/ai` 沉淀目录和 Java-Go parity 记录。
- Java 版 EventHub 已经沉淀了 `.agents/`、`.codex/`、`AGENTS.md`、`docs/templates/` 和 `docs/ai/`，其中包含设计优先、implementation note、ADR、固定总结格式等协作规则。
- Go 版目标是复刻 Java 版业务语义、接口契约、错误码、数据库模型、测试策略和文档沉淀方式，但不能逐行翻译 Java/Spring，需要建立 Go 生态自然的工程纪律。

## 2. 目标
- 在 Go 仓库建立 `.agents/`、`.codex/`、`AGENTS.md`、`docs/templates/` 和 `docs/ai/` 基线。
- `.codex/config.toml` 沿用 Java 版最小项目级 Codex 配置，并补充 Go 仓库说明。
- 沿用 Java 版模板大纲，并把 Spring、Maven、MyBatis、H2 语境替换为 Go、sqlc、migration、OpenAPI 和 Go 测试工具链语境。
- 明确非微小修改必须先写设计文档、实现后写 implementation note、关键取舍写 ADR。
- 明确 parity 文档是独立工作流，不只作为设计文档中的附带检查项。
- 明确 `AGENTS.md` 顶层规则和 `.agents/skills/backend-design-first/SKILL.md` 对 parity 的执行要求保持一致。
- 明确 `docs/ai/README.md` 对 parity 文档的说明应达到与 design、implementation、ADR 同等的可执行程度。
- 明确 Go 版分层：`handler -> service -> repository -> sqlc/database`。
- 明确 Go 质量门禁：`gofmt`、`go test ./...`、`go vet ./...`、可选 `golangci-lint run`、`sqlc generate`、migration 测试、OpenAPI validate。
- 新增 Java-Go parity matrix，为后续迁移业务语义和测试策略提供索引。

## 3. 非目标
- 本次不实现任何业务代码。
- 本次不初始化 Go module、不选择 HTTP router、不选择 migration 工具、不生成 sqlc 代码。
- 本次不迁移 Java 版具体业务模块，例如 auth、user、event、order、payment。
- 本次不改写 Java 版文档，只在 Go 仓库建立对应规则。

## 4. 影响范围
- 涉及目录：
  - `.agents/`
  - `.codex/`
  - `docs/templates/`
  - `docs/ai/design/`
  - `docs/ai/implementation/`
  - `docs/ai/adr/`
  - `docs/ai/parity/`
- 涉及文件：
  - `AGENTS.md`
  - `.agents/skills/backend-design-first/SKILL.md`
  - `.codex/config.toml`
  - `docs/templates/design-template.md`
  - `docs/templates/implementation-note-template.md`
  - `docs/templates/adr-template.md`
  - `docs/ai/README.md`
  - `docs/ai/design/000-go-port-project-rules.md`
  - `docs/ai/implementation/000-bootstrap-ai-docs.md`
  - `docs/ai/adr/0001-go-port-engineering-discipline.md`
  - `docs/ai/parity/java-go-parity-matrix.md`
- 涉及表 / 缓存 / 外部接口：
  - 无。本次只新增文档和协作规则。

## 5. 领域建模
- 核心实体：
  - `AGENTS.md`：项目级 AI 协作规则。
  - `backend-design-first` skill：非微小后端改动的设计优先流程。
  - `design-template`：需求实现前的设计文档模板。
  - `implementation-note-template`：实现后的复盘和验证记录模板。
  - `adr-template`：关键工程取舍记录模板。
  - `java-go-parity-matrix`：Java-Go 对齐状态索引。
- 实体关系：
  - `AGENTS.md` 定义全局协作规则。
  - `.agents` skill 把规则拆成可执行工作流。
  - `.codex/config.toml` 迁移 Java 版项目级 Codex 配置示例，用于保持两端协作入口一致。
  - `AGENTS.md` 和 skill 都必须写明 parity matrix 的触发条件、记录字段和无需更新时的说明方式。
  - `docs/templates` 提供文档格式约束。
  - `docs/ai` 按设计、实现、ADR、parity 分类沉淀实际记录。
  - `docs/ai/README.md` 说明各子目录用途、写作原则、模板约定、parity 更新规则和质量门禁。
  - `java-go-parity-matrix` 在实现后、总结前作为独立检查点更新。
- 关键状态：
  - 文档状态：初始化完成、后续按需求持续更新。
  - parity 状态：规则已初始化、业务模块待迁移、差异需记录。

## 6. API 设计
- 本次不新增运行时 API。
- Go 版未来 API 设计必须优先读取 Java 版 controller、OpenAPI、测试和错误码定义，确保路径、字段、状态码、错误码语义对齐。
- 未来 API 设计文档必须写清：
  - 接口列表
  - 请求参数
  - 响应结构
  - 错误码 / 异常场景
  - 与 Java 版契约的差异
- 如果 API 契约变化，需要运行 OpenAPI validate，或说明当前仓库尚未建立 OpenAPI 校验工具。

## 7. 数据设计
- 本次不新增数据库 schema、migration 或 sqlc query。
- Go 版未来数据设计必须优先对齐 Java 版数据库模型，包括表名、字段名、状态值、索引、唯一约束和关键查询语义。
- 未来涉及数据库改动时必须说明：
  - migration 文件和回滚策略
  - sqlc query 与生成模型影响
  - repository 如何包装 sqlc/database
  - 迁移测试或 schema 校验命令
- 本次不选择具体 migration 工具，避免在规则初始化阶段提前锁死技术栈。

## 8. 关键流程
- 正常流程：
  - 读取 Java 版协作规则、模板和相关 docs/ai 说明。
  - 在 Go 版建立同名目录、对应模板和最小 Codex 配置。
  - 写入 Go 版设计文档、implementation note、ADR 和 parity matrix。
  - 运行当前可行的文档级验证命令。
- 后续非微小修改流程：
  - 读取 Java 版相关业务和文档。
  - 读取 `.agents/skills/backend-design-first/SKILL.md`，确认当前仓库工作流。
  - 更新或新增 Go 版设计文档。
  - 按 `handler -> service -> repository -> sqlc/database` 实现。
  - 运行 Go 质量门禁和必要 parity 验证。
  - 更新 implementation note。
  - 检查并按需更新 parity matrix，记录 Java 来源、Go 目标、当前状态、差异原因和后续文档链接。
  - 如出现关键取舍，再更新 ADR。
- 异常流程：
  - 如果当前仓库尚无 `go.mod`、Makefile、sqlc 或 OpenAPI 文件，则对应验证项记录为暂不可运行，并说明原因。
  - 如果模板大纲不适用，必须在具体文档中说明调整原因。
- 状态流转：
  - 需求状态：`理解需求 -> 设计文档 -> 实现 -> 验证 -> 实现说明 -> 必要 ADR -> 总结`。

## 9. 并发 / 幂等 / 缓存
- 本次没有运行时并发、幂等或缓存逻辑。
- 规则层面要求后续涉及票务库存、订单创建、支付回调、通知投递等场景时，必须在设计文档中写清并发风险、幂等键、事务边界和缓存边界。
- Go 版禁止用 handler 直接拼接数据库逻辑来绕过 service 层的并发和幂等判断。

## 10. 权限与安全
- 本次不实现认证或授权代码。
- 规则层面明确未来 JWT claim 边界：
  - 可以包含稳定身份和技术性 claim，例如用户 ID / `sub`、`sid`、`jti`、`typ`、`iss`、`iat`、`exp`。
  - 不允许把角色、邮箱、用户名、用户状态写入 JWT。
- 未来权限判断应在服务端基于数据库或受控缓存获得最新用户状态和角色，避免 token 内动态属性陈旧。

## 11. 测试策略
- 本次验证重点：
  - 确认目标目录和文件已创建。
  - 使用 `git diff --check` 检查文档无明显空白错误。
  - 检查当前是否存在 Go 文件、`go.mod`、Makefile、sqlc 配置和 OpenAPI 文件。
- 后续代码改动验证策略：
  - `gofmt`：所有 Go 文件。
  - `go test ./...`：有 Go module 后必须运行。
  - `go vet ./...`：有 Go module 后必须运行。
  - `golangci-lint run`：已配置或工具可用时运行。
  - `sqlc generate`：SQL 或 sqlc 配置变化时运行。
  - migration 测试：migration 变化时运行。
  - OpenAPI validate：API 契约变化时运行。
  - Java-Go parity 验证：对照 Java 测试、OpenAPI、错误码和数据库模型。
- parity 文档更新触发条件：
  - API 契约、错误码、数据库模型、状态机、并发/幂等/缓存、认证授权、测试策略或刻意的 Go-only 实现差异发生变化。
  - 如果没有更新 parity matrix，需要在 implementation note 或最终总结中说明原因。
- 协作规则一致性验证：
  - `AGENTS.md` 必须显式包含 parity matrix 的更新时机、触发条件和记录字段，避免只在 skill 中存在细则。
  - `docs/ai/README.md` 必须显式说明 parity matrix 的定位、触发条件、记录字段、状态值和与 design/implementation/ADR 的关系。

## 12. 风险与替代方案
- 当前方案的风险：
  - Go 仓库仍未初始化业务工程，部分质量门禁暂时只能写入规则，不能完整执行。
  - 规则较严格，短期会增加每次改动的文档成本。
  - Java-Go parity matrix 初始只覆盖规则层面，业务迁移时需要持续补全；如果 `AGENTS.md`、skill 和 `docs/ai/README.md` 对 parity 的要求不一致，后续实现容易漏记对齐状态。
- 备选方案：
  - 方案 A：直接复制 Java 版 `.agents`、模板和文档，不做 Go 化改写。
  - 方案 B：先写业务代码，后续再补文档规范。
  - 方案 C：只保留 `AGENTS.md`，不建立 `docs/ai` 和模板。
- 为什么不选备选方案：
  - 不选方案 A：会把 Spring/Maven/MyBatis/H2 语境带入 Go 仓库，降低后续执行性。
  - 不选方案 B：违背 Java 版已经建立的设计先行规则，也不利于复盘 Go port 的关键取舍。
  - 不选方案 C：缺少模板和沉淀目录后，规则会停留在口头约定，后续实现难以审查。
