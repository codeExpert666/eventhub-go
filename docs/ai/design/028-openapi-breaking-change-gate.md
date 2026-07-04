# OpenAPI breaking change 门禁设计

## 1. 背景
- 当前 Go 版采用 spec-first OpenAPI，契约源是 `api/openapi/eventhub.yaml`。
- API 路径已经使用 `/api/v1` 前缀，但仓库现有门禁只覆盖 OpenAPI 结构合法性、生成代码漂移、通用文档 lint 和团队 policy test。
- PR 目前不会自动比较 base branch 与当前分支的 OpenAPI 契约差异，因此删除路径、删除响应字段、收紧 schema 等 v1 breaking change 可能在 review 中被遗漏。
- Java 版通过 Controller、DTO、Springdoc 注解和测试共同表达接口契约；Go 版不逐行迁移 Springdoc 机制，而是通过 spec-first YAML 和 CI 门禁承接相同的契约治理目标。

## 2. 目标
- 新增 `make openapi-breaking-check`，使用固定版本 oasdiff 检测 `/api/v1/**` OpenAPI breaking changes。
- 本地检查默认比较 `origin/main:api/openapi/eventhub.yaml` 与当前工作区 `api/openapi/eventhub.yaml`。
- 当本地没有 `origin/main` 或 base branch 中没有 OpenAPI spec 时，给出清晰提示并失败，不误报成功。
- 在 GitHub Actions pull request workflow 中新增 OpenAPI breaking change 检测 job，比较 PR base branch 与当前 PR 工作区的 spec。
- 文档记录 v1 API 兼容性策略，以及确实需要 breaking change 时的处理路径。
- 成功标准：
  - CI PR 中有 OpenAPI breaking change job。
  - `make openapi-breaking-check` 在存在 `origin/main` 时可运行。
  - `go test ./...` 通过。
  - `make openapi-check` 通过。

## 3. 非目标
- 不修改 HTTP handler、service、repository、sqlc query、migration、数据库模型或运行时业务逻辑。
- 不改变 `api/openapi/eventhub.yaml` 当前 API 契约内容。
- 不把 breaking change 检测并入 `make openapi-check`；`openapi-check` 继续保持 validate、generate 和 generated drift 语义。
- 不引入 Node 项目结构、npm lockfile 或 OpenAPI SaaS 服务依赖。
- 首版不增加 ignore 文件或自动豁免机制，避免 breaking change 被静默放行。

## 4. 影响范围
- `Makefile`：
  - 新增 `OASDIFF_VERSION`、`OPENAPI_BASE_REF`、`OPENAPI_BREAKING_MATCH_PATH`。
  - 新增 `openapi-breaking-check` target。
- `.github/workflows/ci.yml`：
  - 新增仅在 pull request 运行的 OpenAPI breaking change job。
  - checkout 当前 PR commit 后 fetch base branch，再调用 Makefile target。
- `README.md`：
  - 更新本地命令和 OpenAPI 门禁分工说明。
- `docs/ai/adr/`：
  - 新增 ADR，记录 v1 API 兼容性策略和 oasdiff 选择。
- `docs/ai/implementation/`：
  - 实现后新增 implementation note。
- `docs/ai/parity/java-go-parity-matrix.md`：
  - 更新 OpenAPI / Swagger 与质量门禁记录，因为本次改变 API 契约治理和 CI 验证策略。

## 5. 领域建模
- 本次不新增业务领域实体。
- 质量门禁模型：
  - `Base OpenAPI spec`：PR base branch 或本地 `origin/main` 上的 `api/openapi/eventhub.yaml`。
  - `Revision OpenAPI spec`：当前工作区的 `api/openapi/eventhub.yaml`。
  - `Breaking change`：oasdiff 判定会破坏既有客户端兼容性的 API 契约变化，例如删除路径、删除 method、删除响应字段、收紧请求/响应 schema、移除状态码等。
  - `v1 protected path`：默认只匹配 `^/api/v1($|/)`，聚焦稳定业务 API。
- 与 Java 版领域对象没有直接对应关系；这是 Go spec-first 契约治理能力。

## 6. API 设计
- 不新增或修改 HTTP API 路径、方法、请求字段、响应字段、状态码或错误码。
- v1 兼容策略：
  - `/api/v1/**` 默认视为稳定 API，PR 不应无意引入 breaking change。
  - 非破坏性演进优先：新增可选字段、新增响应字段、保留旧字段并标记 `deprecated: true`。
  - 需要改变既有语义时，优先新增 `/api/v2` 或新路径。
  - 必须破坏 `/api/v1` 时，需要 ADR 或设计文档说明原因、影响范围、迁移策略和人工批准方式。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Java 版没有直接迁移 oasdiff；Go 版使用 oasdiff 比较 spec-first YAML，目的是保持对外契约语义一致和可审计。

