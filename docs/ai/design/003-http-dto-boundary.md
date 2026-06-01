# HTTP DTO 与 VO 边界规范设计

## 1. 背景
- 当前 Go 版 EventHub 已完成基础 HTTP 工程、项目 package layout 规范化，以及 system 端点的 `handler -> service` 初步落地。
- Java 版对应语义来源包括：
  - `backend/src/main/java/com/eventhub/modules/system/dto/request/EchoRequest.java`
  - `backend/src/main/java/com/eventhub/modules/system/vo/PingInfo.java`
  - `backend/src/main/java/com/eventhub/modules/system/vo/EchoInfo.java`
  - `backend/src/main/java/com/eventhub/modules/auth/dto/request/*.java`
  - `backend/src/main/java/com/eventhub/modules/auth/vo/*.java`
- Java 版常见 `VO` 命名可表达 View Object 或响应展示对象；Go 版如果照搬 `internal/http/vo`，会和 DDD Value Object 混淆。
- Go 版已经有 `internal/http/response` 承载统一 `APIResponse` envelope 和响应写出工具；具体业务 response DTO 如果放入该包，会混淆统一响应外壳与业务 data DTO 的职责。
- 本次任务不是大目录重构，也不新增 auth、user、repository、database、OpenAPI 或 Docker 等能力；重点是审计并统一 HTTP DTO / VO / response / domain value object 的边界。

## 2. 目标
- 明确本项目不设置 `internal/http/vo`，也不保留其他 HTTP VO 目录。
- 确认 HTTP request body、query 参数对象、path 参数辅助对象、HTTP response data、list item / summary / detail response 对象统一放 `internal/http/dto/<module>`。
- 确认 `internal/http/response` 只放统一响应 envelope 和 writer，例如 `APIResponse`、`Success`、`Failure`、`WriteSuccess`、`WriteError`、`WriteJSON`、`WriteStatus`。
- 确认 DDD Value Object 放 `internal/domain/<domain>` 或 `internal/domain/common`，不放 HTTP 层。
- 审计 `internal/http/handler` 是否仍存在内嵌 request/response struct，并在发现时迁移到 `internal/http/dto/<module>`。
- 审计 service 是否依赖 `internal/http/dto`，避免 HTTP 传输层类型向业务用例层泄漏。
- 保持现有 API 路径、JSON 字段名、响应体 shape、错误码、校验行为和测试语义不变。
- 成功标准：
  - 代码中没有 `internal/http/vo`、`internal/**/vo`、`*VO` HTTP 类型或误放在 `internal/http/response` 的具体业务 DTO。
  - system 端点 request/response 结构体位于 `internal/http/dto/system`。
  - service 使用 Command / Result 类型，不直接依赖 HTTP DTO。
  - `gofmt -w .`、`go test ./...`、`go vet ./...` 通过；如 Makefile 存在，`make fmt`、`make test`、`make vet` 通过。

## 3. 非目标
- 不实现 auth、user、event、reservation、order、ticket、payment、notification、audit 等新业务。
- 不新增数据库表、migration、sqlc query、repository/mysql、OpenAPI 契约或生成代码。
- 不改动现有 API 路径、HTTP 方法、状态码、统一响应字段、错误码或 validation 语义。
- 不创建空的 `internal/domain`、`internal/repository`、`internal/security` 等 Go package。
- 不为了文档完整性创建 `internal/http/vo` 或其他 VO 占位目录。
- 不把 Java/Spring 的 DTO/VO 包结构逐字翻译为 Go 包结构。

## 4. 影响范围
- 本次需要审计的 Go package / 模块：
  - `internal/http/handler`
  - `internal/http/dto`
  - `internal/http/response`
  - `internal/service`
  - `internal/domain`，如果存在
  - `internal/repository`，如果存在
  - `docs/ai/design`
  - `docs/ai/implementation`
  - `docs/ai/adr`
  - `docs/ai/parity`
