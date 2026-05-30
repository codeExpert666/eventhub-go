# Go 版 AI 协作文档基线初始化实现说明

## 1. 本次改动解决了什么问题

本次改动解决了 Go 版 EventHub 缺少 AI 协作规则、文档模板、质量门禁和 Java-Go parity 记录入口的问题。

Go 仓库后续要复刻 Java 版 EventHub 的业务语义、接口契约、错误码、数据库模型和测试策略，因此需要先把设计优先、implementation note、ADR 和质量验证流程固定下来，避免后续业务迁移时只有代码、没有可复盘的决策记录。

## 2. 改动内容
- 新增了项目级协作规则 `AGENTS.md`。
- 根据 review 反馈，补充了 `AGENTS.md` 中的 parity matrix 工作流，明确触发条件、记录字段和无需更新时的说明要求。
- 新增了 `.agents/skills/backend-design-first/SKILL.md`，把非微小后端改动约束为设计优先、实现后记录、必要时写 ADR。
- 根据 review 反馈，补充了 `.agents/skills/backend-design-first/SKILL.md` 中的独立 parity 文档步骤，明确何时更新、更新哪里、记录什么。
- 新增了 `.codex/config.toml`，沿用 Java 版最小项目级 Codex 配置，并补充 Go 仓库说明。
- 新增了 Go 版文档模板：
  - `docs/templates/design-template.md`
  - `docs/templates/implementation-note-template.md`
  - `docs/templates/adr-template.md`
- 新增了 `docs/ai/README.md`，说明设计、实现、ADR 和 parity 目录用途。
- 根据 review 反馈，补充了 `docs/ai/README.md` 中的 parity 文档约定，使其说明充分程度与 design、implementation、ADR 保持一致。
- 新增了本次设计文档 `docs/ai/design/000-go-port-project-rules.md`。
- 新增了本实现说明 `docs/ai/implementation/000-bootstrap-ai-docs.md`。
- 新增了工程纪律 ADR `docs/ai/adr/0001-go-port-engineering-discipline.md`。
- 新增了 Java-Go parity matrix `docs/ai/parity/java-go-parity-matrix.md`。
- 是否更新 Java-Go parity 记录：
  - 已新增 parity matrix，记录规则、模板、分层、质量门禁、JWT claim 边界等初始对齐状态。
  - 已更新 parity matrix 中 docs/ai 目录行，说明 README 已补齐 parity 文档约定。
  - 已更新 parity matrix 中 AI 协作规则行，说明 `AGENTS.md` 已补齐 parity 文档执行要求。
  - 已更新 parity matrix 中 Agent skill 行，说明 skill 现在包含独立 parity 文档步骤。

## 3. 为什么这样设计
- 沿用 Java 版模板大纲，可以保持两端文档沉淀方式一致，后续对照复盘时不需要重新适应结构。
- 模板内容改写为 Go 语境，可以避免把 Spring、Maven、MyBatis、H2 的实现细节误带到 Go 仓库。
- 把质量门禁写入 `AGENTS.md`、skill、模板和 `docs/ai/README.md`，是为了让规则在任务入口、执行流程和文档输出中都可见。
- 把 parity 文档写成 skill 的独立步骤，是为了让 Java-Go 对齐不只停留在设计阶段，而是在实现后、总结前形成明确检查点。
- 把同样的 parity 要求补进 `AGENTS.md`，是为了让项目顶层规则和可执行 skill 不产生口径差异。
- 把 parity 文档约定补进 `docs/ai/README.md`，是为了让目录说明本身也能指导怎么维护 parity，而不只告诉读者有一个 `parity/` 子目录。
- 把 `handler -> service -> repository -> sqlc/database` 写成硬约束，是为了后续业务迁移时避免 handler 直接访问数据库，也避免 sqlc 生成代码承载业务判断。
- 单独新增 ADR，是因为“Go port 不逐行翻译 Java，而是以工程纪律和 parity 文档约束迁移”属于长期影响后续开发方式的关键决策。

## 4. 替代方案
- 方案 A：直接复制 Java 版全部文档和配置。
  - 没有采用，因为 Java 模板包含 Spring/Maven/MyBatis/H2 语境，直接复制会让 Go 端质量门禁不准确；但 `.codex/config.toml` 属于项目级协作配置，已按 Java 版最小配置迁移。
