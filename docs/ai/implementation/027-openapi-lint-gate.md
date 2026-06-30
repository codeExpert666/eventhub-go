# OpenAPI lint 门禁实现说明

## 1. 本次改动解决了什么问题

本次为 Go 版 EventHub 补充通用 OpenAPI lint 门禁。改动前仓库已经有：

- `make openapi-validate`：用 kin-openapi 校验 OpenAPI 结构和引用。
- `make openapi-check`：执行 validate、oapi-codegen generate 和 generated file diff。
- Go policy test：用 `go test ./...` 固化统一响应 envelope、错误响应集中引用、RBAC 文档元数据、router/spec 对齐和真实响应契约。

这些门禁能覆盖“结构合法”和“团队强约束”，但对 operationId、tags、summary、schema example、unused components 等通用 OpenAPI 文档质量问题缺少专门检查。本次新增 Redocly CLI lint，让 API 文档质量检查进入 Makefile 和 CI。

## 2. 改动内容
- 新增了什么
  - 新增 `redocly.yaml`，从 `minimal` 规则集起步，并显式配置首批温和规则。
  - 新增 Makefile 变量 `REDOCLY_CLI_VERSION`、`OPENAPI_LINT_CONFIG`。
  - 新增 `make openapi-lint`，通过 `npx --yes @redocly/cli@$(REDOCLY_CLI_VERSION)` 执行固定版本 Redocly CLI。
  - 新增 ADR：`docs/ai/adr/0023-openapi-lint-tooling.md`。
  - 新增设计文档：`docs/ai/design/027-openapi-lint-gate.md`。
- 修改了什么
  - 修改 `.github/workflows/ci.yml`，在 generated-contract job 中设置 Node 24 并额外执行 `make openapi-lint`。
  - 修改 `api/openapi/eventhub.yaml`，为 `ApiResponse.data` 和 `ApiResponseVoid.data` 补充符合 OpenAPI lint 的 nullable object schema。
  - 重新生成 `api/openapi/gen/eventhub.gen.go`，同步 `ApiResponse.Data` 和 `ApiResponseVoid.Data` 的 generated 类型。
  - 修改 `README.md`，增加 `make openapi-lint` 和 OpenAPI lint/validate/check/policy test 分工说明。
  - 修改 `docs/ai/parity/java-go-parity-matrix.md`，在 OpenAPI / Swagger 与质量门禁行索引 Redocly lint、CI step 和 ADR-0023。
- 删除了什么
  - 未删除现有 OpenAPI validate、policy test 或 generated check。
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次改变 Go spec-first OpenAPI 文档质量门禁和 CI 验证策略。

## 3. 为什么这样设计
- 关键设计原因
  - Redocly CLI 对 OpenAPI 文档质量规则开箱即用，适合当前“温和起步、低维护”的目标。
  - 仓库没有 Node 项目结构，因此使用 npx 固定版本，避免引入 `package.json`、lockfile 或前端构建链。
  - `openapi-lint` 独立于 `openapi-check`，让文档质量失败和生成物漂移失败更容易定位。
  - Redocly recommended 默认会启用 `security-defined`、`operation-4xx-response` 等规则；这些规则与当前 Go policy 的刻意设计存在冲突，因此本次从 `minimal` 规则集开始，逐步启用必要规则。
- 与 Go 项目当前阶段的匹配点
  - 延续 Makefile 中低频工具使用固定版本命令执行的策略。
  - CI 继续复用 Makefile target，不在 workflow 中复制复杂命令。
  - 不改变 handler/service/repository/sqlc/database 分层。
- 与 Java 版业务语义的对齐方式
  - Java 版 Springdoc 注解中的 operation/tag/summary/description 质量要求，在 Go 版通过 spec-first YAML、Redocly lint 和 Go policy test 共同承接。
  - Go 版不迁移 Springdoc 注解扫描机制，仍以 `api/openapi/eventhub.yaml` 作为契约源。

## 4. 替代方案
- 方案 A：使用 Spectral。
  - 未采用原因：Spectral 自定义能力强，但本次只需要通用 OpenAPI 文档质量 lint。Redocly 配置更轻，初期维护成本更低。
