# HTTP handler / DTO 模块化组织实现说明

## 1. 本次改动解决了什么问题

本次改动解决的是 HTTP 层随业务模块增长后的目录组织问题：如果所有 handler 都直接放在 `internal/http/handler` 根目录、所有 request / response DTO 都直接放在 `internal/http/dto` 根目录，后续 auth、user、event、order、ticket、payment 等模块会让目录扫描、命名和 code review 成本持续上升。

本次将规则固化到 `AGENTS.md` 和 backend-design-first skill，并把现有 system HTTP 层迁移为模块化样板：`internal/http/handler/system` 与 `internal/http/dto/system`。API 契约、JSON 字段、错误码、service 行为和测试语义保持不变。

## 2. 改动内容
- 新增了设计文档：
  - `docs/ai/design/005-http-module-package-boundary.md`
  - 明确目标、非目标、涉及目录、不触及目录、DTO/handler 模块化规则、测试策略和替代方案。
- 更新了持久协作规则：
  - `AGENTS.md`
  - 新增 `HTTP handler / DTO 模块化组织` 小节。
  - 明确具体业务 handler 默认放 `internal/http/handler/<module>`，具体业务 DTO 默认放 `internal/http/dto/<module>`。
  - 明确 `request.go` / `response.go` 拆分规则、import alias 建议和不创建空文件原则。
- 更新了 backend skill：
  - `.agents/skills/backend-design-first/SKILL.md`
  - 新增实现前的 HTTP module package check。
  - 将 HTTP DTO boundary check 从 `internal/http/dto` 细化为 `internal/http/dto/<module>`。
- 更新了 ADR：
  - `docs/ai/adr/0005-go-project-package-layout.md`
  - `docs/ai/adr/0006-http-dto-vs-vo-boundary.md`
  - 记录 HTTP handler/dto 按模块子包组织的长期决策。
- 更新了 Java-Go parity matrix：
  - `docs/ai/parity/java-go-parity-matrix.md`
  - 新增 HTTP handler / DTO 模块化组织记录。
  - 更新 system ping、system echo、actuator health/info 的 Go 目标路径。
- 更新了既有 DTO 边界文档中的当前路径引用：
  - `docs/ai/design/003-http-dto-boundary.md`
  - `docs/ai/implementation/003-http-dto-boundary.md`
- 文件移动和 package 边界变化：
  - 删除 `internal/http/handler/system_handler.go`。
  - 新增 `internal/http/handler/system/handler.go`，package 为 `system`，对外类型为 `Handler`，constructor 为 `NewHandler`。
  - 删除 `internal/http/dto/system_dto.go`。
  - 新增 `internal/http/dto/system/request.go`，承载 `EchoRequest`。
  - 新增 `internal/http/dto/system/response.go`，承载 `PingResponse`、`EchoResponse`、`HealthResponse`、`InfoResponse`、`AppInfoResponse`、`RuntimeInfoResponse`。
  - `internal/http/router.go` 改为通过 `systemhandler` alias 调用 `systemhandler.NewHandler`。
- DTO 与 service command/domain model 的映射关系：
  - `systemdto.EchoRequest` 由 handler decode 和 validate。
  - handler 将 `systemdto.EchoRequest` 映射为 `internal/service/system.EchoCommand`。
  - service 返回 `PingResult`、`EchoResult`、`HealthResult`、`InfoResult`。
  - handler 将 service result 映射为 `systemdto.PingResponse`、`systemdto.EchoResponse`、`systemdto.HealthResponse`、`systemdto.InfoResponse`。
  - `internal/service/system` 不依赖 `internal/http/dto`。
- 是否更新 Java-Go parity 记录：
  - 已更新。本次是 Go 版为了模块可读性对 Java Controller / DTO / VO 结构做出的 Go idiom 包组织差异，触发 parity matrix 更新条件。

## 3. 为什么这样设计
- `handler/<module>` 和 `dto/<module>` 保持 HTTP 层横向分层，同时补上模块边界，避免根目录变成所有业务的混合命名空间。
- DTO 子包内优先拆 `request.go` / `response.go`，比每个结构体单独一个文件更克制，也比全部堆在一个 `xxx_dto.go` 更清晰。
- handler 子包内使用 `Handler` / `NewHandler`，调用处通过 `systemhandler`、`authhandler` 这类 alias 表达模块，减少类型名重复。
- 现有 system 模块虽然简单，但迁移后能作为后续复杂模块的样板，避免规则只停留在文档里。
- service/domain/repository 边界没有变化，仍由 handler 负责 HTTP DTO 与 service Command / Result 的映射。

