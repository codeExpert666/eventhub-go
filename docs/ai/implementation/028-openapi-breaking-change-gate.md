# OpenAPI breaking change 门禁实现说明

## 1. 本次改动解决了什么问题

本次为 Go 版 EventHub 增加 OpenAPI breaking change 检测，防止 PR 无意破坏 `/api/v1` API 兼容性。

改动前仓库已经有：

- `make openapi-lint`：Redocly 通用文档质量检查。
- `make openapi-check`：OpenAPI validate、oapi-codegen generate 和 generated diff。
- `go test ./...`：项目自定义 OpenAPI policy test、router/spec 对齐和真实响应契约测试。

这些检查都只验证当前工作区内部的一致性，不能判断当前 PR 相比 base branch 是否删除或收紧了已存在的 v1 API 契约。本次新增 oasdiff 门禁后，本地和 CI 都能比较 base spec 与 revision spec，并在发现 `/api/v1/**` breaking changes 时失败。

## 2. 改动内容
- 新增了什么
  - 新增 `Makefile` 变量 `OASDIFF_VERSION`、`OPENAPI_BASE_REF`、`OPENAPI_BREAKING_MATCH_PATH`。
  - 新增 `make openapi-breaking-check`。
  - 新增 GitHub Actions `openapi-breaking` job，仅在 pull request 事件运行。
  - 新增设计文档 `docs/ai/design/028-openapi-breaking-change-gate.md`。
  - 新增 ADR `docs/ai/adr/0024-openapi-v1-compatibility-policy.md`。
  - 新增本实现说明。
- 修改了什么
  - 修改 `.github/workflows/ci.yml`，在 PR 中 checkout 当前代码、setup Go、fetch base branch，并执行 `make openapi-breaking-check OPENAPI_BASE_REF=origin/${{ github.base_ref }}`。
  - 修改 `README.md`，补充 `make openapi-breaking-check` 命令和 OpenAPI 门禁分工。
  - 修改 `docs/ai/parity/java-go-parity-matrix.md`，记录 oasdiff v1 兼容性门禁和 PR job。
- 删除了什么
  - 未删除现有 `openapi-lint`、`openapi-check`、generated check 或 Go policy test。
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次改变 Go 端 OpenAPI 契约治理和 CI 验证策略。

## 3. 为什么这样设计
- 关键设计原因
  - oasdiff 专注 OpenAPI diff 与 breaking change 分类，适合比较 base branch 与当前 PR 的 `eventhub.yaml`。
  - `go run github.com/oasdiff/oasdiff@$(OASDIFF_VERSION)` 延续仓库低频 Go 工具固定版本策略，不要求开发者预装 CLI。
  - `openapi-breaking-check` 依赖 base ref，独立于 `openapi-check` 可以保持失败原因清晰。
  - 本地 target 在 `origin/main` 或 base spec 不存在时返回失败并提示 fetch，避免把未实际比较的场景误报为成功。
  - 首版只匹配 `^/api/v1($|/)`，聚焦稳定业务 API 兼容性。
- 与 Go 项目当前阶段的匹配点
  - 不改 handler、service、repository、sqlc/database 分层。
  - 不改 OpenAPI 契约内容和 generated code。
  - CI 继续复用 Makefile target，而不是在 workflow 中复制工具命令。
- 与 Java 版业务语义的对齐方式
  - Java 版通过 Controller、DTO、Springdoc 注解和测试表达接口契约；Go 版继续用 spec-first YAML 表达契约，并新增 oasdiff 作为 v1 兼容性保护。
  - Go 不迁移 Springdoc 运行时生成机制，保持当前 OpenAPI spec-first 方向。

## 4. 替代方案
- 方案 A：使用 Redocly 或 Spectral 自定义规则检测 breaking changes。
  - 未采用原因：它们更适合 lint / style guide；跨版本契约 diff 和 breaking 分类不是核心职责。
- 方案 B：把 breaking check 并入 `openapi-check`。
  - 未采用原因：`openapi-check` 当前语义是 validate、generate 和 generated diff。breaking check 需要 base ref，独立 target 更容易本地和 CI 排错。
- 方案 C：检查整个 OpenAPI spec。
  - 未采用原因：本次目标是 v1 业务 API 兼容性。actuator、Swagger 文档路由或未来其他非 v1 API 是否纳入，应单独决策。
- 方案 D：增加 ignore 文件支持。
  - 未采用原因：首版先避免静默豁免。确实需要 breaking change 时，应通过 deprecated、版本升级、人工批准或 `/api/v2` 处理，并用 ADR 或设计文档记录。

## 5. 测试与验证
- 跑了哪些测试
  - RED：实现前运行 `make openapi-breaking-check`，失败为 `No rule to make target 'openapi-breaking-check'`。
  - GREEN：实现后运行 `make openapi-breaking-check`，输出 `No changes detected`。
  - NEGATIVE：运行 `make openapi-breaking-check OPENAPI_BASE_REF=origin/__missing__`，按预期失败并提示 `OpenAPI breaking check requires base ref 'origin/__missing__'.`。
- 跑了哪些质量门禁
  - `make openapi-breaking-check`：通过。
  - `make openapi-check`：通过。
  - `make openapi-lint`：通过。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `make lint`：通过，`0 issues.`。
  - `git diff --check`：通过。
- 手工验证了哪些场景
  - 确认本地 target 默认比较 `origin/main:api/openapi/eventhub.yaml` 与当前工作区 `api/openapi/eventhub.yaml`。
  - 确认 CI job 只在 `pull_request` 运行，并显式 fetch `github.base_ref` 后传入 `OPENAPI_BASE_REF`。
- Java-Go parity 如何验证
  - 已更新 parity matrix 的 OpenAPI / Swagger 行，记录 oasdiff 对 `/api/v1` breaking changes 的保护。
  - 已更新质量门禁行，记录 PR 中新增 OpenAPI breaking change job。
- 结果如何
  - 新增 target 和 CI job 均已落地，现有 OpenAPI validate/generate、lint、Go test、vet 和 lint 门禁保持通过。

## 6. 已知限制
- 当前版本还缺什么
  - 尚未引入 ignore 文件；确实需要 breaking change 时需要设计文档或 ADR 记录，并通过人工流程处理。
  - 默认只检查 `/api/v1/**`，不覆盖 actuator、Swagger 文档路由或未来其他版本路径。
- 哪些地方后面需要继续演进
  - `/api/v2` 出现后，可为不同版本配置不同的 match path。
  - 稳定 API 增多后，可引入 deprecation-days 策略。
  - 如果团队需要批准制豁免，可新增 oasdiff ignore 文件，并要求每条豁免关联 ADR 或审批记录。
- 与 Java 版仍有哪些差距
  - Java 版没有迁移 oasdiff；这是 Go spec-first 契约治理增强。
  - oasdiff 只能识别 OpenAPI 契约层 breaking changes，不能替代业务语义 review。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - PR 级别的 API 兼容性保护更完整，能展示 spec-first、CI contract governance 和版本化 API 策略。
- 对微服务 / 云原生演进的影响
  - 后续 API 网关、SDK、前后端并行开发或服务拆分时，v1 breaking change 检测可以降低契约变更风险。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续修改 `api/openapi/eventhub.yaml` 时，除 `make openapi-lint`、`make openapi-check`、`go test ./...` 外，还应运行 `make openapi-breaking-check`。
  - CI PR 会自动比较 base branch 与 PR spec，因此 breaking change 需要在设计阶段提前处理版本策略。
