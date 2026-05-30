# Java-Go Parity Matrix

本文记录 Go 版 EventHub 与 Java 版 EventHub 在业务语义、接口契约、错误码、数据库模型、测试策略和文档沉淀方式上的对齐状态。

Java 版参考项目：

```text
/Users/xinnz/Library/Mobile Documents/com~apple~CloudDocs/Code/Java/eventhub
```

## 状态说明

- `已对齐`：Go 版已经建立对应规则、文档或实现。
- `规则已初始化`：Go 版已写入约束，但尚未有业务代码可验证。
- `待迁移`：Java 版已有能力，Go 版尚未实现。
- `待决策`：Go 版需要 ADR 或设计文档明确技术选择。
- `不适用`：Java 版实现细节不直接迁移到 Go。

## 对齐矩阵

| 领域 | Java 版来源 | Go 版目标 | 当前状态 | 说明 |
| --- | --- | --- | --- | --- |
| AI 协作规则 | `AGENTS.md` | `AGENTS.md` | 已对齐 | 保留设计优先、文档沉淀、固定 7 项总结，补充 Go 分层、质量门禁和 parity matrix 触发条件/记录字段。 |
| Agent skill | `.agents/skills/backend-design-first/SKILL.md` | `.agents/skills/backend-design-first/SKILL.md` | 已对齐 | 改写为 Go port、Java-Go parity、sqlc/database 和 Go 验证命令语境；已补充独立 parity 文档步骤，明确触发条件和记录字段。 |
| Codex 配置目录 | `.codex/config.toml` | `.codex/config.toml` | 已对齐 | Go 版沿用 Java 版最小项目级配置：`model`、reasoning effort、personality、开发期 approval policy 和 sandbox mode。 |
| 设计模板 | `docs/templates/design-template.md` | `docs/templates/design-template.md` | 已对齐 | 沿用 12 个小节，补充 Go package、sqlc、migration、OpenAPI validate 和 parity 验证。 |
| 实现说明模板 | `docs/templates/implementation-note-template.md` | `docs/templates/implementation-note-template.md` | 已对齐 | 沿用 7 个小节，补充 Go 质量门禁和 Java-Go parity。 |
| ADR 模板 | `docs/templates/adr-template.md` | `docs/templates/adr-template.md` | 已对齐 | 沿用 Java 版大纲，补充 Go 生态取舍说明。 |
| docs/ai 目录 | `docs/ai/design`、`implementation`、`adr` | 同名目录加 `parity` | 已对齐 | Go 版增加 `parity`，用于持续记录双端差异；README 已补齐 parity 的定位、触发条件、记录字段、状态值和与其他文档的关系。 |
| 工程纪律 ADR | 多份 Java ADR | `docs/ai/adr/0001-go-port-engineering-discipline.md` | 已对齐 | 明确 Go 版长期迁移纪律。 |
| 分层边界 | Java `controller / service / mapper / domain` | Go `handler -> service -> repository -> sqlc/database` | 规则已初始化 | 后续业务代码必须按此边界实现。 |
| 业务错误 | Java `BusinessException` / `ErrorCode` | Go 显式错误类型 / 错误码映射 | 待迁移 | 未来实现时对齐 Java 错误码和响应结构，不用 `panic` 表达业务错误。 |
| API 契约 | Java controller / OpenAPI / MockMvc 测试 | Go handler / OpenAPI / HTTP 测试 | 待迁移 | 后续每个 API 设计需对照 Java 路径、字段、状态码和错误码。 |
| 数据库模型 | Java migration / mapper / entity | Go migration / sqlc / repository | 待迁移 | 后续表、字段、索引、唯一约束和状态值需对齐 Java 版。 |
| JWT claim 边界 | Java auth ADR 和实现 | Go auth token 设计 | 规则已初始化 | Go 版禁止把角色、邮箱、用户名、用户状态写入 JWT。 |
| 测试策略 | Java unit / integration / MockMvc / H2 | Go unit / service / repository / API / migration 测试 | 规则已初始化 | 当前尚无 Go module，后续建立工程后执行 `go test ./...`、`go vet ./...`。 |
| 质量门禁 | Java Maven test、OpenAPI、profile 测试 | Go `gofmt`、`go test ./...`、`go vet ./...`、lint、sqlc、migration、OpenAPI validate | 规则已初始化 | 本次只落规则，工具选择和 CI 待后续工程初始化。 |
| Spring Boot / Maven | Java 基础工程 | Go module / Makefile / CI | 待决策 | 不直接迁移 Spring/Maven 结构，后续单独设计 Go 工程基础。 |
| MyBatis | Java mapper 持久化边界 | sqlc/database + repository | 待决策 | 已在规则中指定 sqlc/database 边界，具体 sqlc 配置待工程初始化。 |
| H2 测试 profile | Java test profile | Go migration / test database strategy | 待决策 | Go 版不默认采用 H2，需要在数据库测试设计中另行决策。 |

## 后续维护规则

1. 每迁移一个 Java 业务模块，必须新增或更新对应矩阵行。
2. 如果 Go 版刻意偏离 Java 版实现方式，但保持业务语义一致，需要在设计文档中说明。
3. 如果 Go 版无法保持接口或错误码兼容，必须新增 ADR 或在设计文档中写明理由。
4. 矩阵只记录对齐状态和索引，详细设计仍放在 `docs/ai/design/`、`docs/ai/implementation/` 和 `docs/ai/adr/`。
