# AI 协作文档目录说明

这个目录不是为了堆文档，而是为了把 Go 版 EventHub 对齐 Java 版时的设计、实现和取舍固化下来。

## 推荐子目录

- `design/`
  - 某个需求开始前的设计说明
- `implementation/`
  - 某次实现后的说明
- `adr/`
  - 关键技术决策记录
- `parity/`
  - Java-Go 业务语义、接口契约、错误码、数据库模型和测试策略对齐记录

## 命名规则

Go 版 `docs/ai` 文档以序号表达迁移顺序和决策顺序，不使用日期前缀。

设计文档和实现说明统一使用三位递增序号：

```text
docs/ai/design/NNN-主题.md
docs/ai/implementation/NNN-主题.md
```

规则：

- 常规 design / implementation 配对文档的 `NNN` 从 `000` 开始，按创建顺序递增，创建前以 `docs/ai/design/` 中已有常规三位序号的最大值为准。
- 同一次非微小修改的 design 和 implementation 使用同一个 `NNN` 与同一个主题名。
- 文件所在目录已经表达文档类型，文件名不额外追加 `-design` 或 `-implementation`。
- 主题使用简短 kebab-case，优先英文，例如 `017-doc-naming-rule-alignment.md`。

少数被明确标记为 implementation-only、没有对应 design 的实现说明使用独立命名空间：

```text
docs/ai/implementation/implementation-only-NNN-主题.md
```

implementation-only 的 `NNN` 从 `000` 开始，只参考已有 `implementation-only-NNN-*.md` 文件递增，不占用常规 design / implementation 配对编号。

ADR 使用四位递增序号，独立于 design / implementation 序号：

```text
docs/ai/adr/NNNN-主题.md
```

例如：

- `docs/ai/design/017-doc-naming-rule-alignment.md`
- `docs/ai/implementation/017-doc-naming-rule-alignment.md`
- `docs/ai/implementation/implementation-only-000-example.md`
- `docs/ai/adr/0023-example-decision.md`

Parity 文档以稳定索引名为主，例如 `docs/ai/parity/java-go-parity-matrix.md`。专题契约文档可以使用清晰、稳定的名称，不强制套用迭代序号。

## 写作原则

1. 文档服务于后续复盘、面试表达和 Java-Go parity 审查。
2. 设计文档要讲清楚为什么这样迁移，而不是只列任务。
3. 实现文档要讲清楚怎么做、验证了什么、还没做什么。
4. ADR 要讲清楚取舍与长期影响。
5. Parity 文档要讲清楚 Java 来源、Go 目标、当前状态和差异原因。
6. 非微小修改必须更新本目录下对应文档。

## 目录结构与文档联动

- 目录结构变化属于非微小修改。
- 移动 package、拆分层次、引入 repository/sqlc/openapi/migrations，都必须更新设计文档和 implementation note。
- 长期结构规范以 `AGENTS.md` 为准。
- 关键 package layout 决策以 ADR 为准。
- parity matrix 要记录 Java 分层到 Go 目录的映射。

## HTTP 传输模型与 VO 规范

- OpenAPI strict server 已生成且适用于本项目的 HTTP request/response 结构体，handler 直接使用 `api/openapi/gen` 类型。
- `internal/http/dto` 仅用于 generated model 不适用或非 OpenAPI HTTP 面；新增时必须在 design / implementation note 中说明原因和生命周期。
- 本项目不设置 `internal/http/vo`。
- `internal/http/response` 只维护统一响应元数据和错误写出能力，例如 `SuccessMeta`、`ErrorBody`、`WriteError`；成功 envelope 使用 OpenAPI generated typed response。
- DDD Value Object 放 `internal/domain/<domain>` 或 `internal/domain/common`。
- 涉及 DTO 边界调整时，属于非微小修改，必须更新 design / implementation note / parity matrix。
- 若引入例外，必须写 ADR。

## Service Contract 规范

- `internal/service/<domain>/service.go` 只放 `Service`、constructor 和依赖字段。
- service 写操作输入放 `command.go`，命名为 `XxxCommand`。
- service 读/列表/搜索/详情输入放 `query.go`，命名为 `XxxQuery`；没有 Query 时不要创建空文件。
- service 输出放 `result.go`，命名为 `XxxResult` 或窄范围内部结果类型。
- 业务方法按 use case 拆到 `register.go`、`login.go`、`create_event.go` 等文件。
- Command / Query / Result 不带 HTTP `json` tag，不使用 `XxxRequest`、`XxxResponse`、`XxxDTO`、`XxxVO`、`XxxResp` 后缀。
- service 不依赖 `internal/http/dto` 或 `api/openapi/gen`，不暴露 sqlc generated model。
- 涉及 service contract 边界调整时，属于非微小修改，必须更新 design / implementation note / parity matrix；架构性例外需要更新 ADR。

## Parity 文档约定

Parity 文档不是设计文档、实现说明或 ADR 的替代品，而是 Java-Go 对齐状态的索引和台账。

当前统一维护：

- `docs/ai/parity/java-go-parity-matrix.md`

以下情况必须检查并按需更新 parity matrix：

- API 路径、方法、请求字段、响应字段、状态码、分页语义或 OpenAPI 契约变化。
- 错误码、错误消息、校验行为或业务失败语义变化。
- 数据库表、字段、索引、唯一约束、枚举/状态值、migration、sqlc query 或 repository 行为变化。
- 业务流程、状态机、并发、幂等、缓存或事务边界变化。
- 认证、授权、JWT claim、auth session、refresh token 或安全边界变化。
- 测试策略、测试夹具、Java 测试对齐、migration 测试或契约测试变化。
- 为了使用 Go 自然写法而刻意偏离 Java 实现结构，但仍保持业务语义一致。

每条 parity 记录至少说明：

- Java 来源或文档引用。
- Go 目标文件、package 或文档。
- 当前状态，例如 `已对齐`、`规则已初始化`、`待迁移`、`待决策`、`不适用`。
- 刻意差异的简短原因。
- 对应设计文档、implementation note 或 ADR 链接；如果细节已经在其他文档中展开，matrix 中只保留索引和摘要。

如果某次非微小修改不需要更新 parity matrix，需要在 implementation note 或最终总结中说明原因。

## 模板约定

写文档时优先参考：

- 设计文档：`docs/templates/design-template.md`
- 实现说明：`docs/templates/implementation-note-template.md`
- ADR：`docs/templates/adr-template.md`

除非当前任务确实不适用，不要随意改变模板大纲结构。需要调整时，在具体文档中说明原因。

Parity matrix 当前不单独使用模板；以 `docs/ai/parity/java-go-parity-matrix.md` 的表格结构和状态说明为准。

## Go 版质量门禁

实现类改动完成后，根据改动范围运行可行命令：

- `gofmt`
- `go test ./...`
- `go vet ./...`
- `golangci-lint run`，如果已配置或工具可用
- `sqlc generate`，如果 SQL 或 sqlc 配置有变化
- migration 测试，如果数据库迁移有变化
- OpenAPI validate，如果 API 契约有变化
- `make test`，如果仓库提供 Makefile

如果当前仓库阶段尚不支持某项命令，需要在 implementation note 和最终总结中说明原因。
