# Service Command / Query / Result 边界实现说明

## 1. 本次改动解决了什么问题

本次改动解决的是 service 层文件结构和输入输出契约命名缺少明确规范的问题。

在改动前，`internal/service/system/service.go` 同时包含 `Service` struct、constructor、`EchoCommand`、多个 `Result` 类型，以及 `Ping`、`Echo`、`Health`、`Info` 方法。system 模块当前简单，短期还能阅读；但后续 auth、user、event、order、inventory、payment 等模块会有更多用例、事务边界、幂等和状态流转，如果继续堆在一个 `service.go` 中，可读性和 code review 成本会明显变高。

本次把规范沉淀到设计文档、AGENTS、skill、ADR 和 parity matrix，并把 system service 拆成最小样板。

## 2. 改动内容
- 新增了设计文档：
  - `docs/ai/design/004-service-contract-boundary.md`
  - 说明 service Command / Query / Result 文件边界、AGENTS/skill 更新必要性、system 样板拆分计划和验证策略。
- 更新了项目持久规则：
  - `AGENTS.md`
  - 新增“Service Command / Query / Result 文件边界”，明确 `service.go`、`command.go`、`query.go`、`result.go` 和 use case 文件职责。
  - 更新结构检查清单和类型放置表。
- 更新了 backend-design-first skill：
  - `.agents/skills/backend-design-first/SKILL.md`
  - 新增 `Service contract boundary check`，要求每次新增或调整 service 前检查 Command / Query / Result 命名、文件放置、空文件、HTTP tag 和 sqlc 泄漏。
- 更新了 docs/ai 目录说明：
  - `docs/ai/README.md`
  - 新增 `Service Contract 规范`。
- 更新了 package layout ADR：
  - `docs/ai/adr/0005-go-project-package-layout.md`
  - 新增 `Service Command / Query / Result 文件边界` 小节，作为 ADR-0005 package layout 决策的细化。
- 更新了 Java-Go parity matrix：
  - `docs/ai/parity/java-go-parity-matrix.md`
  - 新增 `Service contract 文件边界` 行，记录 Go 版不逐字照搬 Java 单 Service 类结构，而是按同 package 多文件拆分。
- 拆分了 `internal/service/system`：
  - `service.go`：保留 `Service` struct、`NewService` 和依赖字段。
  - `command.go`：新增承载 `EchoCommand`。
  - `result.go`：新增承载 `PingResult`、`EchoResult`、`HealthResult`、`InfoResult`、`AppInfo`、`RuntimeInfo`。
  - `ping.go`：新增承载 `Ping`。
  - `echo.go`：新增承载 `Echo`。
  - `actuator.go`：新增承载 `Health` 和 `Info`。
- 没有创建的文件：
  - 没有创建 `query.go`，因为当前 system 模块没有 Query 类型。
  - 没有创建 `errors.go`，因为当前 system service 没有 service 层业务错误辅助。
- 是否更新 Java-Go parity 记录：
  - 已更新。本次属于 Go 版为了可读性和长期维护，对 Java Service 类结构做出的刻意 Go idiom 差异。

## 3. 为什么这样设计
- `service.go` 只保留依赖和 constructor，可以让读者快速看清一个 service 依赖什么。
- `command.go`、`query.go`、`result.go` 把 service 输入输出契约集中放置，便于 handler mapping、测试和 code review。
- 业务方法按 use case 拆分，可以让 auth/user/event/order 这类复杂模块避免单文件膨胀。
- 不创建 `internal/service/<domain>/dto` 子包，是因为 Command / Query / Result 不是 HTTP DTO，也不需要跨 package 暴露成独立子模块；同 package 多文件拆分足够清晰。
- 不创建空 `query.go`，符合项目阶段化落地原则，避免为了“看起来完整”制造空文件。
- 需要更新 `AGENTS.md`，因为这是后续所有 Codex 任务必须遵守的持久规则。
- 需要更新 skill，因为结构规范只有进入 workflow check，后续实现前才会被主动检查。

