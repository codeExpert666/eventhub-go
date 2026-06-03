# App Provider 与构造函数规则补强实现说明

## 1. 本次改动解决了什么问题

本次把最新确定的依赖组织规则沉淀到持久协作规则和 backend-design-first 执行清单中，解决以下问题：

- `AGENTS.md` 之前允许构造函数使用 `Dependencies` struct，容易把 composition root 的装配形状下沉到 service、handler、domain、repository。
- `internal/app/bootstrap.go` 随着 auth/user/db/security 装配增长，已经需要明确后续拆分到 `internal/app/providers` 的方向。
- `Bootstrap()` 内部隐式使用 `context.Background()` 的问题需要进入规则层，后续涉及 DB、Redis、外部探活或启动阻塞操作时应由调用方传入 context。
- backend-design-first skill 需要在执行阶段提醒检查构造函数、provider 和 bootstrap context 规则。

本次只改文档规则，不修改运行时代码；对外 API、错误码、数据库模型、JWT claim、refresh token 和 Java-Go 业务语义不变。

## 2. 改动内容
- 新增了什么
  - 新增 `docs/ai/design/010-app-provider-constructor-rules.md`，记录规则补强的背景、目标、范围、流程、风险和后续演进点。
  - 新增本实现说明 `docs/ai/implementation/010-app-provider-constructor-rules.md`。
- 修改了什么
  - `AGENTS.md`：
    - 长期目标结构中补充 `internal/app/application.go` 和 `internal/app/providers/{platform,system,auth,user,http}.go`。
    - 7.8 依赖组织规则中补充 `Bootstrap(ctx context.Context)`、`bootstrap.go` 与 providers 分工、provider 聚合结构体边界、构造函数显式参数优先、禁止 service/handler/domain/repository 使用字段转移型 `Dependencies` / `Deps` / `Options`。
    - 保留 `RouterDependencies`，但明确它只表达路由注册所需 handler / middleware，不负责对象创建。
  - `.agents/skills/backend-design-first/SKILL.md`：
    - Step 2 增加构造函数、app provider 和 bootstrap context 规则检查点。
    - Step 5 增加 provider 装配边界、显式构造函数和 caller-provided context 的实现检查点。
  - `docs/ai/parity/java-go-parity-matrix.md`：
    - 更新“Go 内部依赖组织与装配边界”记录，说明 2026-06-03 新增规则补强，以及当前 `authsvc.Dependencies`、`Bootstrap()` 属于后续需收敛的结构债。
- 删除了什么
  - 未删除文件。
  - 未修改运行时代码。
- 是否更新 Java-Go parity 记录
  - 已更新。该变化属于 Go 内部依赖组织规则固化，不改变 Java 对外契约。

## 3. 为什么这样设计
- 关键设计原因
  - `AGENTS.md` 是详细项目规则源，必须放完整规则，否则后续 Codex 生成代码仍可能延续 service 包内 `Dependencies`。
  - skill 是执行清单，只引用和提醒检查 `AGENTS.md`，避免复制大段规则导致两份文档漂移。
  - provider 聚合结构体放在 `internal/app/providers`，可以降低 bootstrap 文件膨胀，又不污染业务层构造 API。
  - 构造函数显式参数优先，可以让依赖增长直接暴露出来，促使审视职责边界，而不是用字段转移结构体隐藏复杂度。
  - `Bootstrap(ctx)` 让启动期依赖初始化可取消、可超时，也与 Go 生态中 context 由调用方传递的习惯一致。
- 与 Go 项目当前阶段的匹配点
  - 当前代码已经完成 router option 收敛、handler 直接依赖具体 service、bootstrap 创建 handler/service/middleware。
  - auth service 和 bootstrap 仍有结构债，本次先把规则固化，后续再用独立实现任务收敛代码，避免一次变更过大。
- 与 Java 版业务语义的对齐方式
  - Java 的 Spring DI 容器不是 Go 版迁移目标。
  - Go 版通过显式 composition root 和 `NewXxx` 构造函数保持 Controller / Service / Repository 分层语义。

## 4. 替代方案
- 方案 A：只更新 `AGENTS.md`。
  - 没有采用。依赖组织规则变更属于非微小修改，需要设计和实现记录支撑后续复盘。
- 方案 B：同时重构运行时代码，立即拆 `internal/app/providers`、修改 `Bootstrap(ctx)`、删除 `authsvc.Dependencies`。
  - 没有采用。本次用户目标是审视并沉淀规则；运行时代码重构会扩大范围，需要单独设计、实现和测试。
- 方案 C：继续允许 service 使用 `Dependencies` struct。
  - 没有采用。字段转移型 `Dependencies` 会把装配形状下沉到业务层，并掩盖职责过大的信号。
- 方案 D：把完整规则复制到 skill。
  - 没有采用。skill 目标是轻量执行清单，详细规则继续以 `AGENTS.md` 为准。

## 5. 测试与验证
- 跑了哪些测试
  - `go test ./...`：通过。
  - `make test`：通过，当前 target 执行 `go test ./...`。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `git diff --check`：通过。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `make test`：通过。
  - `make vet`：通过。
  - `gofmt` / `make fmt`：未运行，本次未修改 Go 文件。
  - `golangci-lint run`：未运行，当前环境 `command -v golangci-lint` 无输出且退出码为 1。
  - `sqlc generate`：未运行，本次未修改 SQL、migration 或 `sqlc.yaml`。
  - OpenAPI validate：未运行，Go 版当前仍没有 OpenAPI 契约文件，本次未修改 API 契约。
- 手工验证了哪些场景
  - 检查 `AGENTS.md` 7.2 和 7.8 已包含 app/providers、构造函数、provider 聚合边界和 `Bootstrap(ctx)` 规则。
  - 检查 backend-design-first skill 已增加对应执行检查点。
  - 检查 parity matrix 已更新 Go 内部依赖组织与装配边界记录。
- Java-Go parity 如何验证
  - 本次仅影响 Go 内部装配规则，Java 业务契约无变化。
  - parity matrix 已记录 Go 版刻意不迁移 Spring DI，而采用显式 composition root。
- 结果如何
  - 可运行验证均通过；lint 因本机未安装工具未运行。

## 6. 已知限制
- 当前版本还缺什么
  - `internal/app/providers` 尚未实际落地。
  - `Bootstrap()` 尚未改为 `Bootstrap(ctx context.Context)`。
  - `internal/service/auth.NewService` 仍接收 `authsvc.Dependencies`，属于本次明确记录的结构债。
- 哪些地方后面需要继续演进
  - 单独设计和实现 app/providers 拆分。
  - 单独设计和实现 `Bootstrap(ctx)`。
  - 收敛 auth service 构造函数为显式参数，或拆分职责以降低参数数量。
- 与 Java 版仍有哪些差距
  - Java Spring 容器装配不迁移到 Go；这是刻意差异。
  - Go 后续仍需用显式代码维护装配可读性和分层边界。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 规则能更清楚体现企业级 Go 后端的 composition root、显式依赖和分层边界。
- 对微服务 / 云原生演进的影响
  - `Bootstrap(ctx)` 和 app/providers 规则有利于后续接入启动超时、依赖探活、服务拆分和可观测性。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 新业务模块增加 service/handler/middleware 时，应优先显式构造并由 app/providers 装配。
  - provider 可以聚合依赖，但聚合结构体不得进入 service、handler、domain、repository。
  - SQL、migration、OpenAPI 不受本次影响；未来相关改动仍按原质量门禁执行。
