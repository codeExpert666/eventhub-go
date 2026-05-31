# Go 项目结构规范化重构实现说明

## 1. 本次改动解决了什么问题

本次改动把上一阶段已经写入规则和 ADR 的 Go package layout 从“文档约束”推进到“运行时代码实际落地”。

重构前，`cmd/eventhub/main.go` 同时承担配置加载、日志初始化、signal handling 和 HTTP server 装配；system handler 直接持有 config 并组装 ping/echo/info/health 响应；system HTTP DTO 定义在 handler 文件内；request id 位于 `internal/http/requestid`，不利于后续日志、追踪、response、recover 等跨 HTTP 子包复用；`response`、`apperror`、`page` 也都集中在单文件中。

本次将这些职责拆到长期规范目录中，同时保持现有 API 路径、统一响应字段、错误码、requestId、分页和测试意图不变。

## 2. 改动内容
- 新增了应用装配层：
  - `internal/app/bootstrap.go`
  - `internal/app/lifecycle.go`
  - `cmd/eventhub/main.go` 现在只调用 `app.Run()` 并处理最终退出状态。
- 拆分了配置包：
  - `internal/config/config.go`：`Config`、`LogConfig`、`Load`、`Addr`。
  - `internal/config/env.go`：环境变量读取、正整数解析、日志级别解析。
  - `internal/config/profile.go`：`dev/test/prod`、`ActiveProfiles`、环境标准化。
  - 保持 `EVENTHUB_APP_NAME`、`EVENTHUB_ENV`、`EVENTHUB_HTTP_PORT`、`EVENTHUB_VERSION`、`EVENTHUB_LOG_LEVEL` 兼容。
- 迁移了 request id：
  - 删除 `internal/http/requestid/requestid.go` 和对应测试。
  - 新增 `internal/platform/idgen/request_id.go` 和 `request_id_test.go`。
  - 对外函数改为 `HeaderRequestID`、`NewRequestID`、`ValidRequestID`、`WithRequestID`、`RequestIDFromContext`。
  - 更新 middleware、recover、response 和 HTTP 测试 import。
- 拆分了统一响应包：
  - `internal/http/response/api_response.go`：`APIResponse`、`Success`、`Failure`。
  - `internal/http/response/writer.go`：`WriteSuccess`、`WriteError`、`WriteStatus`、`WriteJSON`。
  - `data` 字段不加 `omitempty`，保持 `data:null` 可见。
- 拆分了应用错误包：
  - `internal/apperror/code.go`：`Code`、错误码、默认消息和 HTTP status。
  - `internal/apperror/error.go`：`AppError`、`New`、`WithData`、`Wrap`、`Error`、`Unwrap`、访问器。
  - `internal/apperror/mapper.go`：`FromError`、`normalizeCode`。
- 拆分了分页包：
  - `internal/page/page_request.go`：分页常量、`Request`、`NewRequest`、`DefaultRequest`、`Offset`。
  - `internal/page/page_response.go`：`Response[T]`、`NewResponse`、`totalPages` 计算。
- 新增了 system DTO 和 service：
  - `internal/http/dto/system_dto.go`：`EchoRequest`、`PingResponse`、`EchoResponse`、`HealthResponse`、`InfoResponse` 等 HTTP data 契约。
  - `internal/service/system/service.go`：`Service`、`EchoCommand`、service result 类型和 ping/echo/health/info 组装逻辑。
  - `internal/platform/clock/clock.go`：`Clock` interface 和 `RealClock`。
  - `internal/http/handler/system_handler.go` 改为 decode/validate DTO、调用 service、映射 service result 到 HTTP DTO。
- 新增了阶段可用工程资产：
  - `README.md`
  - `Makefile`
  - `.golangci.yml`
  - `configs/dev.env.example`
  - `configs/test.env.example`
  - `configs/prod.env.example`
  - `api/openapi/.gitkeep`
  - `migrations/.gitkeep`
- 未创建的未来阶段目录：
  - 没有创建 `internal/domain`、`internal/repository`、`internal/security`、`internal/platform/db`、`internal/platform/redis` 等空 Go package。
  - 没有创建 `sqlc.yaml`、Dockerfile、docker-compose.yml 或 OpenAPI 契约文件。
- 是否更新 Java-Go parity 记录：
  - 已更新。项目结构从规则状态进入部分代码落地状态，且 system Java Controller/Service/DTO/VO 语义映射到 Go handler/service/dto，触发 parity matrix 更新。

