# OpenAPI lint 门禁设计

## 1. 背景
- 当前 Go 版已经有 `make openapi-validate` 和 `make openapi-check`，其中 `openapi-check` 负责 OpenAPI 结构校验、oapi-codegen 生成和生成物漂移检查。
- `api/openapi/openapi_policy_test.go` 已经通过 Go test 固化团队强约束，例如统一响应 envelope、错误响应集中引用、admin role 元数据、router/spec 对齐和少量真实响应契约校验。
- 现有门禁缺少通用 OpenAPI 文档质量 lint，无法在 CI 中提前发现 operationId/tag/summary、schema example、未使用组件等风格和可维护性问题。
- Java 版通过 Springdoc 注解表达 operation、tag、summary/description 等文档元数据；Go 版采用 spec-first，需要把这类质量要求落在 YAML lint 与 policy test 上。

## 2. 目标
- 新增 `redocly.yaml`，用 Redocly CLI 为 `api/openapi/eventhub.yaml` 提供温和的通用 lint 规则。
- 新增 `make openapi-lint`，通过固定版本 `npx --yes @redocly/cli` 执行，不要求仓库引入 `package.json` 或 lockfile。
- CI 额外执行 `make openapi-lint`，让远端流水线覆盖 OpenAPI 文档质量。
- 保持 `openapi-check` 继续执行 validate、generate 和 generated diff。
- 修复 lint 暴露的当前 schema 问题，并同步 OpenAPI generated code。
- 在 implementation note 中说明 `openapi-validate`、Go policy test 和 `openapi-lint` 的职责边界。

## 3. 非目标
- 不引入 Node 项目结构、npm lockfile、前端构建流程或长期安装的本地 Node CLI。
- 不改 HTTP handler、service、repository、sqlc query、migration 或运行时鉴权逻辑。
- 不把 Redocly 默认 recommended 规则一次性全量接入，避免引入大量与当前仓库策略无关的失败。
- 不迁移 Java Springdoc 注解扫描机制；Go 版继续以 `eventhub.yaml` 为契约源。

## 4. 影响范围
- 新增 `redocly.yaml`。
- 修改 `Makefile`：增加 Redocly CLI 版本变量、配置变量和 `openapi-lint` target。
- 修改 `.github/workflows/ci.yml`：在 generated-contract job 中设置 Node 并执行 `make openapi-lint`。
- 修改 `api/openapi/eventhub.yaml`：修正 nullable schema 表达。
- 通过 `make openapi-check` 同步 `api/openapi/gen/eventhub.gen.go`。
- 更新 `README.md`、`docs/ai/implementation/027-openapi-lint-gate.md`、`docs/ai/adr/0023-openapi-lint-tooling.md` 和 `docs/ai/parity/java-go-parity-matrix.md`。
- 影响 parity matrix。原因是本次改变 OpenAPI 契约质量门禁和 CI 验证策略。

## 5. 领域建模
- 本次不新增业务领域实体。
- 质量门禁模型如下：
  - `OpenAPI spec`：`api/openapi/eventhub.yaml`，业务接口契约源。
  - `validate`：OpenAPI 标准结构和引用合法性检查。
  - `policy test`：EventHub 团队强约束，运行在 `go test ./...` 中。
  - `lint`：通用 OpenAPI 文档质量和风格检查。
- 与 Java 版的对应关系：
  - Java `@Operation` / `@Tag` 的文档元数据在 Go 版由 OpenAPI YAML 字段和 Redocly lint 共同约束。
  - Java Springdoc 生成能力不直接迁移；Go 版继续维护 spec-first 契约。

## 6. API 设计
- 不新增或修改 HTTP API 路径、方法、请求字段、响应字段、状态码或错误码。
- OpenAPI schema 修正：
  - `ApiResponse.data` 去掉无类型的 `nullable: true`，保留为通用 data 插槽，由具体 `ApiResponseXxx` schema 约束实际 data 类型。
  - `ApiResponseVoid.data` 明确为 `type: object`、`nullable: true`、`additionalProperties: true`，用于表达纯操作响应中的 `data: null`。
- 与 Java 版 OpenAPI/controller 契约的差异：
  - Go 版新增 Redocly lint 作为 spec-first 文档质量门禁；Java 版仍依赖 Springdoc 注解和测试。
  - 不改变 Go 版 OpenAPI 文档路径和生产环境关闭策略。