- 方案 B：只写 `AGENTS.md`，不新增模板和 docs/ai 目录。
  - 没有采用，因为后续每次非微小修改需要稳定文档落点，仅有口头规则不便审查。
- 方案 C：本次顺手初始化 Go module、router、migration 和 sqlc。
  - 没有采用，因为当前任务明确要求不实现业务代码；技术栈选择应在对应工程基础设计中单独论证。
- 方案 D：只在设计文档中提 parity，不在 skill workflow 中增加独立步骤。
  - 没有采用，因为实现后才最容易发现实际差异；没有独立步骤会让 parity matrix 在真实迁移中被遗漏。
- 方案 E：只在 skill 中写 parity 细则，不改 `AGENTS.md`。
  - 没有采用，因为 `AGENTS.md` 是项目级入口规则；顶层规则缺少细则会让 agent 在不读取 skill 的场景下漏掉 parity 更新。
- 方案 F：保持 `docs/ai/README.md` 只做目录索引，不展开 parity 维护规则。
  - 没有采用，因为 design、implementation、ADR 在 README 中都有写作原则和模板入口；parity 也需要同等可执行的说明。

## 5. 测试与验证
- 跑了哪些测试：
  - 本次没有 Go 代码和 Go module，未运行代码级测试。
- 跑了哪些质量门禁：
  - `git diff --check`：通过，无空白错误。
  - 文件和目录存在性检查：通过，目标 `.agents`、`.codex`、`docs/templates`、`docs/ai` 文件已创建。
  - Go 项目状态检查：`go env GOMOD` 返回 `/dev/null`，且当前没有 `.go` 文件。
- 手工验证了哪些场景：
  - 对照 Java 版 `AGENTS.md`、`.agents` skill、`.codex/config.toml`、`docs/templates` 和 `docs/ai/README.md`，确认 Go 版规则已覆盖设计优先、implementation note、ADR 和固定总结格式。
  - 复查 `docs/ai/README.md`，确认 parity 文档的定位、触发条件、记录字段、状态值和与其他文档的关系已补齐。
  - 复查 `AGENTS.md`，确认 parity matrix 的更新时机、触发条件、记录字段和无需更新时说明要求已写入顶层规则。
  - 复查 `.agents/skills/backend-design-first/SKILL.md`，确认 parity matrix 已从泛化检查项提升为独立 workflow step。
- Java-Go parity 如何验证：
  - 新增 `docs/ai/parity/java-go-parity-matrix.md`，记录初始规则层面对齐和后续业务待迁移项。
  - 更新 docs/ai 目录对齐说明，记录 README 中 parity 文档约定已补齐。
  - 更新 AI 协作规则对齐说明，记录 `AGENTS.md` 中 parity 文档要求已补齐。
  - 更新 Agent skill 对齐说明，记录 parity 文档步骤已补齐。
- 结果如何：
  - 当前变更为文档和协作规则初始化，不涉及运行时行为。
  - `gofmt`、`go test ./...`、`go vet ./...` 当前不适用，原因是仓库尚未建立 Go module 且没有 Go 源文件；后续工程初始化后纳入强制验证。

## 6. 已知限制
- 当前 Go 仓库仍没有 `go.mod`、业务 package、sqlc 配置、migration 工具或 OpenAPI 文件。
- parity matrix 目前只建立索引和规则基线，尚未记录具体业务模块的一一对齐结果。
- `.codex/config.toml` 已迁移 Java 版最小项目级配置；实际生效仍取决于 Codex 是否信任项目，以及当前会话权限策略。
- 质量门禁已经写入规则，但部分命令要等 Go 工程初始化后才能实际运行。

## 7. 对后续版本的影响
- 对简历可用版的价值：
  - 后续每个业务模块都能形成设计、实现、ADR 和 parity 证据链，便于复盘和讲清工程取舍。
- 对微服务 / 云原生演进的影响：
  - 通过 ADR 和 parity matrix 记录早期单体边界、数据库模型和接口契约，后续拆分服务时更容易判断哪些语义必须保持稳定。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响：
  - 后续初始化工程基础时，需要把本文档中的质量门禁落成 Makefile、CI 或脚本。
  - 后续新增 API、数据库和业务模块时，需要同步更新 parity matrix。
