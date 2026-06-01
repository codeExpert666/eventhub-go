# HTTP handler / DTO 模块化组织设计

## 1. 背景
- 当前 Go 版 EventHub 已建立 `handler -> service -> repository -> sqlc/database` 分层，并已通过 `internal/service/system` 的文件拆分固化 service package 内部边界。
- 当前 HTTP 层仍采用较扁平的组织方式：
  - `internal/http/handler/system_handler.go`
  - `internal/http/dto/system_dto.go`
- system 模块很小，所以 request 和 response 放在一个 DTO 文件中尚可阅读；但后续 auth、user、event、order、ticket、payment 等模块会产生更多 request、response、list item、summary、detail DTO，以及多个 handler use case。如果继续把所有 handler 文件直接放在 `internal/http/handler` 下、所有 DTO 文件直接放在 `internal/http/dto` 下，会降低目录扫描、命名和 code review 的清晰度。
- Java 版对应来源仍以 Controller、request DTO、VO / response 对象为语义参考；Go 版不复制 Java 包名，而是用 Go package 边界表达模块归属和传输层职责。

## 2. 目标
- 制定 HTTP handler / DTO 的模块化组织规则：
  - 正式业务模块默认使用 `internal/http/handler/<module>` 子包。
  - 正式业务模块默认使用 `internal/http/dto/<module>` 子包。
  - DTO 子包内优先拆分 `request.go` 和 `response.go`；复杂模块可继续按 use case 拆分。
  - handler 子包内优先保留 `handler.go` 放 handler struct、constructor 和依赖字段；复杂模块按 use case 拆文件。
- 更新持久规则：
  - `AGENTS.md` 记录长期目录规则。
  - `.agents/skills/backend-design-first/SKILL.md` 增加实现前检查项。
  - ADR、implementation note 和 parity matrix 记录本次结构决策。
- 审视现有代码并将 system HTTP 层迁到新结构，作为后续模块的最小样板。
- 保持现有 API 路径、HTTP 方法、JSON 字段、状态码、错误码、统一响应 envelope、handler/service 映射和测试语义不变。
- 成功标准：
  - `system` handler 位于 `internal/http/handler/system`。
  - `system` DTO 位于 `internal/http/dto/system`，且 request / response 分文件。
  - router 使用明确 import alias，例如 `systemhandler`。
  - service 仍不依赖 HTTP DTO。
  - `gofmt`、`go test ./...`、`go vet ./...` 和 Makefile 对应命令通过。

## 3. 非目标
- 不新增 auth、user、event、order、ticket、payment 等业务能力。
- 不改变 system API 契约、校验规则、错误响应、actuator 行为或统一响应结构。
- 不新增数据库表、migration、sqlc query、repository/mysql 或 OpenAPI 契约。
- 不创建空业务模块目录或空 Go package。
- 不引入 `internal/http/vo`，也不把 DTO 放入 `internal/http/response`。
- 不把 service Command / Query / Result 改成 HTTP DTO。

## 4. 影响范围
- 本次触及的 Go package / 模块：
  - `internal/http/router`
  - `internal/http/handler/system`
  - `internal/http/dto/system`
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
  - `internal/http/middleware`
  - `internal/http/response`
  - `internal/http/validation`
  - `internal/apperror`
  - `internal/page`
  - `internal/domain`
  - `internal/service`
  - `internal/repository`
  - `internal/security`
  - `api/openapi`
  - `migrations`
  - `configs`
- 涉及 API / 表 / 缓存 / 外部接口：
  - API 路径、请求字段、响应字段、状态码、错误码、分页语义和 OpenAPI 契约均不变化。
  - 不涉及数据库、缓存或外部接口。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 是。本次属于 Go 版为了长期可读性和模块边界清晰，对 Java Controller / DTO / VO 结构做出的 Go idiom 包组织差异，需要记录。

## 5. 领域建模
- `HTTP module`：
  - 一个 HTTP 传输层业务模块，例如 `system`、`auth`、`user`、`event`、`order`。
  - 与 service/domain 模块名保持语义一致，但 package 边界仍属于 HTTP 层。
- `Handler package`：
  - 路径为 `internal/http/handler/<module>`。
  - 包内 handler 负责 decode、validate、调用 service、映射 result、写出 response。
  - 子包内类型可命名为 `Handler`，由调用方通过 import alias 表达模块，例如 `systemhandler.NewHandler`。
- `DTO package`：
  - 路径为 `internal/http/dto/<module>`。
  - 承载本模块 HTTP request、query/path 参数辅助对象和 response data。
  - 子包内 request / response 分文件；没有 request 或没有 response 时不创建空文件。
- 与 Java 版领域对象的对应关系：
  - Java `SystemController` -> Go `internal/http/handler/system.Handler`。
  - Java `modules/system/dto/request/EchoRequest` -> Go `internal/http/dto/system.EchoRequest`。
  - Java `modules/system/vo/PingInfo`、`EchoInfo` -> Go `internal/http/dto/system.PingResponse`、`EchoResponse`。

