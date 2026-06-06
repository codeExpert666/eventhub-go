# 文档命名规则对齐实现说明

## 1. 本次改动解决了什么问题

本次改动解决 Go 版文档命名规则与实际沉淀结果不一致的问题。

当前 `docs/ai/design/` 和 `docs/ai/implementation/` 已经使用 `000-` 到 `016-` 的三位序号前缀，`docs/ai/adr/` 使用 `0001-` 到 `0022-` 的四位序号前缀；但 `docs/ai/README.md` 和 backend-design-first skill 仍建议使用 `YYYY-MM-DD-...` 日期前缀。这个不一致会让后续新增文档时产生误导。

## 2. 改动内容
- 新增设计文档：
  - `docs/ai/design/017-doc-naming-rule-alignment.md`
- 新增本实现说明：
  - `docs/ai/implementation/017-doc-naming-rule-alignment.md`
- 修改 `docs/ai/README.md`：
  - 将“命名建议”调整为“命名规则”。
  - 明确 design / implementation 使用 `NNN-<topic>.md`。
  - 明确同一次非微小修改的 design 与 implementation 共享同一个三位序号和主题名。
  - 明确 implementation-only 使用 `implementation-only-NNN-<topic>.md` 独立命名空间。
  - 明确 ADR 使用独立四位序号 `NNNN-<decision-topic>.md`。
  - 明确 parity 文档使用稳定索引名，不强制套用迭代序号。
- 修改 `.agents/skills/backend-design-first/SKILL.md`：
  - Step 4 的设计文档文件名规则改为三位序号。
  - Step 7 的 implementation note 文件名规则改为复用对应 design 的三位序号。
  - Step 7 明确 implementation-only 不使用常规三位序号，而是使用 `implementation-only-NNN-<topic>.md`。
  - Step 9 增加 ADR 四位序号命名规则。
- 更新 Java-Go parity 记录：
  - `docs/ai/parity/java-go-parity-matrix.md` 的“协作与文档基线”行已补充本次 017 文档索引。
  - 说明 Go 版文档命名按实际沉淀方式采用序号规则，并记录 implementation-only 的独立命名空间，不逐字迁移 Java 历史日期前缀。

## 3. 为什么这样设计
- 当前仓库事实已经形成序号命名：design / implementation 共享三位序号能直观看出同一次改动的设计与实现说明对应关系。
- implementation-only 没有对应 design，如果继续使用常规三位序号会让下一次 design 按 `docs/ai/design/` 最大序号取号时撞号；独立命名空间可以保留例外，同时保护常规配对编号。
- ADR 使用四位序号与现有 ADR 文件一致，也能表达决策记录独立于普通迭代文档。
- 文件所在目录已经表达文档类型，因此 design / implementation 文件名不再追加 `-design` 或 `-implementation`，可以减少重复信息。
- Go 版保留 Java 版“文档沉淀、设计优先、ADR 记录取舍”的语义，但不逐字迁移 Java 日期命名方式。

## 4. 替代方案
- 方案 A：继续保留日期命名建议，把现有序号文档视为初始化例外。
  - 没有采用。当前序号文档已经覆盖 000-016 和 ADR 0001-0022，序号命名不是临时例外。
- 方案 B：批量把 Go 版已有文档重命名为日期前缀。
  - 没有采用。这样会破坏既有链接、parity 索引和历史引用，收益低。
- 方案 C：使用日期加序号的混合命名。
  - 没有采用。命名更长，也削弱 design / implementation 同序号对应关系的可读性。
- 方案 D：implementation-only 继续使用常规 `NNN-<topic>.md`。
  - 没有采用。它会消耗常规配对编号，并可能让下一次 design / implementation 配对产生编号碰撞。

## 5. 测试与验证
- 规则文本检索：
  - `rg -n 'YYYY-MM-DD|Suggested filename|日期快照' .agents/skills/backend-design-first/SKILL.md docs/ai/README.md`
  - 结果：无匹配，活动规则中已不再保留旧日期命名建议。
- implementation-only 命名空间检索：
  - `rg -n 'next available zero-padded three-digit|use the next available zero-padded|目录中已有最大序号为准' .agents/skills/backend-design-first/SKILL.md docs/ai/README.md docs/ai/design/017-doc-naming-rule-alignment.md`
  - 结果：无匹配，活动规则中已不再要求 implementation-only 使用常规三位序号。
- Go 测试：
  - `go test ./...`
  - 结果：通过。
- Go vet：
  - `go vet ./...`
  - 结果：通过。
- lint：
  - `golangci-lint run`：本机未安装 `golangci-lint`。
  - `make lint`：通过；Makefile 使用 Docker fallback 运行固定版本 lint。
- 手工验证：
  - 检查现有 `docs/ai/design/`、`docs/ai/implementation/` 和 `docs/ai/adr/` 文件名，确认规则与实际一致。
  - 对照 Java 版 `docs/ai` 文件名，确认 Java 仍为日期前缀，Go 版序号规则是有意差异。
- 不适用验证：
  - `gofmt`：不适用，本次未修改 Go 文件。
  - `sqlc generate`：不适用，本次未修改 SQL 或 sqlc 配置。
  - migration 测试：不适用，本次未修改 migration。
  - OpenAPI validate：不适用，本次未修改 API 契约。

## 6. 已知限制
- 当前仍需要人工查看对应命名空间中的最大序号来决定下一个文档编号。
- 如果多人并行新增文档，常规配对命名空间或 implementation-only 命名空间内仍可能出现序号冲突，需要在合并前按实际落盘顺序调整。
- 本次没有新增脚本自动生成文档编号；后续如果文档数量继续增长，可以考虑增加轻量检查脚本。

## 7. 对后续版本的影响
- 后续非微小修改可以直接按 `NNN-<topic>.md` 生成 design 和 implementation note，减少命名分歧。
- 后续被明确标记为 implementation-only 的说明使用 `implementation-only-NNN-<topic>.md`，不会占用常规配对编号。
- 后续 ADR 继续按 `NNNN-<decision-topic>.md` 递增，独立记录关键技术取舍。
- parity matrix 继续作为索引，不被文档编号规则绑死；专题 parity 文档可以保持稳定名称。
