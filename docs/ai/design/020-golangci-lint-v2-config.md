# golangci-lint v2 配置迁移设计

## 1. 背景
- 当前仓库 `.golangci.yml` 使用 golangci-lint v1 风格配置：
  - `linters.disable-all: true`
  - `gofmt` 作为 linter 启用
  - `issues.exclude-use-default: false`
- golangci-lint v2 已调整配置结构：
  - `linters.disable-all` 迁移为 `linters.default: none`。
  - `gofmt` 等格式检查从 `linters` 迁移到 `formatters`。
  - `issues.exclude-use-default` 迁移到 `linters.exclusions.presets`；当前值为 `false`，等价于不启用预置排除规则。
- Java 版没有对应 lint 配置；本次属于 Go-only 工程质量门禁维护，不涉及 Java 业务语义迁移。

## 2. 目标
- 将 `.golangci.yml` 更新为 golangci-lint v2 配置风格。
- 保持当前低噪音规则集语义不变：
  - `gofmt`
  - `govet`
  - `ineffassign`
  - `staticcheck`
  - `unused`
- 同步 Makefile / README / ADR 中的固定 golangci-lint 版本，避免 v2 配置被 v1 执行器读取失败。
- Makefile 增加本机 golangci-lint 版本探测：只有本机版本匹配固定版本时才使用本机工具；不匹配时回退 Docker fallback。
- 成功标准：
  - v2 配置 schema 可验证。
  - `make lint` 可通过固定 v2 Docker fallback 执行。
  - 模拟本机旧版 golangci-lint 时，`make lint` 不再调用旧版本机工具，而是回退 Docker。
  - 不改变业务 Go 代码、API、错误码、数据库或 OpenAPI 契约。

## 3. 非目标
- 不新增或删除 lint 规则。
- 不启用更高噪音的风格、复杂度、安全或错误包装规则。
- 不修改 handler / service / repository / domain 分层。
- 不修改 Java-Go 业务 parity 语义。
- 不调整 GitHub Actions job 结构。

## 4. 影响范围
- 涉及文件：
  - `.golangci.yml`
  - `Makefile`
  - `README.md`
  - `docs/ai/adr/0022-golangci-lint-quality-gate.md`
  - `docs/ai/design/020-golangci-lint-v2-config.md`
  - 后续 `docs/ai/implementation/020-golangci-lint-v2-config.md`
- 涉及 Go package / 模块：
  - 不触碰 Go package。
  - 不新增、移动或删除业务 Go 文件。
- 涉及 API / 表 / 缓存 / 外部接口：
  - 不涉及生产 API、数据库表、Redis 缓存或业务外部接口。
  - 涉及本地 / CI 质量门禁工具版本。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 不需要新增业务 parity 记录；本次仅迁移 Go-only lint 配置格式，检查规则和质量门禁意图保持不变。
  - 若已有质量门禁索引行需要表达 v2 版本，可做轻量补充。

## 5. 领域建模
- `LintConfig`
  - `.golangci.yml`，声明 lint 和 formatter 检查集合。
- `LintRunner`
  - 本机 `golangci-lint` 或 Docker fallback。
  - 必须使用固定版本读取 v2 配置。
- `QualityGate`
  - `make lint` 与 `make quality` 的一部分。
  - 不承载业务领域状态。
- 与 Java 版领域对象的对应关系：
  - 无对应 Java 领域对象。
  - 这是 Go 端工程质量能力。

## 6. API 设计
- 本次不新增或修改 HTTP API。
- 不修改请求字段、响应字段、状态码、错误码或 OpenAPI 契约。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - 不适用。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：不涉及业务数据读写。

## 8. 关键流程
- 正常流程：
  1. 开发者运行 `make lint`。
  2. Makefile 读取本机 `golangci-lint version`。
  3. 本机版本匹配 `GOLANGCI_LINT_VERSION` 时，使用本机工具运行 `golangci-lint run ./...`。
  4. 本机未安装或版本不匹配时，使用固定 Docker 镜像运行 `golangci-lint run ./...`。
  5. golangci-lint v2 读取 `.golangci.yml`。
  6. `linters` 执行 `govet`、`ineffassign`、`staticcheck`、`unused`。
  7. `formatters` 执行 `gofmt` 检查但不直接改写文件。
