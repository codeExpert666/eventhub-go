# Go 版 EventHub 工程纪律 ADR

## 标题
Go 版 EventHub 采用设计优先、Java-Go parity 和 Go 质量门禁作为工程纪律

## 状态
- accepted

## 背景
Go 版 EventHub 的目标是复刻 Java 版 EventHub 的业务语义、接口契约、错误码、数据库模型、测试策略和文档沉淀方式。

如果只按功能点直接写 Go 代码，后续容易出现几个问题：

- API 字段、错误码或状态语义与 Java 版偏离，但没有记录原因。
- handler 直接访问数据库，导致业务规则散落在 HTTP 层。
- sqlc 生成代码被当作业务模型使用，repository 边界不清。
- JWT 中塞入角色、邮箱、用户名或用户状态，导致动态权限语义陈旧。
- 每次实现后缺少设计和验证记录，学习型项目的复盘价值下降。

因此需要在业务代码开始前先建立工程纪律。

## 决策
Go 版 EventHub 采用以下工程纪律作为长期基线：

- 非微小修改必须先写或更新 `docs/ai/design/` 下的设计文档。
- 实现后必须写或更新 `docs/ai/implementation/` 下的 implementation note。
- 出现关键技术取舍时必须写或更新 `docs/ai/adr/` 下的 ADR。
- Java-Go 业务语义、接口契约、错误码、数据库模型和测试策略差异必须记录在设计文档或 `docs/ai/parity/java-go-parity-matrix.md`。
- `AGENTS.md` 必须显式说明 parity matrix 的触发条件、记录字段和无需更新时的说明要求。
- `.agents/skills/backend-design-first/SKILL.md` 必须把 parity 文档维护作为独立工作流步骤，而不是只作为设计或验证中的附带检查项。
- Go 代码遵守 `handler -> service -> repository -> sqlc/database` 分层。
- 业务错误使用显式错误和错误码映射，不使用 `panic`。
- JWT 不写入角色、邮箱、用户名、用户状态等动态属性。
- 每次完成后运行当前可行的质量门禁：`gofmt`、`go test ./...`、`go vet ./...`、可选 `golangci-lint run`、`sqlc generate`、migration 测试、OpenAPI validate。

## 备选方案
- 方案 1：直接逐功能实现 Go 代码，后续再补文档。
- 方案 2：逐行或逐类翻译 Java/Spring 结构。
- 方案 3：只依赖 `AGENTS.md`，不维护模板、ADR 和 parity matrix。

## 决策理由
选择当前方案的原因：

- Java 版已经通过设计文档、implementation note 和 ADR 形成了可复盘的开发方式，Go 版应复用这种沉淀方式。
- Go 版不应复制 Spring 分层和 Java 类型风格，但必须保留业务语义、错误码和接口契约，因此需要 parity matrix 记录对齐状态。
- `handler -> service -> repository -> sqlc/database` 更符合 Go 项目中 HTTP、业务、持久化语义和生成代码的边界。
- JWT claim 边界需要在早期固定，避免后续认证模块为了方便把动态权限信息放进 token。
- 质量门禁写入规则后，后续可以自然落到 Makefile、CI 或本地脚本。

## 影响
- 好处：
  - 后续业务迁移能持续解释“为什么这样用 Go 实现”。
  - 设计、实现和 ADR 文档与 Java 版保持同构，便于双端对照。
  - 分层边界和 JWT claim 禁区提前固化，降低后续返工风险。
  - 质量门禁明确，方便在 Go 工程初始化后接入 CI。
- 代价：
  - 每次非微小修改都需要额外文档工作。
  - 初期在没有 Go module、sqlc、migration 和 OpenAPI 工具前，部分门禁只能记录为暂不可运行。
  - parity matrix 需要持续维护，否则会逐渐失去审查价值。
- 后续可能需要调整的地方：
  - Go 工程初始化后，应把质量门禁落成 Makefile 或 CI。
  - 选定 migration、OpenAPI 和 lint 工具后，应回填模板中的具体命令。
  - 当业务模块增多后，可以把 parity matrix 拆分为 auth、user、event、order、payment 等子矩阵。
