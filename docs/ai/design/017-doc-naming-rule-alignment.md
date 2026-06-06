# 文档命名规则对齐设计

## 1. 背景
- 当前 Go 版 `docs/ai/design/` 和 `docs/ai/implementation/` 的实际文件名已经采用 `000-` 到 `016-` 的三位递增序号前缀。
- 当前 Go 版 `docs/ai/adr/` 的实际文件名采用 `0001-` 到 `0022-` 的四位递增序号前缀。
- `docs/ai/README.md` 和 `.agents/skills/backend-design-first/SKILL.md` 仍建议使用 `YYYY-MM-DD-...` 日期命名，和当前仓库实际沉淀方式不一致。
- Java 版 EventHub 历史文档仍采用日期前缀；Go 版已在初始化阶段选择序号前缀，用于表达迁移阶段和决策顺序。

## 2. 目标
- 将 Go 版文档命名规则调整为和现有文件实际情况一致。
- 明确 design / implementation 文档使用三位递增序号，且同一次非微小修改的 design 与 implementation 共享同一个序号。
- 明确 implementation-only 实现说明使用独立命名空间，不占用 design / implementation 配对序号。
- 明确 ADR 使用四位递增序号，独立于 design / implementation 迭代序号。
- 明确 parity 文档以稳定索引名为主，不强制套用迭代序号。

## 3. 非目标
- 不批量重命名既有文档。
- 不改变 Java 版文档命名方式。
- 不修改设计模板、implementation note 模板和 ADR 模板的小节结构。
- 不改变代码、API、数据库、错误码、JWT 或测试契约。

## 4. 影响范围
- 涉及 Go 文档与协作规则：
  - `docs/ai/README.md`
  - `.agents/skills/backend-design-first/SKILL.md`
  - `docs/ai/parity/java-go-parity-matrix.md`
  - `docs/ai/design/017-doc-naming-rule-alignment.md`
  - `docs/ai/implementation/017-doc-naming-rule-alignment.md`
- 不涉及 Go package、HTTP API、数据库 migration、sqlc、OpenAPI 或运行时代码。
- 影响 `docs/ai/parity/java-go-parity-matrix.md`：需要更新“协作与文档基线”行，记录 Go 版文档命名规则已从日期建议收敛为序号规则。

## 5. 领域建模
- 本次没有业务领域对象。
- 文档类型可以视为协作流程对象：
  - design：需求实现前的设计说明。
  - implementation：实现后的说明和验证记录。
  - ADR：关键技术决策记录。
  - parity：Java-Go 对齐索引和台账。
- 与 Java 版的关系：
  - Java 版历史文档使用日期前缀。
  - Go 版继续对齐 Java 的文档沉淀意图，但用序号表达迁移顺序和同一迭代内的设计/实现关联。

## 6. API 设计
- 本次不涉及 HTTP API。
- 不新增请求参数、响应结构、错误码或 OpenAPI 契约。
- 文档规则更新如下：
  - design：`NNN-<topic>.md`，例如 `017-doc-naming-rule-alignment.md`。
  - implementation：`NNN-<topic>.md`，与对应 design 共享 `NNN`。
  - implementation-only：`implementation-only-NNN-<topic>.md`，没有对应 design 时使用，独立递增。
  - ADR：`NNNN-<decision-topic>.md`，例如 `0023-example-decision.md`。
  - parity：以稳定索引名为主，例如 `java-go-parity-matrix.md`；专题契约文档可使用清晰的稳定名称。

## 7. 数据设计
- 本次不涉及表结构、索引、唯一约束、migration 或 sqlc query。
- 不需要运行 `sqlc generate` 或 migration 测试。

## 8. 关键流程
- 正常流程：
  1. 开始非微小修改前，先确定本次 design / implementation 的下一个三位序号。
  2. 在 `docs/ai/design/` 新增或更新 `NNN-<topic>.md`。
  3. 实现后在 `docs/ai/implementation/` 新增或更新同序号 `NNN-<topic>.md`。
  4. 如有关键技术取舍，再在 `docs/ai/adr/` 使用下一个四位 ADR 序号新增记录。
  5. 按触发条件更新 parity matrix 或在 implementation note 中说明不需要更新。
- 异常流程：
  - 如果多个迭代并行产生同一个序号冲突，以已落盘文档为准，后创建者使用下一个可用序号。
  - 如果某次明确是 implementation-only 且没有对应 design，则在 `docs/ai/implementation/` 使用 `implementation-only-NNN-<topic>.md`，其 `NNN` 只从已有 implementation-only 文件递增，不占用常规配对编号。
  - 如果某次只有 ADR，没有对应 design / implementation，则 ADR 仍使用独立四位序号。
- 分层影响：
  - 不涉及 handler / service / repository / sqlc/database。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及运行时并发、幂等或缓存。
- 文档序号冲突的风险通过“创建前查看对应命名空间中的最大序号，使用下一个可用序号”降低。

## 10. 权限与安全
- 本次不涉及认证、授权、JWT claim、敏感信息或操作日志。

## 11. 测试策略
- 单元测试：不适用，本次不修改 Go 代码。
- service / repository 测试：不适用。
- migration / sqlc 验证：不适用。
- 接口验证和 OpenAPI validate：不适用，本次不修改 API 契约。
- Java-Go parity 验证：
  - 对照 Java 文档实际日期命名，确认 Go 版是有意采用序号命名。
  - 更新 parity matrix 的“协作与文档基线”索引。
- 需要运行的命令：
  - `rg` 检查是否仍存在日期命名建议。
  - `go test ./...` 和 `go vet ./...` 如仓库可运行，用于确认文档改动没有破坏 Go module 基线。

## 12. 风险与替代方案
- 当前方案的风险：
  - Go 版文档命名与 Java 版历史日期命名不同，需要在规则中写清楚这是刻意差异。
  - 常规配对文档和 implementation-only 文档各自要求创建前查看对应命名空间的最大序号，否则可能产生冲突。
- 备选方案：
  - 方案 A：继续使用日期命名建议，只把现有文档视为初始化例外。
  - 方案 B：批量重命名现有 Go 文档为日期前缀。
  - 方案 C：使用日期加序号的混合前缀。
  - 方案 D：让 implementation-only 继续使用常规三位序号。
- 为什么不选备选方案：
  - 不选方案 A：当前已有 000-016 和 0001-0022 文档，序号命名已经不是初始化例外。
  - 不选方案 B：会制造大量历史链接和 parity 索引变更，收益低。
  - 不选方案 C：命名更长，且不能解决 design / implementation 同序号关联的可读性问题。
  - 不选方案 D：implementation-only 会占用常规配对编号，后续 design 文档按 design 目录取号时可能产生编号碰撞。
- 后续可演进点：
  - 如果未来引入文档生成或索引脚本，可自动检测下一个可用序号。