- 当前审计结果：
  - `internal/http/vo` 不存在。
  - `internal/**/vo` 不存在。
  - 未发现文件名包含 `vo` 的 Go 文件。
  - 未发现类型名以 `VO` 结尾的 Go struct。
  - `internal/http/handler/system_handler.go` 只保留 handler、构造函数、HTTP 方法和 request validation，没有内嵌 HTTP request/response struct。
  - 2026-06-02 后，system 和 actuator HTTP DTO 已由 `internal/http/dto/system/request.go`、`internal/http/dto/system/response.go` 承载：`PingResponse`、`EchoRequest`、`EchoResponse`、`HealthResponse`、`InfoResponse`、`AppInfoResponse`、`RuntimeInfoResponse`。
  - `internal/http/response` 只包含统一响应 envelope、writer 和相关测试，没有具体业务 API response DTO。
  - `internal/service/system` 使用 `EchoCommand`、`PingResult`、`EchoResult`、`HealthResult`、`InfoResult` 等 service 层类型，没有依赖 `internal/http/dto`。
- 本次明确不触及的运行时代码目录：
  - `cmd`
  - `internal/app`
  - `internal/config`
  - `internal/platform`
  - `internal/http/router`
  - `internal/http/middleware`
  - `internal/http/validation`
  - `internal/apperror`
  - `internal/page`
  - `api/openapi`
  - `migrations`
  - `configs`
- 涉及 API / 表 / 缓存 / 外部接口：
  - API 路径、请求字段、响应字段、错误码、分页语义、OpenAPI 契约均不变化。
  - 不涉及数据库表、索引、migration、sqlc、缓存或外部接口。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 是。本次继续记录 Java `dto/request` 与 `vo` 命名习惯到 Go `internal/http/dto/<module>`、`internal/http/response` 和 `internal/domain` 的刻意结构差异，并补充本次审计结论索引。

## 5. 领域建模
- `HTTP DTO`：
  - HTTP 入参和出参的数据契约，包括 request body、query、path 参数辅助对象和 response data。
  - Go 版统一放入 `internal/http/dto/<module>`，用 `Request`、`Response`、`ListItemResponse`、`SummaryResponse`、`DetailResponse` 等后缀表达用途。
- `Response envelope`：
  - 统一 API 响应外壳，例如 `APIResponse`，稳定表达 `code`、`message`、`data`、`requestId`、`timestamp`。
  - 只属于 `internal/http/response`。
- `Response writer`：
  - 统一响应写出工具，例如 `WriteSuccess`、`WriteError`、`WriteJSON`、`WriteStatus`。
  - 只负责 HTTP 写出，不承载具体业务 response 字段。
- `Domain model`：
  - 表达业务实体、聚合状态或业务枚举，不承担 HTTP JSON 契约。
- `Domain Value Object`：
  - DDD 意义上的值对象，例如未来的 `Email`、`Username`、`Money`、`OrderNo`。
  - 放入 `internal/domain/<domain>` 或 `internal/domain/common`，不放 HTTP 层。
- `Service Command / Query / Result`：
  - service 用例输入和输出，不依赖 `internal/http/dto`。
  - 当前 system service 已使用 `EchoCommand` 和 result 类型承接 handler 映射。
- 与 Java 版领域对象的对应关系：
  - Java `modules/system/dto/request/EchoRequest` -> Go `internal/http/dto/system.EchoRequest`。
  - Java `modules/system/vo/PingInfo` -> Go `internal/http/dto/system.PingResponse`。
  - Java `modules/system/vo/EchoInfo` -> Go `internal/http/dto/system.EchoResponse`。
  - Java `modules/auth/vo/LoginResponse`、`TokenPairResponse`、`UserInfo` 等未来迁移时，应落到 Go `internal/http/dto/auth` 中的 `LoginResponse`、`TokenPairResponse`、`CurrentUserResponse` 或更明确的响应类型，而不是 `internal/http/vo`。