## 7. 数据设计
- 不调整表结构、索引、唯一约束或 migration。
- 不新增 sqlc query。
- `api/openapi/gen/eventhub.gen.go` 会因 schema 修正重新生成，属于 OpenAPI generated model 同步，不改变数据库模型。
- 数据一致性不受影响。

## 8. 关键流程
- 本地 lint 流程：
  1. 开发者修改 `api/openapi/eventhub.yaml`。
  2. 执行 `make openapi-lint`。
  3. Makefile 通过固定版本 `npx --yes @redocly/cli@$(REDOCLY_CLI_VERSION)` 和 `redocly.yaml` 校验 spec。
- 本地 OpenAPI check 流程：
  1. `make openapi-check` 执行 `openapi-validate`。
  2. 执行 `openapi-generate`。
  3. 用 `git diff --exit-code api/openapi/gen/eventhub.gen.go` 检查 generated drift。
- CI 流程：
  1. generated-contract job 设置 Go 和 Node。
  2. 执行 `make openapi-lint`。
  3. 执行 `make generated-check`，继续覆盖 sqlc 和 OpenAPI generated drift。
- handler/service/repository/sqlc/database 分工不变，本次不触碰运行时分层。

## 9. 并发 / 幂等 / 缓存
- 不涉及库存、订单、支付或其他并发业务流程。
- 不涉及幂等 token、事务边界或缓存。
- CI 中 Redocly CLI 通过 npx 下载固定版本，结果由版本 pin 和配置文件控制，避免不同 runner 之间的规则漂移。

## 10. 权限与安全
- 不改变认证、授权、JWT claim 或 admin RBAC 运行时边界。
- Redocly recommended 中的 `security-defined` 与当前 Go policy 冲突：本仓库刻意要求顶层 security 为空，并由每个 operation 显式表达公开或 BearerAuth 策略。因此 Redocly 配置关闭该通用规则，继续由 Go policy test 检查团队安全约束。
- `x-required-roles` 仍只作为 OpenAPI 文档治理元数据，运行时授权仍由 `internal/http/middleware` 和 service 规则负责。

## 11. 测试策略
- OpenAPI lint：
  - `make openapi-lint`
  - 预期通过，且当前 nullable schema 问题已被修复。
- OpenAPI validate / generate：
  - `make openapi-check`
  - 预期通过，并确认 generated code 无未提交漂移。
- Go 测试：
  - `go test ./...`
  - 覆盖 OpenAPI policy test、router/spec 契约测试和真实响应契约测试。
- Go 静态检查：
  - `go vet ./...`
  - 本次不改 Go 生产代码，但仍作为质量门禁补充。
- 格式与空白：
  - `gofmt` 不适用手写 Go 文件；generated code 由 oapi-codegen 产生。
  - `git diff --check`
- Java-Go parity 验证：
  - 更新 parity matrix 的 OpenAPI / Swagger 行和质量门禁行，索引 Redocly lint、CI step 和 027 文档。

## 12. 风险与替代方案
- 当前方案风险：
  - npx 首次运行需要网络下载 npm 包，CI 首次执行会增加少量耗时。
  - Redocly 规则默认集可能随版本变化；固定 `REDOCLY_CLI_VERSION` 降低漂移。
  - `no-unused-components` 初始设为 warn，短期内不会阻断所有未使用组件问题。
- 备选方案 A：使用 Spectral。
  - 没有采用。Spectral 自定义规则能力更强，但本次只需要通用 OpenAPI 文档质量 lint，Redocly 配置更轻，维护成本更低。
- 备选方案 B：引入 `package.json` 和 lockfile。
  - 没有采用。仓库当前没有 Node 工具链，npx 固定版本能满足 CI 可运行和低维护目标。
- 备选方案 C：把 `openapi-lint` 并入 `openapi-check`。
  - 首阶段没有采用。`openapi-check` 当前语义是 validate/generate/generated diff，lint 作为独立文档质量门禁在 CI 中单独执行，失败定位更清楚。未来如果团队希望本地一个 target 覆盖所有 OpenAPI 检查，可以再把 lint 纳入 `openapi-check`。
- 后续可演进点：
  - 将 `operation-description` 从 warn 提升为 error，或改成 summary/description 二选一的自定义规则。
  - 将 `no-unused-components` 从 warn 提升为 error。
  - 对分页、错误码、幂等 header、callback 等业务规则继续使用 Go policy test 表达。
