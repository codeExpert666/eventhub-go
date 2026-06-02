# Java-Go parity matrix 审计实现说明

## 1. 本次改动解决了什么问题

本次改动解决了 `docs/ai/parity/java-go-parity-matrix.md` 中信息重复、状态边界不够清楚、以及 Java 已有但 Go 尚未迁移能力记录不完整的问题。

矩阵现在按“当前已落地的 Go 基线”和“Java 已有但 Go 待迁移/待决策的能力”重新收敛，避免把规则、ADR、运行时代码和未来计划混成难以维护的长表。

## 2. 改动内容
- 新增设计文档：
  - `docs/ai/design/006-parity-matrix-audit.md`
- 新增实现说明：
  - `docs/ai/implementation/006-parity-matrix-audit.md`
- 更新 Java-Go parity matrix：
  - `docs/ai/parity/java-go-parity-matrix.md`
  - 合并重复行：业务错误、错误码、参数校验、panic 统一归入基础错误与异常映射；handler/DTO/VO/service 文件边界归入一组结构边界。
  - 补齐遗漏行：OpenAPI、数据库迁移与 MyBatis、数据库测试策略、auth API、当前用户和管理员用户、JWT/RBAC、refresh token/auth sessions、容器化部署、活动票务后续业务。
  - 调整状态表达：对当前 Go 只做规则初始化或决策但未写代码的能力，避免标成完全已对齐。
- 是否更新 Java-Go parity 记录：
  - 已更新。本次工作本身就是 parity matrix 审计与收敛。

## 3. 为什么这样设计
- parity matrix 应该是索引表，而不是替代设计文档或 implementation note。
- 按语义能力合并，可以减少重复行，同时仍覆盖 API、错误码、数据模型、认证安全、测试和工程资产。
- 对待迁移能力明确写出 Java 来源和 Go 目标，可以让后续迁移 auth、database、OpenAPI 时直接知道该改哪一行。
- 对 Go 当前仅有规则、`.gitkeep` 或配置示例的能力使用 `待迁移` / `待决策`，能更真实表达项目进度。

## 4. 替代方案
- 方案 A：只追加遗漏项。
  - 没有采用，因为原矩阵中已有重复表达，追加会让后续维护更困难。
- 方案 B：按 Java 文件逐项列出。
  - 没有采用，因为会把矩阵变成文件清单，难以保持“最简最全”。
- 方案 C：按语义能力重写矩阵。
  - 已采用。它能保留完整对齐信息，同时让矩阵更短、更稳定。

## 5. 测试与验证
- 跑了哪些测试：
  - `go test ./...`：通过。
  - `make test`：通过。
- 跑了哪些质量门禁：
  - `go vet ./...`：通过。
  - `make vet`：通过。
  - `git diff --check`：通过。
  - `gofmt`：不适用，本次没有 Go 文件变化。
  - `golangci-lint run`：未通过运行前置条件，当前 shell 返回 `command not found: golangci-lint`；仓库已有 `.golangci.yml`，安装工具后可运行。
  - `sqlc generate`：不适用，本次没有 SQL、schema 或 sqlc 配置变化。
  - migration 测试：不适用，本次没有 migration 变化。
  - OpenAPI validate：不适用，本次没有 OpenAPI 契约文件变化。
- 手工验证：
  - 对照 `git ls-files` 确认 Go 当前只有 system/HTTP foundation、docs、config 示例、OpenAPI/migration 占位。
  - 对照 Java `common`、`system`、`auth`、`infra/security`、`db/migration`、`mapper/auth` 和测试文件，确认矩阵新增的待迁移能力都有 Java 来源。
  - 对照 Go `docs/ai/design/003/004/005` 和 ADR-0005/0006，确认 HTTP DTO、handler 模块化、service contract 边界已在当前代码中落地。
- Java-Go parity 如何验证：
  - 已直接在 matrix 中更新每个语义能力的来源、Go 目标、状态和差异原因。

## 6. 已知限制
- 矩阵仍是当前阶段快照；后续迁移 auth、database 或 OpenAPI 后必须再次更新。
- 活动、票务、订单、支付、通知、审计等模块在 Java 侧当前主要来自 roadmap，不是已落地 production code；矩阵只按后续业务方向记录为待决策。
- 当前 Go 仓库尚无数据库和 OpenAPI 契约，因此对应验证命令仍不可运行。

## 7. 对后续版本的影响
- 对简历可用版的价值：
  - 矩阵更清楚地展示 Go port 当前已经完成哪些基础工程能力，以及下一阶段应优先迁移哪些 Java 能力。
- 对微服务 / 云原生演进的影响：
  - 数据库、认证安全、容器化和 OpenAPI 的缺口被显式列出，便于后续按阶段补齐。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响：
  - 后续新增模块时，直接更新对应语义能力行，避免重复创建“泛 API / 泛数据库 / 泛测试”行。