## 7. 数据设计
- 不调整表结构、索引、唯一约束或 migration。
- 不新增 sqlc query。
- 不重新生成 OpenAPI Go 代码，因为本次不修改 `api/openapi/eventhub.yaml`。
- 数据一致性不受影响。

## 8. 关键流程
- 本地流程：
  1. 开发者执行 `make openapi-breaking-check`。
  2. Makefile 检查 `$(OPENAPI_BASE_REF)` 是否存在，默认是 `origin/main`。
  3. Makefile 检查 `$(OPENAPI_BASE_REF):$(OPENAPI_SPEC)` 是否存在。
  4. 调用固定版本 `go run github.com/oasdiff/oasdiff@$(OASDIFF_VERSION) breaking "$(OPENAPI_BASE_REF):$(OPENAPI_SPEC)" "$(OPENAPI_SPEC)" --fail-on ERR --match-path "$(OPENAPI_BREAKING_MATCH_PATH)"`。
  5. oasdiff 检出 breaking changes 时返回非零，Make target 失败。
- PR CI 流程：
  1. checkout 当前 PR commit。
  2. setup Go。
  3. fetch `github.base_ref` 到 `refs/remotes/origin/<base>`。
  4. 执行 `make openapi-breaking-check OPENAPI_BASE_REF=origin/<base>`。
- handler / service / repository / sqlc/database 分工不变，本次只调整工程质量门禁。

## 9. 并发 / 幂等 / 缓存
- 不涉及活动库存、订单、支付、refresh token 或其他并发业务流程。
- 不涉及请求幂等、事务边界或缓存。
- CI 中 oasdiff 使用固定 Go module 版本，降低不同 runner 上判定结果漂移。

## 10. 权限与安全
- 不改变认证、授权、JWT claim 或 RBAC 运行时边界。
- breaking check 会发现安全 scheme、参数、响应等 OpenAPI 契约层变化，但不会替代运行时鉴权测试。
- `x-required-roles`、BearerAuth 和 admin operation policy 仍由 `go test ./...` 中的 OpenAPI policy test 负责。

## 11. 测试策略
- TDD / RED：
  - 在实现前运行 `make openapi-breaking-check`，预期因为 target 不存在而失败。
- Makefile target 验证：
  - `make openapi-breaking-check`，在当前存在 `origin/main` 时应成功运行并无 breaking changes。
  - 可选：临时覆盖不存在的 `OPENAPI_BASE_REF` 验证错误提示明确，例如 `make openapi-breaking-check OPENAPI_BASE_REF=origin/__missing__`。
- OpenAPI 现有门禁：
  - `make openapi-check`。
  - `make openapi-lint`。
- Go 质量门禁：
  - `go test ./...`。
  - `go vet ./...`。
- Java-Go parity 验证：
  - 更新 parity matrix，记录 Go 端用 oasdiff 补充 v1 API 兼容性检查。

## 12. 风险与替代方案
- 当前方案风险：
  - `go run github.com/oasdiff/oasdiff@...` 首次运行需要网络下载 Go module。
  - oasdiff 对 breaking change 的定义是工具规则，不等价于所有业务语义风险；复杂语义仍需人工 review 和设计文档。
  - 本地比较依赖 `origin/main`，未 fetch 时会失败并提示开发者先更新 base ref。
- 备选方案 A：使用 Redocly 或 Spectral 自定义规则检测 breaking changes。
  - 没有采用。Redocly / Spectral 更适合 lint 和 style guide；跨版本契约 diff 不是它们的核心职责。
- 备选方案 B：把 breaking check 并入 `openapi-check`。
  - 没有采用。`openapi-check` 当前是单分支内的 validate、generate、generated drift；breaking check 依赖 base ref，独立 target 更容易理解和排错。
- 备选方案 C：检查整个 OpenAPI spec。
  - 没有采用首版全量检查。当前需求聚焦 `/api/v1` 兼容性，actuator 等非业务 API 后续可以按需要纳入。
- 后续可演进点：
  - 增加显式 ignore 文件，但要求每条 ignore 都关联 ADR 或人工批准记录。
  - 对 beta / stable endpoint 引入 deprecation-days 策略。
  - 当 `/api/v2` 出现后，为不同版本配置不同的兼容策略。