## 4. 替代方案
- 方案 A：继续把 `Service`、Command、Query、Result 和所有业务方法放在 `service.go`。
  - 没有采用，因为复杂模块会快速变成大文件，降低可读性。
- 方案 B：创建 `internal/service/<domain>/dto` 或 `contract` 子包。
  - 没有采用，因为这会增加跨包跳转，也容易和 HTTP DTO 混淆；当前同 package 文件拆分已经足够。
- 方案 C：按 Java 风格为每个 service 提前抽 interface 和 impl。
  - 没有采用，因为 Go 版当前没有多实现或端口替换需求，有真实抽象点时再引入接口更自然。
- 方案 D：机械一方法一文件。
  - 没有采用。当前采用“按 use case 或相关能力聚合”的方式，例如 actuator 的 `Health` 和 `Info` 放在同一个 `actuator.go`。

## 5. 测试与验证
- 跑了哪些测试：
  - `go test ./...`：通过。
  - `make test`：通过，内部执行 `go test ./...`。
- 跑了哪些质量门禁：
  - `gofmt -w .`：通过。
  - `go vet ./...`：通过。
  - `make fmt`：通过，内部执行 `gofmt -w .`。
  - `make vet`：通过，内部执行 `go vet ./...`。
  - `git diff --check`：通过，无空白错误。
  - `golangci-lint run`：未运行成功，当前 shell 返回 `command not found: golangci-lint`；仓库已有 `.golangci.yml`，安装工具后可继续执行。
  - `sqlc generate`：不适用，本次没有 SQL、schema 或 sqlc 配置变化。
  - migration 测试：不适用，本次没有 migration 变化。
  - OpenAPI validate：不适用，本次没有 OpenAPI 契约文件变化。
- 手工验证：
  - `internal/service/system/service.go` 不再承载 Command / Result 和业务方法。
  - `command.go`、`result.go`、`ping.go`、`echo.go`、`actuator.go` 均在同一 package `system` 下，未改变对外方法签名。
  - 没有新增空 `query.go`。
  - service 仍不依赖 `internal/http/dto`。
  - `rg -n "internal/http/dto|json:\"|sqlc\\." internal/service || true`：无输出，确认 service contract 没有 HTTP DTO、JSON tag 或 sqlc 泄漏。
- Java-Go parity 如何验证：
  - 对照 Java `SystemService#ping`、`SystemService#echo`，确认 Go 版仅调整文件组织，保留业务语义。
  - 对照 Java DTO / VO 进入 service 的方式，确认 Go 版仍由 handler 做 DTO 与 Command / Result 映射。

## 6. 已知限制
- system service 当前仍主要通过 handler 测试覆盖；后续复杂 service 应补充独立 service 单元测试。
- 当前规范还没有在 auth/user/event/order 等复杂模块中验证，后续迁移时需要持续复查。
- 如果未来某个模块 use case 极多，可能需要进一步按子能力组织文件，但仍应保持同 package 和 Command / Query / Result 边界。
- 当前没有新增 service interface；如果未来出现多实现、端口替换或跨模块抽象，再通过设计文档和 ADR 评估。
- 结构债方面，本次已经清理 system service 大文件问题；剩余风险是后续模块实现时是否持续遵守规范。

## 7. 对后续版本的影响
- 对简历可用版的价值：
  - service 层有清晰的输入、输出和 use case 文件边界，更容易展示工程化分层能力。
- 对微服务 / 云原生演进的影响：
  - Command / Query / Result 边界稳定后，未来拆分服务、抽端口或引入异步消息时更容易定位用例契约。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响：
  - 新增业务模块时，service package 默认按 `service.go`、`command.go`、`query.go`、`result.go` 和 use case 文件组织。
  - 新增 repository/sqlc 时，service Result 仍不能直接暴露 sqlc generated model。
  - OpenAPI schema 仍映射到 HTTP DTO，而不是 service Command / Query / Result。