- 异常流程：
  - 本机安装的是 v1 或其他非固定版本时，Makefile 打印提示并回退 Docker。
  - Docker 不可用且本机未安装 v2 工具时，`make lint` 失败并暴露原因。
  - `go.mod` / `go.sum` 需要隐式更新时，`modules-download-mode: readonly` 让 lint 失败，提醒先显式整理依赖。
- 状态流转：
  - 不涉及业务状态机。
- handler / service / repository / sqlc/database 分工：
  - 不改变分工。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无。
- 如何防重复提交：不涉及业务接口幂等。
- 事务边界在哪里：不改变事务边界。
- 缓存放在哪里，为什么：
  - golangci-lint 和 Go toolchain 可继续使用自身缓存。
  - Docker fallback 首次可能拉取 v2 镜像。

## 10. 权限与安全
- 哪些角色能访问：
  - 本地开发者和 CI workflow 可运行质量门禁。
- 鉴权与鉴别约束：
  - 不涉及应用鉴权。
- JWT claim 边界：
  - 不修改 JWT。
- 是否涉及敏感信息、审计或操作日志：
  - 不新增 secrets。
  - Docker fallback 只挂载当前仓库目录用于 lint。

## 11. 测试策略
- 单元测试：
  - 本次不修改 Go 业务代码，单元测试用于确认仓库仍可编译测试。
- service / repository 测试：
  - 不涉及业务行为变化，运行现有 `go test ./...`。
- migration / sqlc 验证：
  - 不修改 SQL、schema、migration 或 sqlc 配置，不运行 `sqlc generate`。
- 接口验证：
  - 不修改 HTTP API，不运行额外接口验证。
- OpenAPI validate：
  - 不修改 OpenAPI 契约，不运行 OpenAPI validate。
- 异常场景验证：
  - 验证 v2 配置可被 golangci-lint 读取。
  - 验证 `make lint` 使用固定 v2 Docker fallback 可运行。
  - 通过临时 fake `golangci-lint` 模拟旧版本，验证版本不匹配时回退 Docker。
- Java-Go parity 验证：
  - 确认没有业务契约变化。
- 需要运行的命令：
  - `gofmt` 不适用：未修改 Go 文件。
  - `go test ./...`
  - `go vet ./...`
  - `make lint`
  - 可行时运行 v2 配置校验命令。

## 12. 风险与替代方案
- 当前方案的风险：
  - 本机版本不匹配时即使是较新的 v2 版本也会回退 Docker；这是固定版本策略的代价。
  - Docker fallback 首次拉取 v2 镜像会更慢。
  - v2 内部规则实现相较 v1 可能有少量报告差异，即使启用规则名相同。
- 备选方案：
  - 方案 A：只改 `.golangci.yml`，不改 Makefile 固定版本。
  - 方案 B：保留 v1 配置，暂不迁移。
  - 方案 C：迁移 v2 同时启用更多规则。
  - 方案 D：Makefile 总是使用 Docker v2，不再优先本机工具。
  - 方案 E：只检查主版本是 v2，不要求匹配固定版本。
- 为什么不选备选方案：
  - 不选方案 A：v2 配置配 v1 runner 会导致 `make lint` 不稳定。
  - 不选方案 B：不满足本次“新版风格”目标。
  - 不选方案 C：规则集扩大属于质量策略变化，容易引入噪音；本次只做格式迁移。
  - 不选方案 D：会降低已安装固定版本工具开发者的本地体验，且偏离既有 ADR 的本机优先策略。
  - 不选方案 E：仓库已采用固定版本策略，只检查主版本仍可能造成不同机器 lint 结果漂移。
- 后续可演进点：
  - 后续可在业务模块稳定后评估 `errcheck`、`revive`、`gocritic`、`gosec` 等规则。