## 6. API 设计
- 本次不新增或修改运行时 API。
- 既有 API 保持：
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET /actuator/health`
  - `HEAD /actuator/health`
  - `GET /actuator/info`
  - `HEAD /actuator/info`
- HTTP DTO 命名规则：
  - 请求体：`RegisterRequest`、`LoginRequest`、`RefreshRequest`、`UpdateUserStatusRequest`、`CreateEventRequest`。
  - 响应 data：`RegisterResponse`、`LoginResponse`、`CurrentUserResponse`、`EventDetailResponse`、`ReservationResponse`。
  - 列表项：`AdminUserListItemResponse`、`EventListItemResponse`。
  - 摘要：`UserSummaryResponse`、`EventSummaryResponse`。
  - 详情：`UserDetailResponse`、`EventDetailResponse`。
  - 不推荐 `XxxVO`、`XxxDTO`、`XxxView`、`XxxResp`，除非外部生成代码或兼容需求明确要求。
- 错误码 / 异常场景：
  - 本次不改变错误码。
  - echo JSON 解析失败仍返回 HTTP 400、`COMMON-400`、`message=请求体格式不合法`。
  - echo 字段校验失败仍返回 HTTP 400、`COMMON-400`、`message=请求体参数校验失败`，字段错误仍放入 `data`。
  - `internal/http/response` 继续负责统一成功 / 失败响应写出。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Go 版对齐路径、字段、状态码、错误码和状态语义，但不逐字照搬 Java `VO` 包名。
  - Java View Object 语义在 Go 版归入 HTTP DTO；DDD Value Object 语义在 Go 版归入 domain。

## 7. 数据设计
- 本次不新增数据库表、索引、唯一约束、migration 或 sqlc query。
- 本次不修改 `sqlc.yaml`，也不运行 `sqlc generate`。
- 规则层面约束：
  - sqlc generated model 不能作为 HTTP DTO 对外暴露。
  - `repository/mysql` 负责 sqlc row 与 domain model 的映射。
  - domain model 不因为 HTTP JSON 输出需要而直接携带传输层职责。
- 数据一致性考虑：
  - 本次不涉及事务、锁、状态机、库存扣减或持久化一致性。

## 8. 关键流程
- 审计流程：
  1. 检查 `internal/http/vo`、`internal/**/vo`、文件名包含 `vo` 的 Go 文件、类型名以 `VO` 结尾的 struct。
  2. 检查 `internal/http/handler` 是否定义 HTTP request/response struct。
  3. 检查 `internal/http/response` 是否包含具体业务 API response DTO。
  4. 检查 service 是否直接 import `internal/http/dto`。
  5. 检查 domain 或 repository 是否被迫承担 HTTP JSON 或 sqlc-to-handler 泄漏职责。
- 当前代码调整计划：
  - 若审计继续保持当前结果，则不移动 Go 代码，只更新文档、ADR 和 parity matrix，记录已审计状态。
  - 若后续新增业务 DTO，必须放入 `internal/http/dto/<module>`，handler 负责 DTO 与 service 类型映射。
  - 若发现具体业务 response 被放入 `internal/http/response`，迁移到 `internal/http/dto/<module>` 并更新 import。
  - 若发现 `VO` 类型代表 HTTP 展示对象，迁移到 `internal/http/dto/<module>` 并重命名为 `XxxResponse`、`XxxListItemResponse`、`XxxSummaryResponse` 或 `XxxDetailResponse`。
  - 若发现 `VO` 类型代表 DDD Value Object，迁移到 `internal/domain/<domain>` 或 `internal/domain/common`，并改名为明确业务名。
- 正常运行流程保持：
  1. handler decode 并 validate HTTP DTO。
  2. handler 将 DTO 映射为 service Command / Query。
  3. service 执行业务规则、事务边界和状态决策。
  4. repository/mysql 将 sqlc row 映射为 domain model。
  5. service 返回业务结果或 domain model。
  6. handler 将结果映射为 HTTP response DTO。
  7. handler 调用 `response.WriteSuccess`、`response.WriteError` 或 actuator 场景下的 `response.WriteJSON` / `WriteStatus`。
- 状态流转：
  - `VO 命名歧义 -> DTO/Value Object 设计明确 -> 运行时代码审计 -> ADR/parity 更新 -> 后续业务按 DTO boundary check 执行`。
- handler / service / repository / sqlc/database 分工：
  - handler：HTTP DTO、校验、映射、响应写出。
  - service：业务规则、事务边界、Command / Query / Result。
  - repository：持久化语义接口。
  - repository/mysql：sqlc row 与 domain model 映射。
  - sqlc/database：生成查询代码，不承载业务判断。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及运行时并发、幂等或缓存。
- DTO 不承载并发、幂等或缓存策略。
- 幂等、防重复提交、事务边界和缓存策略由 service 设计承载。
- 缓存返回结构如需对外暴露，也必须先映射为 HTTP DTO。

## 10. 权限与安全
- 本次不实现认证或授权代码。
- HTTP DTO 可以表达请求或响应中的安全相关字段，但 JWT claim、Principal、安全上下文属于 `internal/security`。
- 不因为 response DTO 需要就把角色、邮箱、用户名、用户状态写入 JWT。
- domain model 和 domain value object 不承担 HTTP JSON 契约职责，避免敏感字段被误暴露。
- 不涉及敏感信息、审计或操作日志实现。

## 11. 测试策略
- 审计验证：
  - `find internal -path '*/vo' -o -path '*/vo/*' -o -iname '*vo*.go' -print`
  - `rg -n "\b[A-Za-z0-9_]*VO\b|internal/http/vo|/vo" internal docs api cmd .agents AGENTS.md || true`
  - `rg -n "^type\s+\w+\s+struct|internal/http/dto|http/dto|dto\." internal cmd api docs --glob '*.go'`
  - `rg -n "internal/http/dto" internal/service internal/domain internal/repository || true`
- 质量门禁：
  - `gofmt -w .`
  - `go test ./...`
  - `go vet ./...`
  - 如果 Makefile 存在，运行 `make fmt`、`make test`、`make vet`。
  - 如果已配置且工具可用，运行 `golangci-lint run`；不可用时说明原因。
- 单元测试：
  - 不新增业务逻辑时不新增测试；保留现有 HTTP、response、apperror、page、idgen 测试。
- service / repository 测试：
  - 当前只审计 system service 与 DTO 边界；repository 不适用。
- migration / sqlc 验证：
  - 不适用，本次没有 SQL、schema、sqlc 或 migration 变化。
- 接口验证 / OpenAPI validate：
  - 不适用，本次没有 OpenAPI 契约变化。
- 异常场景验证：
  - 通过现有 handler 测试继续覆盖 JSON 解析失败、字段校验失败、not found、panic recover、actuator HEAD 无 body 等场景。
- Java-Go parity 验证：
  - 对照 Java `modules/system/dto/request`、`modules/system/vo` 和 `modules/auth/vo` 命名习惯，确认 Go 版记录为 HTTP DTO / domain Value Object 两类边界。

## 12. 风险与替代方案
- 当前方案的风险：
  - 当前业务 DTO 数量仍少，规则主要通过 system 端点和文档约束验证；后续 auth/user/event 阶段仍需持续检查。
  - 如果未来 OpenAPI 生成代码引入 `DTO` 后缀，可能需要在设计文档中说明兼容例外。
  - 如果 domain model 为内部序列化临时携带 `json` tag，需要额外说明，避免被误认为 HTTP DTO。
- 备选方案：
  - 方案 A：创建 `internal/http/vo` 放响应展示对象。
  - 方案 B：拆分 `internal/http/request` 和 `internal/http/response` 存业务请求/响应。
  - 方案 C：让 service 直接复用 HTTP DTO，减少映射代码。
  - 方案 D：请求和响应结构体统一放 `internal/http/dto`。
- 为什么不选备选方案：
  - 不选方案 A：`VO` 同时可能表示 View Object 和 Value Object，长期容易混淆。
  - 不选方案 B：`internal/http/response` 已用于统一响应 envelope 和 writer，继续放业务 response 会让职责不清。
  - 不选方案 C：service 依赖 HTTP DTO 会破坏 handler/service 边界，不利于后续复用和测试。
  - 选择方案 D：HTTP 传输契约集中管理，命名通过后缀表达用途，同时保持 domain / service 不依赖 HTTP。
- 后续可演进点：
  - 新增 auth/user/event/order DTO 时，implementation note 必须写明 DTO 与 service command/result/domain model 的映射关系。
  - 新增 OpenAPI schema 后，继续验证 schema 名称与 DTO 命名约定是否一致。
  - 新增 DDD Value Object 时，优先放入 domain，并避免携带 HTTP JSON 契约职责。
