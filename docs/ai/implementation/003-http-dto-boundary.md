# HTTP DTO 与 VO 边界规范实现说明

## 1. 本次改动解决了什么问题

本次改动解决的是 Go 版 EventHub 在继续迁移 Java DTO / VO 语义前，需要确认运行时代码和文档规则是否已经统一的问题。

Java 版中 `modules/system/vo/PingInfo`、`modules/system/vo/EchoInfo` 以及 auth 模块下的 `vo/*` 都是典型 HTTP 展示对象；Go 版不逐字复制 `VO` 包名，而是把 HTTP request/response data 统一放入 `internal/http/dto/<module>`，把统一响应 envelope 和 writer 留在 `internal/http/response`，把 DDD Value Object 留给 domain 层。

本次审计确认当前 Go 运行时代码已经符合该边界，没有需要移动的 Go struct；因此改动集中在更新设计文档、ADR 和 parity matrix，把“规则已初始化”推进为“当前代码已审计确认”。

## 2. 改动内容
- 更新了设计文档：
  - `docs/ai/design/003-http-dto-boundary.md`
  - 从早期“只固化规则、不迁移 Go DTO”的设计，更新为本次运行时代码审计设计。
  - 补充 Java 来源、当前审计结果、涉及目录、不触及目录、API 兼容性、审计流程和验证命令。
- 更新了 package layout ADR：
  - `docs/ai/adr/0005-go-project-package-layout.md`
  - 在“HTTP DTO 与 VO 边界”小节补充 2026-06-01 审计结论：当前不存在 `internal/http/vo`、`internal/**/vo`、文件名包含 `vo` 的 Go 文件或 `*VO` struct，system/actuator HTTP DTO 已位于 `internal/http/dto`。
- 更新了 Java-Go parity matrix：
  - `docs/ai/parity/java-go-parity-matrix.md`
  - 补充 Java system/auth DTO/VO 来源，并记录 Go 端本次审计确认的 DTO、response、service 边界状态。
- 文件移动和 package 边界变化：
  - 本次没有移动 Go 文件。
  - 本次没有创建 `internal/http/vo`。
  - 本次没有新增空 Go package。
  - 本次没有修改 API、handler 行为、service 行为、response body shape、JSON 字段或错误码。
- DTO 与 service command/domain model 的映射关系：
  - `internal/http/dto/system.EchoRequest` 由 handler decode 和 validate。
  - handler 将 `EchoRequest` 映射为 `internal/service/system.EchoCommand`。
  - service 返回 `PingResult`、`EchoResult`、`HealthResult`、`InfoResult` 等 result 类型。
  - handler 将 service result 映射为 `systemdto.PingResponse`、`systemdto.EchoResponse`、`systemdto.HealthResponse`、`systemdto.InfoResponse`。
  - `internal/service/system` 不依赖 `internal/http/dto`；当前没有 domain/repository 代码参与。
- 是否更新 Java-Go parity 记录：
  - 已更新。Java VO 命名习惯与 Go HTTP DTO / domain Value Object 边界属于刻意结构差异，且本次审计了当前运行时代码，触发 parity matrix 更新条件。

## 3. 为什么这样设计
- `internal/http/dto` 集中承载 HTTP 传输契约，适合管理 request body、query/path 参数辅助对象和 response data。
- `internal/http/response` 保持为统一 envelope 和 writer 包，可以避免具体业务 response 与 `APIResponse`、`WriteSuccess`、`WriteError` 混在一起。
- 不创建 `internal/http/vo`，可以避免 Java View Object 与 DDD Value Object 的命名歧义。
- service 使用 Command / Result 类型，不复用 HTTP DTO，能保持 `handler -> service -> repository -> sqlc/database` 的长期边界。
- 当前代码已经满足规则，继续移动 Go 文件只会制造无意义 diff；把审计结论写入设计、ADR 和 parity matrix，更利于后续 auth/user/event 阶段复用。

## 4. 替代方案
- 方案 A：创建 `internal/http/vo` 存放响应对象。
  - 没有采用，因为 `VO` 在 Java 语境可表示 View Object，在 DDD 语境又可表示 Value Object，长期会造成歧义。
- 方案 B：把业务 response 放入 `internal/http/response`。
  - 没有采用，因为该包已经承担统一响应 envelope 和 writer；放入业务 DTO 会混淆职责。
- 方案 C：让 service 直接复用 `internal/http/dto`。
  - 没有采用，因为这会让业务用例层依赖 HTTP 传输层，破坏后续复用和测试边界。
- 方案 D：在本次强行重命名或拆分现有 system DTO。
  - 没有采用，因为现有 `PingResponse`、`EchoRequest`、`EchoResponse`、`HealthResponse`、`InfoResponse` 已符合命名和目录规范，强改会增加不必要风险。

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
- 手工 / 命令审计：
  - `find internal -path '*/vo' -o -path '*/vo/*' -o -iname '*vo*.go' -print`：无输出，未发现运行时代码 VO 目录或 VO 文件。
  - `rg -n "\b[A-Za-z0-9_]*VO\b|internal/http/vo|/vo" internal api cmd .agents AGENTS.md || true`：运行时代码无命中；仅规则文档中有禁止和说明性命中。
  - `rg -n "^type\s+\w+\s+struct" internal/http/handler internal/http/response internal/http/dto internal/service`：确认 system handler 位于模块子包，HTTP DTO 位于 dto 模块子包，response 包只有 `APIResponse`，service 为 Command / Result 类型。
  - `rg -n "internal/http/dto" internal/service internal/domain internal/repository || true`：无输出，service/domain/repository 未依赖 HTTP DTO。
- Java-Go parity 如何验证：
  - 对照 Java `modules/system/dto/request/EchoRequest`、`modules/system/vo/PingInfo`、`modules/system/vo/EchoInfo`，确认 Go 版 system HTTP request/response 已在 `internal/http/dto/system`。
  - 对照 Java `modules/auth/vo/*`，确认未来 auth 迁移应继续落到 Go HTTP DTO，而不是新增 `internal/http/vo`。

## 6. 已知限制
- 当前业务 DTO 仍主要集中在 system/actuator 端点；auth/user/event/order 等模块还未迁移，后续仍需在每次实现中重复做 DTO boundary check。
- 当前没有 OpenAPI schema，未来如果生成代码命名与本规范冲突，需要在设计文档或 ADR 中说明兼容例外。
- 当前没有 domain value object 落地；未来新增 `Email`、`Username`、`Money`、`OrderNo` 等类型时，需要放入 domain 层并避免 HTTP JSON 契约职责。
- 结构债方面，本次审计未发现 HTTP DTO / VO 边界债；剩余的是未来业务模块迁移时的持续执行风险。

## 7. 对后续版本的影响
- 对简历可用版的价值：
  - HTTP DTO、统一响应 envelope、service command/result 和 domain value object 的边界更清楚，能展示 Go 版如何对齐 Java 契约但不复制 Java 命名习惯。
- 对微服务 / 云原生演进的影响：
  - DTO 与 domain value object 的边界清晰后，未来服务拆分、OpenAPI schema 管理和跨服务契约治理会更稳定。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响：
  - 新增 HTTP request/response 时默认进入 `internal/http/dto/<module>`。
  - 新增 DDD Value Object 时默认进入 `internal/domain/<domain>` 或 `internal/domain/common`。
  - 新增 repository/sqlc 时继续通过 repository/mysql 映射，不把 sqlc generated model 暴露给 handler 或 DTO。
