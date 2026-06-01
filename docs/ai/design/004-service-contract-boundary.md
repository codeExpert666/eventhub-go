# Service Command / Query / Result 边界设计

## 1. 背景
- 当前 Go 版 EventHub 已经建立 `handler -> service -> repository -> sqlc/database` 分层，system 模块也已从 handler 中抽出 `internal/service/system`。
- 现状中 `internal/service/system/service.go` 同时包含：
  - `Service` struct 与 constructor。
  - service 输入类型 `EchoCommand`。
  - service 输出类型 `PingResult`、`EchoResult`、`HealthResult`、`InfoResult`。
  - 真正业务方法 `Ping`、`Echo`、`Health`、`Info`。
- system 模块当前简单，所以大文件尚可阅读；但后续 auth、user、event、order、inventory、payment 等模块会出现更多 use case、Command / Query、Result、状态流转和事务边界，如果继续混在一个文件中，可读性和 code review 成本会快速变差。
- Java 版 `SystemService` 当前也是一个类内承载方法和返回对象引用；Go 版不逐字复制 Java 类结构，而是用同 package 多文件拆分表达更自然的 Go 工程边界。

## 2. 目标
- 固化 service 层输入输出结构体拆分规范：
  - `command.go` 放写操作输入 `XxxCommand`。
  - `query.go` 放读操作或列表查询输入 `XxxQuery`。
  - `result.go` 放 service 输出 `XxxResult` 和仅供 service 层返回使用的 summary/item 类型。
  - `service.go` 放 `Service` struct、constructor 和依赖字段。
  - 复杂业务方法按 use case 拆到 `register.go`、`login.go`、`create_event.go` 等文件。
- 拆分当前 `internal/service/system/service.go`，把 system 模块作为后续模块的最小样板。
- 明确 `AGENTS.md` 和 `.agents/skills/backend-design-first/SKILL.md` 需要同步更新：
  - `AGENTS.md` 是持久项目规则，必须沉淀长期文件结构规范。
  - skill 是每次实现前的检查流程，必须让 Codex 在生成 service 代码前做 service contract boundary check。
- 保持现有 API 路径、JSON 字段、错误码、handler 映射、service 行为和测试语义不变。
- 成功标准：
  - `internal/service/system` 不再把 Service、Command、Result 和业务方法全部堆在一个文件。
  - service 不依赖 `internal/http/dto`，也不带 HTTP `json` tag。
  - 不创建空 `query.go` 或无意义 package。
  - `gofmt -w .`、`go test ./...`、`go vet ./...` 通过；如 Makefile 存在，`make fmt`、`make test`、`make vet` 通过。

## 3. 非目标
- 不实现 auth、user、event、order、inventory、payment 等新业务。
- 不改变 system API 的路径、方法、请求字段、响应字段、状态码或错误码。
- 不新增 repository、domain、database、sqlc、migration 或 OpenAPI。
- 不把 service 输入输出放到 `internal/http/dto`，也不新增 `internal/service/dto`。
- 不为了凑规范创建空 `query.go`；只有出现 Query 类型时才创建。
- 不给每个 service 提前抽 interface；仍保持“有真实抽象点再引入接口”的原则。

## 4. 影响范围
- 本次触及的 Go package / 模块：
  - `internal/service/system`
  - `docs/ai/design`
  - `docs/ai/implementation`
  - `docs/ai/adr`
  - `docs/ai/parity`
  - `AGENTS.md`
  - `.agents/skills/backend-design-first/SKILL.md`
- 本次明确不触及的运行时代码目录：
  - `cmd`
  - `internal/app`
  - `internal/config`
  - `internal/platform`
  - `internal/http`
  - `internal/apperror`
  - `internal/page`
  - `internal/domain`
  - `internal/repository`
  - `internal/security`
  - `api/openapi`
  - `migrations`
  - `configs`
