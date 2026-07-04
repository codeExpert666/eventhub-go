# ADR-0024 OpenAPI v1 兼容性策略

## 标题
使用 oasdiff 在 PR 中阻断无意的 `/api/v1` breaking changes

## 状态
- accepted

## 背景
Go 版 EventHub 采用 spec-first OpenAPI，`api/openapi/eventhub.yaml` 是 API 契约源。仓库已经具备：

- `make openapi-lint`：通用 OpenAPI 文档质量 lint。
- `make openapi-check`：OpenAPI validate、oapi-codegen generate 和 generated diff。
- `go test ./...`：项目自定义 OpenAPI policy test、router/spec 对齐和真实响应契约测试。

这些门禁都在单一工作区内运行，无法判断当前 PR 相比 base branch 是否删除或收紧了已发布的 `/api/v1` 契约。随着 auth/user 之外的 event/order/payment API 增加，v1 API 需要明确的兼容性策略，避免客户端在无迁移窗口的情况下被破坏。

## 决策
选择 oasdiff 作为 OpenAPI breaking change 检测工具。

执行策略：

- 在 `Makefile` 固定 `OASDIFF_VERSION`。
- 新增 `make openapi-breaking-check`。
- 本地默认比较 `origin/main:api/openapi/eventhub.yaml` 与当前工作区 `api/openapi/eventhub.yaml`。
- 当 base ref 或 base spec 缺失时，target 返回失败并提示开发者先 fetch 或检查 base branch，不误报成功。
- GitHub Actions pull request workflow fetch PR base branch，再执行同一个 Makefile target。
- 默认只匹配 `^/api/v1($|/)`，聚焦稳定业务 API。

兼容策略：

- `/api/v1/**` 默认视为稳定 API，PR 不应无意引入 breaking change。
- 非破坏性演进优先，例如新增可选字段、新增响应字段、保留旧字段并标记 `deprecated: true`。
- 如果需要改变既有语义，优先新增 `/api/v2` 或新路径。
- 如果必须破坏 `/api/v1`，需要在设计文档或 ADR 中说明原因、影响范围、迁移策略和人工批准方式。
- 首版不引入 ignore 文件；后续如需豁免，每条豁免必须关联 ADR 或人工批准记录。

## 备选方案
- 方案 1：oasdiff + Go module 固定版本。
- 方案 2：Redocly 或 Spectral 自定义规则。
- 方案 3：引入独立脚本，手写 path/schema 差异比较。
- 方案 4：把 breaking change 检测合并进 `openapi-check`。
- 方案 5：仅依赖人工 review。

## 决策理由
- oasdiff 专门面向 OpenAPI diff 和 breaking change 分类，比 lint 工具更贴合跨版本兼容性检测。
- 通过 `go run github.com/oasdiff/oasdiff@...` 固定版本，符合仓库现有低频 Go 工具使用方式，不要求开发者预装新 CLI。
- 复用 Makefile target 能让本地和 CI 的比较逻辑保持一致。
- breaking check 依赖 base ref，独立于 `openapi-check` 更容易排查失败原因，也不改变现有 validate / generate / drift 语义。
- 限定 `/api/v1/**` 可以让首版策略聚焦业务 API 兼容性，避免 actuator 或文档路由影响 PR 门禁。

## 影响
- 好处
  - PR 会自动阻断无意删除路径、删除响应字段、收紧 schema 等 v1 breaking changes。
  - v1 兼容性策略有明确文档和 CI 执行点。
  - 后续新增 event/order/payment API 时，外部契约治理成本更低。
- 代价
  - 本地和 CI 首次运行 oasdiff 需要下载 Go module。
  - 本地 target 依赖 `origin/main` 或调用方指定的 base ref。
  - oasdiff 的 breaking 分类不能替代业务语义 review；复杂语义变化仍需设计文档和人工判断。
- 后续可能需要调整的地方
  - 为明确批准的 breaking change 引入 ignore 文件，并要求每条 ignore 有 ADR 或批准记录。
  - 当 `/api/v2` 出现后，为不同 API 版本设置不同的匹配规则。
  - 引入 deprecation-days 策略，让稳定端点必须经过足够长的废弃期后才能移除。