## 4. 替代方案
- 方案 A：继续保持 `internal/http/handler/*.go` 和 `internal/http/dto/*.go` 扁平目录。
  - 没有采用，因为复杂模块增多后，目录扫描、类型命名和 code review 成本都会变差。
- 方案 B：拆为 `internal/http/request/<module>` 和 `internal/http/response/<module>`。
  - 没有采用，因为 `internal/http/response` 已承担统一 envelope 和 writer 职责，业务 response 再进入同名边界容易混淆。
- 方案 C：全面改成纵向 `internal/modules/<module>/{handler,dto,service}`。
  - 没有采用，因为当前 ADR 已选择混合式 package layout；全面纵向模块化会扩大改动面，也会打散现有 service/repository/sqlc 边界。
- 方案 D：只写规则，不调整 system 代码。
  - 没有采用，因为当前 system 迁移成本低，落一个样板能降低后续实现时的解释成本。

## 5. 测试与验证
- 跑了哪些测试：
  - `go test ./...`：通过。
  - `make test`：通过，内部执行 `go test ./...`。
- 跑了哪些质量门禁：
  - `gofmt -w internal/http/router.go internal/http/handler/system/handler.go internal/http/dto/system/request.go internal/http/dto/system/response.go`：通过。
  - `make fmt`：通过，内部执行 `gofmt -w .`。
  - `go vet ./...`：通过。
  - `make vet`：通过，内部执行 `go vet ./...`。
  - `git diff --check`：通过，无空白错误。
  - `golangci-lint run`：未运行成功，当前 shell 返回 `command not found: golangci-lint`；仓库已有 `.golangci.yml`，安装工具后可继续执行。
  - `sqlc generate`：不适用，本次没有 SQL、schema 或 sqlc 配置变化。
  - migration 测试：不适用，本次没有 migration 变化。
  - OpenAPI validate：不适用，本次没有 OpenAPI 契约变化。
- 手工 / 命令审计：
  - `find internal/http/handler -maxdepth 2 -type f | sort`：仅有 `internal/http/handler/system/handler.go`。
  - `find internal/http/dto -maxdepth 2 -type f | sort`：仅有 `internal/http/dto/system/request.go` 和 `internal/http/dto/system/response.go`。
  - `rg -n "internal/http/dto" internal/service`：无输出，service 未依赖 HTTP DTO。
  - `find internal -path '*/vo' -o -path '*/vo/*' -o -iname '*vo*.go' -print`：无输出，未新增 VO 目录或 VO 文件。
- Java-Go parity 如何验证：
  - 对照 Java system Controller / DTO / VO，确认 Go 版仅改变 HTTP 层 package 组织，保留路径、字段、状态码、错误码、validation 和 handler/service 分工。

## 6. 已知限制
- 当前只有 system 模块迁入新结构；auth/user/event/order 等复杂模块尚未开始，规则还需要在后续业务实现中持续执行。
- `handler/system`、`dto/system`、`service/system` 同名时必须使用 import alias 保持阅读清晰。
- 当前还没有 OpenAPI schema；未来引入生成代码时，需要重新确认 schema 命名与 `dto/<module>` 子包的映射关系。
- 当前没有 domain/repository 目录落地；本次只验证了 service 未反向依赖 HTTP DTO。

## 7. 对后续版本的影响
- 对简历可用版的价值：
  - HTTP 层目录规则更接近真实业务项目演进方式，后续 auth/user/event/order 模块可直接按样板落地。
- 对微服务 / 云原生演进的影响：
  - handler/dto 按模块归组后，后续拆 OpenAPI schema、权限边界、审计日志或服务拆分时更容易定位 HTTP 契约。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响：
  - 新增 HTTP handler 默认进入 `internal/http/handler/<module>`。
  - 新增 HTTP request/response 默认进入 `internal/http/dto/<module>`。
  - 新增 service/repository/sqlc 时仍遵守 `handler -> service -> repository -> sqlc/database`，不让 DTO 或 sqlc model 穿透层边界。
