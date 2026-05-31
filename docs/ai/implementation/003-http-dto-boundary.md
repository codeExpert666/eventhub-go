# HTTP DTO 与 VO 边界规范实现说明

## 1. 本次改动解决了什么问题

本次改动解决了 Go 版 EventHub 后续新增 HTTP 请求/响应结构体时缺少明确 DTO / VO / domain value object 边界的问题。

Java 版中常见 `VO` 命名可表达响应展示对象，但 Go 版如果照搬 `internal/http/vo`，会和 DDD Value Object 混淆；同时 `internal/http/response` 已承载统一 `APIResponse` envelope 和 writer，不应再放具体业务 response DTO。本次将规则固化到项目协作规则、backend-design-first skill、docs/ai README、ADR 和 parity matrix。

## 2. 改动内容
- 新增了本次设计文档：
  - `docs/ai/design/003-http-dto-boundary.md`
- 更新了项目级协作规则：
  - `AGENTS.md`
  - 在 Go 项目目录结构规范中新增“HTTP DTO / VO / Value Object 边界”，明确不设置 `internal/http/vo`、HTTP 请求和响应结构体统一放 `internal/http/dto`、`internal/http/response` 只放统一响应 envelope 和 writer、DDD Value Object 放 domain。
- 更新了 backend-design-first skill：
  - `.agents/skills/backend-design-first/SKILL.md`
  - 新增 `HTTP DTO boundary check`，要求设计和实现前检查是否新增 HTTP request/response、具体业务 response、VO、service 对 DTO 的依赖、domain `json` tag、sqlc generated model 泄漏。
- 更新了 docs/ai 目录说明：
  - `docs/ai/README.md`
  - 新增“HTTP DTO 与 VO 规范”，说明 DTO 边界调整属于非微小修改，需更新 design / implementation note / parity matrix，例外需写 ADR。
- 更新了 package layout ADR：
  - `docs/ai/adr/0005-go-project-package-layout.md`
  - 补充“HTTP DTO 与 VO 边界”小节，不覆盖已有 package layout 决策。
- 新增了 HTTP DTO / VO 边界 ADR：
  - `docs/ai/adr/0006-http-dto-vs-vo-boundary.md`
  - 状态为 Accepted，记录不创建 `internal/http/vo`、HTTP request/response 统一放 `internal/http/dto`、DDD Value Object 放 domain 的长期决策。
- 更新了 Java-Go parity matrix：
  - `docs/ai/parity/java-go-parity-matrix.md`
  - 新增 `HTTP DTO / Java VO 对照` 行，记录 Java VO 命名习惯到 Go DTO/domain value object 边界的刻意差异。
- 文件移动和 package 边界变化：
  - 本次没有移动任何 Go 文件。
  - 本次没有新建空 Go package。
  - 本次没有创建 `internal/http/vo`。
  - 本次没有改变现有运行时 package 边界，只新增和更新文档规则。
- DTO 与 service command/domain model 的映射关系：
  - 本次不新增具体 DTO。
  - 规则明确未来由 handler 将 `internal/http/dto` 中的 request DTO 映射为 service Command / Query，并将 service result / domain model 映射为 response DTO。
  - service、repository 和 domain 不依赖 `internal/http/dto`；repository/mysql 负责 sqlc row 与 domain model 的映射。
- 是否更新 Java-Go parity 记录：
  - 已更新。Java VO 命名习惯与 Go HTTP DTO / domain Value Object 边界属于刻意结构差异，触发 parity matrix 更新条件。

## 3. 为什么这样设计
- `AGENTS.md` 是项目级持久规则，适合固化后续所有 Codex 任务必须遵守的 DTO / VO 边界。
- backend-design-first skill 负责把规则转成每次任务前的检查动作，避免新增 DTO 时才临时判断放置位置。
- `docs/ai/README.md` 补充规范入口，便于后续复盘和审查时从 docs/ai 目录理解规则来源。
- ADR-0006 记录这是一项长期命名和 package 边界决策，后续偏离时有清晰索引。
- parity matrix 记录 Go 版不逐字照搬 Java VO 命名，而是按 HTTP DTO 和 DDD Value Object 两种语义拆分。
- 不创建 `internal/http/vo`，是为了避免 View Object 与 Value Object 的命名歧义。
- 不把具体业务 response 放入 `internal/http/response`，是为了让该包专注统一 envelope 和 writer。

