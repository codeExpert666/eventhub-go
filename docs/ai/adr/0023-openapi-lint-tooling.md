# ADR-0023 OpenAPI lint 工具选择

## 标题
使用 Redocly CLI 和 npx 固定版本补充 OpenAPI 文档质量 lint

## 状态
- accepted

## 背景
Go 版 EventHub 已采用 spec-first OpenAPI，并已有三类相关能力：

- `make openapi-validate` 负责 OpenAPI 结构校验。
- `make openapi-check` 负责 validate、oapi-codegen 生成和 generated diff。
- Go policy test 负责团队强约束，例如统一响应 envelope、错误响应集中引用、admin role 元数据、router/spec 对齐和真实响应契约校验。

这些能力还缺少一层通用 OpenAPI 文档质量 lint。后续 event/order/payment 等模块会持续增加接口，如果没有 lint，operationId、tags、summary/description、schema 示例和未使用组件等问题容易在人工 review 后期才暴露。

仓库当前没有 Node 项目结构，也没有 package lockfile。引入 lint 工具时需要优先降低维护成本，并保证 CI 易运行、版本可控。

## 决策
选择 Redocly CLI 作为 OpenAPI lint 工具。

执行方式：

- 在 `Makefile` 中固定 `REDOCLY_CLI_VERSION`。
- 通过 `npx --yes @redocly/cli@$(REDOCLY_CLI_VERSION)` 执行。
- 使用仓库根目录 `redocly.yaml` 管理规则。
- 新增 `make openapi-lint`。
- CI 中额外执行 `make openapi-lint`。
- 暂不把 lint 并入 `openapi-check`，保留 `openapi-check` 作为 validate/generate/generated diff 门禁。

规则策略：

- 从温和规则开始，不直接接入 Redocly recommended 的全部错误。
- 强制 operationId、operationId 唯一、summary、顶层 tag 定义、nullable/schema 示例有效性。
- `operation-description` 和 `no-unused-components` 初始设为 warn，后续稳定后再逐步提升。
- 关闭与本仓库刻意策略冲突的通用规则，例如 `security-defined`、`no-server-example.com`、`operation-4xx-response`。

## 备选方案
- 方案 1：Redocly CLI + npx 固定版本。
- 方案 2：Spectral CLI + `.spectral.yaml`。
- 方案 3：新增 `package.json`、lockfile 和 npm script。
- 方案 4：只继续使用 Go policy test，不引入通用 OpenAPI lint。

## 决策理由
- Redocly CLI 对 OpenAPI spec-first 项目开箱即用，配置文件较轻，适合当前“通用 lint、温和起步”的目标。
- Spectral 适合更复杂的自定义 style guide，但本次没有足够多的自定义规则需求；过早引入会增加维护成本。
- npx 固定版本符合仓库现有低频工具策略：不要求每个开发者预装 CLI，也不把 Node 项目结构引入 Go 后端仓库。
- `redocly.yaml` 可以明确关闭与本仓库安全策略冲突的通用规则，避免把团队 policy 和通用 lint 混在一起。
- 独立 `openapi-lint` target 让失败定位清楚：lint 失败是文档质量问题，`openapi-check` 失败是结构、生成或 generated drift 问题。

## 影响
- 好处
  - CI 增加通用 OpenAPI 文档质量检查。
  - 后续新增 API 时能更早发现 operationId、tag、summary/schema 类问题。
  - 与 Go policy test 形成互补：Redocly 管通用风格，Go test 管团队业务契约。
- 代价
  - CI generated-contract job 需要 Node 环境。
  - npx 首次执行需要下载 Redocly CLI。
  - Redocly 规则需要随项目成熟逐步调严。
- 后续可能需要调整的地方
  - 将 `operation-description` 和 `no-unused-components` 从 warn 提升为 error。
  - 如果未来 OpenAPI lint 规则大量自定义，再评估是否迁移到 Spectral 或引入 package lockfile。
  - 如果团队希望一个本地 target 覆盖全部 OpenAPI 门禁，可以把 `openapi-lint` 纳入 `openapi-check`。