- 涉及 API / 表 / 缓存 / 外部接口：
  - 不涉及 API 契约、数据库表、缓存或外部接口变化。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 是。本次是 Go 版为了长期可读性和边界清晰，对 Java service 类结构做出的 Go idiom 文件拆分差异，需要记录为 Java-Go parity 的结构差异。

## 5. 领域建模
- `Service`：
  - 业务用例入口和依赖持有者，放在 `service.go`。
  - 当前 system service 持有 `config.Config` 和 `clock.Clock`。
- `Command`：
  - 写操作或会改变业务状态的用例输入。
  - 即使当前 system `EchoCommand` 不改变持久化状态，它仍是 POST echo 的服务层输入，属于 command 边界。
- `Query`：
  - 读操作、列表、搜索、详情查询的服务层输入。
  - 当前 system 模块没有 Query 类型，因此不创建 `query.go`。
- `Result`：
  - service 输出，承载业务用例结果，不带 HTTP JSON 契约。
  - 当前 system 的 `PingResult`、`EchoResult`、`HealthResult`、`InfoResult` 和内部 `AppInfo`、`RuntimeInfo` 放入 `result.go`。
- 与 Java 版领域对象的对应关系：
  - Java `SystemService#ping`、`SystemService#echo` 方法继续对应 Go `Service.Ping`、`Service.Echo`。
  - Java 的 request DTO / VO 不直接进入 Go service；Go handler 映射为 Command，service 返回 Result，再由 handler 映射为 HTTP DTO。