## 6. API 设计
- 本次不新增或修改运行时 API。
- 既有 API 保持：
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET /actuator/health`
  - `HEAD /actuator/health`
  - `GET /actuator/info`
  - `HEAD /actuator/info`
- HTTP DTO 命名规则保持：
  - 请求体：`XxxRequest`
  - 响应 data：`XxxResponse`
  - 列表项：`XxxListItemResponse`
  - 摘要：`XxxSummaryResponse`
  - 详情：`XxxDetailResponse`
- 模块化文件规则：
  - `internal/http/dto/<module>/request.go` 放请求体、query 参数和 path 参数辅助对象。
  - `internal/http/dto/<module>/response.go` 放 response data、list item、summary、detail 响应对象。
  - DTO 数量很多时，可按 use case 拆为 `login_request.go`、`login_response.go`、`admin_user_response.go` 等，但仍留在同一 `<module>` 子包。
- 错误码 / 异常场景：
  - 本次不改变错误码。
  - echo JSON 解析和字段校验行为保持不变。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - 无 API 契约差异；仅 Go HTTP 层 package 和文件组织不同。

## 7. 数据设计
- 本次不新增数据库表、索引、唯一约束、migration 或 sqlc query。
- 本次不修改 `sqlc.yaml`，也不运行 `sqlc generate`。
- 数据一致性考虑：
  - 不涉及事务、锁、库存扣减、状态机或持久化一致性。

## 8. 关键流程
- 正常运行流程保持：
  1. router 装配 `systemhandler.NewHandler(systemService)`。
  2. handler decode 并 validate `systemdto.EchoRequest`。
  3. handler 将 HTTP DTO 映射为 `systemsvc.EchoCommand`。
  4. service 返回 result。
  5. handler 将 result 映射为 `systemdto.*Response`。
  6. handler 调用 `response.WriteSuccess`、`response.WriteError`、`response.WriteJSON` 或 `response.WriteStatus`。
- 本次结构调整流程：
  1. `internal/http/handler/system_handler.go` 迁移为 `internal/http/handler/system/handler.go`。
  2. handler package 从 `handler` 改为 `system`，对外构造函数改为 `NewHandler`。
  3. `internal/http/dto/system_dto.go` 拆为 `internal/http/dto/system/request.go` 和 `response.go`。
  4. `internal/http/router.go` 使用 `systemhandler` import alias。
- handler / service / repository / sqlc/database 分工：
  - handler：HTTP DTO、校验、映射、响应写出。
  - service：Command / Query / Result、业务规则、事务边界、幂等、状态流转。
  - repository：持久化语义接口。
  - repository/mysql：sqlc row 与 domain model 映射。
  - sqlc/database：生成查询代码，不承载业务判断。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及运行时并发、幂等或缓存。
- 规范层面明确：
  - 幂等键、防重复提交参数、库存扣减输入等可以从 HTTP DTO 映射到 service Command / Query，但幂等和事务决策必须留在 service。
  - 缓存命中结果如果对外返回，也必须映射为对应模块的 HTTP response DTO。

## 10. 权限与安全
- 本次不实现认证或授权。
- 规范层面明确：
  - handler 子包可读取认证上下文并映射为 service Command / Query 所需的 principal 或用户 ID。
  - DTO 子包不承担 JWT claim 或 security principal 职责。
  - 不把角色、邮箱、用户名、用户状态写入 JWT。

## 11. 测试策略
- 单元测试：
  - 本次不改变业务逻辑，不新增单元测试；通过现有 HTTP router 测试覆盖 system 行为。
- service / repository 测试：
  - service 代码不变。
  - repository 不适用。
- migration / sqlc 验证：
  - 不适用，本次没有 SQL、schema、sqlc 或 migration 变化。
- 接口验证 / OpenAPI validate：
  - 不适用，本次没有 OpenAPI 契约变化。
- 异常场景验证：
  - 现有 handler/router 测试继续覆盖 JSON 解析失败、字段校验失败、not found、panic recover、actuator HEAD 无 body 等场景。
- Java-Go parity 验证：
  - 对照 Java system Controller / DTO / VO，确认 Go 版仅改变 package 组织，保留 API 契约和 handler/service 分工。
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
  - 对 system 这种小模块来说，目录层级比原先更深；但它能作为后续复杂模块的统一样板。
  - `handler/system`、`dto/system` 和 `service/system` 都叫 `system`，调用处需要 import alias 保持可读性。
  - 如果未来每个 request/response 都机械拆独立文件，会造成文件碎片化；应先按 `request.go` / `response.go`，复杂后再按 use case 拆。
- 备选方案：
  - 方案 A：继续保持 `internal/http/handler/*.go` 和 `internal/http/dto/*.go` 扁平目录。
  - 方案 B：拆为 `internal/http/request/<module>` 和 `internal/http/response/<module>`。
  - 方案 C：采用纵向模块目录，例如 `internal/modules/system/{handler,dto,service}`。
  - 方案 D：采用当前混合方案，在 HTTP 层内部按模块子包拆分。
- 为什么不选备选方案：
  - 不选方案 A：复杂模块增多后目录扫描和类型命名会变差。
  - 不选方案 B：`internal/http/response` 已用于统一 response envelope 和 writer，继续把业务 response 放进去容易混淆职责。
  - 不选方案 C：当前仓库已经通过 ADR 固化混合式 package layout，全面转向纵向 modules 会带来更大结构变更。
  - 选择方案 D：保持现有分层，同时在 HTTP 层内部获得模块边界，改动面小且更适合当前阶段。
- 后续可演进点：
  - auth/user/event/order 模块新增时直接创建对应 handler/dto 子包。
  - 如果某个模块 use case 很多，可在子包内继续按 use case 拆 handler 和 DTO 文件。
  - OpenAPI schema 引入后，需要确认 schema 与 DTO 子包命名能够稳定映射。