## 4. 替代方案
- 方案 A：创建 `internal/http/vo` 存放响应对象。
  - 没有采用，因为 `VO` 在 Java 语境可表示 View Object，在 DDD 语境可表示 Value Object，长期会造成歧义。
- 方案 B：拆分 `internal/http/request` 和 `internal/http/response` 存业务请求/响应。
  - 没有采用，因为 `internal/http/response` 已承担统一响应 envelope 和 writer；继续放业务 response 会混淆职责。
- 方案 C：所有 HTTP 传输结构体统一放 `internal/http/dto`。
  - 本次采用。通过 `Request`、`Response`、`ListItemResponse`、`SummaryResponse`、`DetailResponse` 等后缀表达用途。
- 方案 D：允许 service 直接复用 HTTP DTO 减少映射代码。
  - 没有采用，因为这会让 service 依赖 HTTP 传输层，破坏 handler -> service -> repository -> sqlc/database 分层。

## 5. 测试与验证
- 跑了哪些测试：
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
- 跑了哪些质量门禁：
  - `grep -R "internal/http/vo" AGENTS.md .agents docs README.md || true`：已运行。根目录没有 `README.md`，因此 grep 输出缺失文件提示；其余命中均为禁止、决策、备选方案或验证命令说明，没有发现旧规则鼓励创建 `internal/http/vo`。
  - `grep -R "HTTP DTO" AGENTS.md .agents docs README.md`：已运行。根目录没有 `README.md`，原命令返回缺失文件提示；随后对现有路径 `AGENTS.md .agents docs` 重跑通过，确认 DTO 规则可检索。
  - `grep -R "Value Object" AGENTS.md .agents docs README.md`：已运行。根目录没有 `README.md`，原命令返回缺失文件提示；随后对现有路径 `AGENTS.md .agents docs` 重跑通过，确认 domain value object 规则可检索。
  - `git diff --check`：通过。
  - `gofmt`：不适用，本次没有修改 Go 文件。
  - `sqlc generate`：不适用，本次没有 SQL、schema 或 sqlc 配置变化。
  - migration 测试：不适用，本次没有 migration 变化。
  - OpenAPI validate：不适用，本次没有 API 契约变化。
- 手工验证了哪些场景：
  - 复查规则不鼓励创建 `internal/http/vo`。
  - 复查 `internal/http/response` 的职责仍是统一 envelope 和 writer。
  - 复查 service/domain/repository 不依赖 HTTP DTO 的规则表达明确。
- Java-Go parity 如何验证：
  - 对照 Java request DTO、response DTO 和 VO 命名习惯，确认 Go 目标记录为 `internal/http/dto`、`internal/http/response` 和 `internal/domain` 的边界拆分。
- 结果如何：
  - 本次为文档和协作规则修改，不改变运行时行为。
  - Go 测试、vet、diff 空白检查均通过。
  - 可选 `README.md` 更新未执行，因为仓库根目录当前没有 `README.md`。

## 6. 已知限制
- 当前没有新增具体业务 DTO，因此规则尚未通过业务代码落地验证。
- 当前没有 OpenAPI schema，未来如果生成代码命名与本规范冲突，需要在设计文档或 ADR 中说明例外。
- 当前 domain model 不新增或调整，因此 `json` tag 规则只在文档层固化。
- 后续新增 auth/user/event/order/ticket DTO 时，需要在 implementation note 中写明 DTO 与 service command/domain model 的映射关系。

## 7. 对后续版本的影响
- 对简历可用版的价值：
  - 后续 API 层、service 层和 domain 层边界更清楚，能展示 Go 版如何对齐 Java 契约但不复制 Java 命名习惯。
- 对微服务 / 云原生演进的影响：
  - DTO 与 domain value object 的边界清晰后，未来服务拆分、OpenAPI schema 管理和跨服务契约治理会更稳定。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响：
  - 新增 HTTP request/response 时默认进入 `internal/http/dto`。
  - 新增 DDD Value Object 时默认进入 `internal/domain/<domain>` 或 `internal/domain/common`。
  - 新增 sqlc 查询或 repository 时，仍需通过 repository/mysql 映射，不把 generated model 暴露给 handler 或 DTO。