## 6. API 设计
- 本次不新增或修改 API。
- 既有 API 保持：
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET /actuator/health`
  - `HEAD /actuator/health`
  - `GET /actuator/info`
  - `HEAD /actuator/info`
- 请求参数和响应结构：
  - HTTP DTO 不变，仍由 `internal/http/dto` 承载。
  - service Command / Result 类型不直接暴露给 HTTP 调用方。
- 错误码 / 异常场景：
  - 本次不改变错误码和 validation 行为。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - 无 API 契约差异；仅 Go service package 内部文件组织不同。

## 7. 数据设计
- 本次不新增数据库表、索引、唯一约束、migration 或 sqlc query。
- 本次不影响 sqlc generated model。
- 数据一致性考虑：
  - system service 仍是无状态数据组装，不引入事务、锁或持久化一致性问题。
  - 后续订单、库存、支付等模块的事务边界仍由 service 方法承载，Command / Query / Result 拆分只影响文件组织，不改变业务一致性规则。

## 8. 关键流程
- 正常流程：
  1. handler decode HTTP DTO。
  2. handler validate HTTP DTO。
  3. handler 将 DTO 映射为 service Command / Query。
  4. service 方法执行业务规则并返回 Result 或 domain model。
  5. handler 将 Result / domain model 映射为 HTTP response DTO。
  6. handler 调用统一 response writer。
- 本次 system 文件拆分流程：
  1. `service.go` 保留 `Service` struct 和 `NewService`。
  2. `command.go` 放 `EchoCommand`。
  3. `result.go` 放 `PingResult`、`EchoResult`、`HealthResult`、`InfoResult`、`AppInfo`、`RuntimeInfo`。
  4. `ping.go` 放 `Ping`。
  5. `echo.go` 放 `Echo`。
  6. `actuator.go` 放 `Health` 和 `Info`。
- 异常流程：
  - 如果未来某个 service 文件继续堆叠大量 Command / Query / Result 和方法，应拆分为上述结构。
  - 如果某个 service 只有一个非常小的用例，可以先保持 `service.go + command.go/result.go`，但不能让 HTTP DTO 或 sqlc model 泄漏进 service contract。
- 状态流转：
  - `service.go 大文件 -> 设计固定 service contract 文件边界 -> AGENTS/skill/ADR/parity 记录 -> system 样板落地 -> 后续模块复用`。
- handler / service / repository / sqlc/database 分工：
  - handler：HTTP DTO decode、validate、mapping、response writer。
  - service：Command / Query / Result、业务规则、事务边界、幂等、状态流转。
  - repository：持久化语义接口。
  - repository/mysql：sqlc row 与 domain model 映射。
  - sqlc/database：生成查询代码，不承载业务判断。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及运行时并发、幂等或缓存变化。
- 规范层面明确：
  - 幂等键、状态流转输入、库存扣减参数等应进入 Command / Query，而不是 HTTP DTO。
  - service 方法仍是事务边界和并发一致性策略的主要承载位置。
  - Result 不表达缓存策略；缓存命中与失效策略应在 service 设计文档中单独说明。

## 10. 权限与安全
- 本次不实现认证、授权或 JWT。
- 规范层面明确：
  - 权限上下文如果需要进入 service，应通过 Command / Query 中明确字段或 security principal 类型表达。
  - 不把角色、邮箱、用户名、用户状态写入 JWT。
  - Command / Query / Result 不带 HTTP JSON tag，避免误暴露安全敏感字段。

## 11. 测试策略
- 单元测试：
  - 本次不改变业务逻辑，不新增 service 单元测试；通过现有 HTTP 测试覆盖 system 行为。
  - 后续复杂 service 应按用例补充 service 单元测试。
- service / repository 测试：
  - 本次不涉及 repository。
  - 拆分后通过 `go test ./...` 确认 package API 未破坏。
- migration / sqlc 验证：
  - 不适用，本次没有 SQL、schema、sqlc 或 migration 变化。
- 接口验证 / OpenAPI validate：
  - 不适用，本次没有 OpenAPI 契约变化。
- 异常场景验证：
  - 现有 handler 测试继续覆盖 JSON 解析失败、字段校验失败、not found、panic recover、actuator HEAD 无 body 等场景。
- Java-Go parity 验证：
  - 对照 Java `SystemService`，确认 Go 版仅改变文件组织，保留 `ping` / `echo` 语义和 handler/service 分工。
- 需要运行：
  - `gofmt -w .`
  - `go test ./...`
  - `go vet ./...`
  - `make fmt`
  - `make test`
  - `make vet`
  - `golangci-lint run`，如果工具可用。

## 12. 风险与替代方案
- 当前方案的风险：
  - 对 system 这种小模块来说，文件数量会增加；但它作为后续复杂模块样板，有助于降低长期成本。
  - 如果每个微小用例都拆一个文件，可能出现文件碎片化；后续应按 use case 聚合，而不是机械一方法一文件。
  - 规范需要写入 AGENTS 和 skill，否则后续 Codex 任务可能不会主动执行该检查。
- 备选方案：
  - 方案 A：继续把 service struct、Command / Query / Result 和方法全部放在 `service.go`。
  - 方案 B：创建 `internal/service/<domain>/dto` 或 `contract` 子包承载输入输出。
  - 方案 C：按 Java 类结构为每个 service 拆 interface 和 impl。
  - 方案 D：在同一个 package 内按 `service.go`、`command.go`、`query.go`、`result.go`、use case 文件拆分。
- 为什么不选备选方案：
  - 不选方案 A：复杂模块中可读性和 code review 成本会变差。
  - 不选方案 B：子包会让同一业务用例内部类型跨包跳转，且容易误导为可被外层复用的 DTO；当前同 package 文件拆分足够。
  - 不选方案 C：Go 版不需要为单实现提前抽 interface；有真实替换点时再引入接口更自然。
  - 选择方案 D：保持 Go package 简洁，同时通过文件名表达职责，兼顾可读性和低样板成本。
- 后续可演进点：
  - auth/user/event/order 模块迁移时按该规范创建 service 文件。
  - 如果某个模块 use case 很多，可进一步按业务动作文件拆分，例如 `register.go`、`login.go`、`refresh_token.go`。
  - 如未来出现跨模块 service contract，可在新的设计文档中评估是否抽出更明确的端口接口。