- 方案 B：引入 `package.json` 和 lockfile。
  - 未采用原因：当前 Go 后端仓库没有 Node 工具链，npx 固定版本已经能满足 CI 可运行和版本可控。
- 方案 C：把 `openapi-lint` 纳入 `openapi-check`。
  - 未采用原因：`openapi-check` 当前语义是 validate、generate 和 generated diff。lint 独立执行能保持失败定位清楚；CI 已额外执行 `make openapi-lint`，不降低覆盖。
- 方案 D：只依赖 Go policy test。
  - 未采用原因：Go policy test 更适合团队业务强约束，不适合替代通用 OpenAPI 风格与 schema example 检查。

## 5. 测试与验证
- 跑了哪些测试
  - RED：使用计划中的 Redocly 规则检查当前 spec，失败于 `nullable-type-sibling: 2` 和 `no-invalid-schema-examples: 1`。
  - RED：新增正式 `make openapi-lint` 后首次运行失败，错误集中在 `ApiResponse.data` 和 `ApiResponseVoid.data`。
  - GREEN：修正 schema 后 `make openapi-lint` 通过。
  - REGRESSION：第一次修正时去掉了 `ApiResponse.data` 的 nullable，`go test ./...` 中真实响应契约测试失败，证明 `ErrorResponse` 通过 allOf 复用 base schema 时仍需要 base data nullable。
  - GREEN：把 `ApiResponse.data` 修正为 nullable object schema 后，`go test ./...` 通过。
- 跑了哪些质量门禁
  - `make openapi-lint`：通过。
  - `make openapi-check`：最终通过；中间曾按预期失败于 generated file diff，用于确认 `api/openapi/gen/eventhub.gen.go` 需要纳入本次改动。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `git diff --check`：通过。
- 手工验证了哪些场景
  - 对照 Redocly 默认 recommended 输出，确认不能直接启用完整 recommended，否则会因为本仓库的安全策略、localhost server、actuator 4xx 等约定产生无关失败。
  - 对照 generated diff，确认 `ApiResponse.Data` 和 `ApiResponseVoid.Data` 的变化来自 schema 修正和 oapi-codegen，而非手工编辑 generated code。
- Java-Go parity 如何验证
  - 已更新 parity matrix 的 OpenAPI / Swagger 行，补充 Redocly CLI 对 Springdoc operation/tag/summary 质量语义的 Go 端承接方式。
  - 已更新质量门禁行，记录 CI 额外执行 `make openapi-lint`。
- 结果如何
  - OpenAPI lint、Go policy test、OpenAPI validate/generate drift gate 和 Go 测试均可运行。

## 6. 已知限制
- 当前版本还缺什么
  - `operation-description` 初始为 warn，不作为阻断项。
  - `no-unused-components` 初始为 warn，不作为阻断项。
  - Redocly lint 不理解 EventHub 的业务 envelope、RBAC 和 router/spec 对齐规则，这些仍由 Go policy test 负责。
- 哪些地方后面需要继续演进
  - 当 API 文档稳定后，可将 `operation-description` 和 `no-unused-components` 提升为 error。
  - 后续 event/order/payment API 增加后，可评估是否加入 pagination、idempotency header、错误码枚举等更细规则。
  - 如果 OpenAPI lint 规则大量自定义，再评估 Spectral 或 package lockfile。
- 与 Java 版仍有哪些差距
  - Java 版仍由 Springdoc 注解生成文档；Go 版继续手写 spec-first YAML，不迁移注解扫描。
  - Go 版使用 Redocly vendor-neutral lint，这是 Go spec-first 的工程增强，不是 Java 版逐项能力迁移。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - OpenAPI 文档质量进入 CI，API 契约更适合作为前后端协作和 SDK 生成基础。
- 对微服务 / 云原生演进的影响
  - 当 API 网关、SDK、服务拆分或外部集成出现时，operationId、tags 和 schema 质量会更关键；lint 门禁可以提前降低契约治理成本。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续修改 `api/openapi/eventhub.yaml` 时，需要同时运行 `make openapi-lint`、`make openapi-check` 和 `go test ./...`。
  - OpenAPI 通用质量问题优先放在 `redocly.yaml`；团队业务强约束继续放在 Go policy test，避免把两类规则混在一起。