## 3. 为什么这样设计
- `internal/app` 承接进程装配和生命周期，让 `cmd/eventhub/main.go` 保持最薄入口，避免后续业务或基础设施初始化散落在 main。
- `internal/config` 拆分后，配置结构、环境变量读取和 profile 派生各自独立，但对外仍通过 `config.Load()` 和 `Config` 使用，兼容现有调用方。
- request id 上移到 `internal/platform/idgen`，是因为它已经被 middleware、recover、response、日志和测试共同依赖，不再只是 HTTP 子目录内部工具。
- `internal/http/dto` 承载 system HTTP data 契约，落实 ADR-0006 的 DTO / VO 边界；service 不依赖 DTO，避免传输层类型渗入业务用例。
- `internal/service/system` 对齐 Java `SystemService` 的职责，把 ping/echo/info/health 的非 HTTP 组装逻辑从 handler 中挪出。
- `Clock` interface 是小而明确的抽象，未来 system service 单元测试可以替换固定时间；当前 response envelope timestamp 仍由 response 包使用 `time.Now()`，因为它属于统一响应写出时刻，后续如需统一时钟可另行设计。
- `response`、`apperror`、`page` 拆分为职责文件，不改变 package 名和对外 API，降低后续新增错误码、writer 或分页能力时的文件膨胀。
- `README.md`、`Makefile`、配置示例和 `.gitkeep` 只提供当前阶段可用或明确保留的落点；不创建当前无法验证的 Docker/sqlc/OpenAPI 骨架。

## 4. 替代方案
- 方案 A：只更新文档，不改运行时代码。
  - 没有采用，因为本次目标是结构规范化重构，必须把关键目录和包边界实际落地。
- 方案 B：一次性创建完整长期目录和空 Go 文件。
  - 没有采用，因为当前没有真实 domain/repository/security/db/redis 代码，空 package 会增加无意义编译单元。
- 方案 C：system handler 继续直接持有 config 并组装响应。
  - 没有采用，因为 Java 版已有 `SystemController -> SystemService` 边界，Go 版也需要在当前最小模块中验证 handler/service 分工。
- 方案 D：service 直接使用 `internal/http/dto`。
  - 没有采用，因为这会让 service 依赖 HTTP 传输层，破坏 DTO 边界和后续复用能力。
- 方案 E：立即创建 Dockerfile、docker-compose.yml、sqlc.yaml。
  - 没有采用，因为当前没有数据库、SQL、migration 或容器运行约束；创建不可执行骨架会误导后续验证。

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
  - `golangci-lint run`：未通过运行前置条件，本机 shell 返回 `command not found: golangci-lint`，说明当前环境未安装该工具；`.golangci.yml` 已落地，后续安装工具后可直接运行。
  - `sqlc generate`：不适用，本次没有 SQL、schema 或 sqlc 配置。
  - migration 测试：不适用，本次没有 migration。
  - OpenAPI validate：不适用，本次没有 OpenAPI 契约文件。
- 手工验证：
  - `git ls-files .idea` 无输出，说明 `.idea` 当前没有被 Git 跟踪；保留 `.gitignore` 中 `.idea/` 规则即可。
  - 检查旧 `internal/http/requestid` 文件已删除，运行时代码 import 已切换到 `internal/platform/idgen`。
- Java-Go parity 验证：
  - 对照 Java `SystemController` 和 `SystemService`，确认 Go handler 只处理 HTTP 边界，Go system service 负责 system 响应数据组装。
  - 对照 Java `EchoRequest`、`PingInfo`、`EchoInfo`，确认 Go HTTP DTO 保持 `message/tag/serviceName/activeProfiles/serverTime/echoedAt` 字段语义。
  - 对照 Java `ApiResponse`、`ErrorCode`、`PageRequest`、`PageResponse`，确认拆分文件不改变外部 JSON 字段、错误码或分页计算。

## 6. 已知限制
- `internal/http/router.go` 当前仍负责创建默认 system service 和 handler。随着依赖增多，可继续把 router dependency assembly 收敛到 `internal/app` 的显式 dependency struct。
- `internal/service/system` 当前依靠 handler 测试覆盖端到端行为，尚未新增独立 service 单元测试；当前逻辑分支很少，后续加入更多 health components 后应补齐。
- response envelope 的 timestamp 仍使用 `time.Now()`，尚未接入 `platform/clock`；这不影响现有契约，但未来如需固定时间测试可以继续演进。
- `api/openapi/.gitkeep` 和 `migrations/.gitkeep` 只是未来落点，不代表当前已有 OpenAPI 契约或 migration。
- `.golangci.yml` 已新增，但是否能运行取决于本机是否安装 `golangci-lint`。

## 7. 对后续版本的影响
- 对简历可用版的价值：
  - 项目已经有更接近生产后端的入口、app、handler、service、DTO、platform 基础设施边界。
- 对微服务 / 云原生演进的影响：
  - `internal/app`、`internal/platform`、`internal/service` 和 `internal/http/dto` 的边界为后续拆分服务、接入 DB/Redis/OpenAPI/观测能力预留了清晰位置。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响：
  - 新增业务模块时必须继续遵守 `handler -> service -> repository -> sqlc/database`。
  - 新增数据库访问时再创建 repository/mysql、queries、sqlc 和 migration，并运行对应生成和迁移验证。
  - 新增 API 契约时再创建 `api/openapi/eventhub.yaml`，避免当前阶段生成空契约。
