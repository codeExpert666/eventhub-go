# OpenAPI Strict Server Router Migration

## 1. 背景
- Go 版已经采用 spec-first OpenAPI，`api/openapi/eventhub.yaml` 是业务 API 与 actuator API 的契约源。
- 当前 `api/openapi/oapi-codegen.yaml` 只生成 `models` 和 `chi-server`，运行时 `internal/http/router.go` 仍手写注册 `/api/**` 与 `/actuator/**` 路由。
- 为降低手写 router 与 OpenAPI paths/methods 漂移风险，先前新增了 `internal/http/router_contract_test.go` 和 `internal/http/openapi_contract_test.go`。这些测试是补救型门禁，不改变运行时事实来源。
- 本次迁移目标是让运行时 router 本身由 `oapi-codegen` generated chi strict server wrapper 注册 OpenAPI 声明的业务与 actuator routes。
- Java 版来源主要是 Spring MVC controller mapping、Springdoc 生成契约、Actuator 风格 endpoint 和既有 API 业务语义。Go 版不复刻 Springdoc 注解扫描，而是用 `eventhub.yaml` 驱动生成代码和运行时路由。

## 2. 目标
- 将 `api/openapi/oapi-codegen.yaml` 升级为生产生成配置，继续输出 `api/openapi/gen/eventhub.gen.go`，开启：
  - `models`
  - `chi-server`
  - `strict-server`
- 不新增 `api/openapi/genstrict`，不保留实验 strict 配置。
- `internal/http/router.go` 不再手写注册 `api/openapi/eventhub.yaml` 中声明的业务 API 和 actuator API path/method。
- 使用 generated `NewStrictHandler` + chi server wrapper 注册 OpenAPI 声明 routes。
- 保留 router 的外围职责：
  - 全局 request id / recover middleware 编排。
  - OpenAPI / Swagger 静态资源路由，继续受 `OPENAPI_ENABLED` 控制。
  - `NotFound` / `MethodNotAllowed` 统一错误响应。
  - OpenAPI security 对应的认证与 ADMIN 授权 middleware 编排。
- 将 strict-server route registration、OpenAPI security middleware 和 generated error handler 适配集中到 `internal/http/openapi_routes.go`，避免 `router.go` 主流程同时承担过多生成代码细节。
- 将实现 `gen.StrictServerInterface` 的生产聚合适配器放到 `internal/http/openapi_adapter.go`，不再新增 `internal/http/handler/api` 这类非业务 handler 子包。
- 清理各业务 handler 子包里旧的 direct `net/http` 入口方法，只保留 strict-server 生产链路仍需要的 `strict.go`、constructor 与校验/解析 helper。
- 保留 `internal/http/handler/<module>` 业务子包边界，不把 auth/system/user handler 摊平到 `internal/http/handler` 根目录。
- 直接使用 strict server 已生成且适用于本项目的 request/response model，不再在 `internal/http/dto/<module>` 保留镜像 HTTP DTO。
- 删除项目自有 `response.APIResponse`/`Success`/`Failure`/`WriteSuccess`/`WriteJSON`/`WriteStatus`，成功响应由 generated typed response 表达 envelope，`internal/http/response` 仅保留公共 meta 与错误写出能力。
- 将 `AppError` 的错误上下文从 `data any` 收敛为 `details Details`，统一通过 generated `ErrorResponse.data` 输出结构化错误详情。
- 保持 handler -> service -> repository -> sqlc/database 分层，service 不依赖 `api/openapi/gen` 或 `internal/http/dto`。
- 删除旧补救型测试：
  - `internal/http/router_contract_test.go`
  - `internal/http/openapi_contract_test.go`
- 保留并强化 OpenAPI gates：
  - `openapi-lint`
  - `openapi-validate`
  - `openapi-check`
  - `openapi-breaking-check`
  - `api/openapi/openapi_policy_test.go`

## 3. 非目标
- 不新增编译期 adapter 或只做覆盖检查的中间层。
- 不保留 genstrict 实验目录，不做双链路评估。
- 不修改 `api/openapi/eventhub.yaml` 的 API 路径、字段、状态码或分页语义。
- 不把 strict-server 当作完整 incoming request validation。DTO/service 层已有字段校验、错误码和中文错误消息需要保持等价。
- 不让 service、domain、repository 或 sqlc generated model 依赖 OpenAPI generated model。
- 不保留与 generated request/response model 同形的 HTTP DTO；如果未来某个非 OpenAPI HTTP 面确需自定义 DTO，需要单独设计说明。
- 不删除业务 handler constructor，也不删除 strict handler 仍复用的字段校验、query/path 参数解析和 principal 读取 helper。
- 不把所有业务 handler 合并成根 package；这会违反当前 HTTP handler 模块化组织约定，并引入 `Handler`、映射 helper 等命名冲突。
- 不调整数据库表、migration、sqlc query、Redis、JWT claim 或 refresh token 业务语义。
- 不改变 OpenAPI / Swagger docs 路由的 `OPENAPI_ENABLED` 行为；文档路由仍不是 strict-server 业务 API 范围。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `api/openapi/oapi-codegen.yaml`
  - `api/openapi/gen/eventhub.gen.go`
  - `internal/http/router.go`
  - `internal/http/openapi_routes.go`
  - `internal/http/openapi_adapter.go`
  - `internal/http/handler/system`
  - `internal/http/handler/auth`
  - `internal/http/handler/user`
  - `internal/http/response`
  - `internal/http/requesterror`
  - `internal/apperror`
  - 删除旧 direct `net/http` handler 文件：
    - `internal/http/handler/auth/{register,login,refresh,logout,mapping}.go`
    - `internal/http/handler/system/handler.go` 中的 direct method，保留 constructor
    - `internal/http/handler/user/{me,admin_users,mapping}.go`
  - 新增或保留 shared helper 文件：
    - `internal/http/handler/system/validation.go`
    - `internal/http/handler/auth/validation.go`
    - `internal/http/handler/user/admin_validation.go`
  - 删除旧镜像 HTTP DTO 文件：
    - `internal/http/dto/system/{request,response}.go`
    - `internal/http/dto/auth/{request,response}.go`
    - `internal/http/dto/user/{request,response}.go`
  - 删除旧 response envelope 文件：
    - `internal/http/response/api_response.go`
  - `internal/app/providers/http.go`
  - `internal/http/router_test.go`
  - `internal/http/auth_integration_test.go`
  - `docs/ai/design/029-openapi-strict-server-router-migration.md`
  - `docs/ai/implementation/029-openapi-strict-server-router-migration.md`
  - `docs/ai/adr/0025-openapi-strict-server-runtime-router.md`
  - `docs/ai/parity/java-go-parity-matrix.md`
- 涉及 API / 表 / 缓存 / 外部接口：
  - API path/method 来源改变，但对外契约不改变。
  - 不涉及表、索引、migration、sqlc、缓存或外部接口。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 影响。router source-of-truth、generated strict server、测试策略和 OpenAPI gate 边界发生变化，必须更新 parity matrix。

## 5. 领域建模
- `OpenAPISpec`
  - 物理文件：`api/openapi/eventhub.yaml`。
  - 继续是业务与 actuator API path/method 的事实来源。
- `GeneratedStrictServer`
  - 物理文件：`api/openapi/gen/eventhub.gen.go`。
  - 由 `oapi-codegen v2.5.0` 根据 `models + chi-server + strict-server` 生成。
  - 提供 `StrictServerInterface`、typed request/response object、`NewStrictHandler` 和 chi registration wrapper。
- `OpenAPIAdapter`
  - 生产代码中的 strict server 实现。
  - 物理文件：`internal/http/openapi_adapter.go`。
  - 私有类型 `openAPIAdapter` 聚合 system/auth/user handler，并实现 `gen.StrictServerInterface`。
  - 属于 `internal/http` 的生成接口适配层，不下沉到 service/domain/repository，也不作为业务 handler 子包存在。
- `OpenAPISecurityMiddleware`
  - router 层外围 middleware。
  - 利用 generated wrapper 对 BearerAuth route 写入的 security context，复用现有 `Authenticate` middleware。
  - admin path 继续叠加 `RequireRole("ADMIN")`，语义对齐 OpenAPI `x-required-roles: [ADMIN]` 和现有 RBAC。
- `OpenAPIRouteAdapter`
  - 物理文件：`internal/http/openapi_routes.go`。
  - 只负责把 generated strict server wrapper 接入 chi router，并适配本项目统一错误 envelope、认证 middleware 和 ADMIN 授权语义。
  - 不承载业务 handler 装配之外的 service/repository 规则。
- `BusinessModuleHandler`
  - 业务 handler 仍按 `internal/http/handler/auth`、`internal/http/handler/system`、`internal/http/handler/user` 子包组织。
  - strict-server 接入后，生产 HTTP 入口统一落到各模块的 `strict.go`。
  - `handler.go` 只保留 `Handler` struct、constructor 和少量跨 strict 方法共享的上下文 helper。
  - 仍被 strict 方法使用的 HTTP 字段校验、query/path 解析保留在模块内校验文件，避免把校验逻辑塞进 router 或 adapter。
- `ResponseHelpers`
  - 物理文件：`internal/http/response/response.go`。
  - `SuccessMeta(ctx)` 是成功响应侧唯一公共 meta helper，只生成 `code/message/requestId/timestamp`，不携带 `data any`。
  - 各 strict handler 使用 generated `ApiResponseXxx` 填充 typed `Data`，避免项目自有 envelope 与 generated envelope 双轨维护。
  - `WriteError` 仍是 router、middleware、docs handler 的统一错误写出入口。
  - 错误 body 在 `WriteError` 内部通过私有 helper 构造 generated `openapigen.ErrorResponse`，`AppError.Details` 映射到 `ErrorResponse.Data`。
  - `ErrorMeta` 和 `ErrorBody` 不作为外部 API 暴露，避免 response 包公共面超过实际使用场景。
- `AppErrorDetails`
  - 物理文件：`internal/apperror/error.go`。
  - `Details map[string]any` 只表达结构化错误上下文，替代旧 `data any`。
- 与 Java 版领域对象的对应关系：
  - Java controller mapping + Springdoc operation 对应 Go `eventhub.yaml` + generated strict server route registration。
  - Java Actuator 风格 endpoint 对应 Go `/actuator/health` 与 `/actuator/info`。

## 6. API 设计
- 对外 API 列表不变，由 `eventhub.yaml` 声明：
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout`
  - `GET /api/v1/me`
  - `GET /api/v1/admin/users`
  - `PATCH /api/v1/admin/users/{userId}/status`
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET /actuator/health`
  - `HEAD /actuator/health`
  - `GET /actuator/info`
  - `HEAD /actuator/info`
- 请求参数：
  - path/query 参数由 generated chi wrapper 绑定到 strict request object。
  - JSON body 由 generated strict handler 解码到 generated request body type。
  - handler 继续执行业务字段校验，并把 generated request object 直接映射到 service Command / Query。
  - 旧 direct `net/http` 方法被删除后，不再存在第二套 `DecodeJSONBody` 入口；字段级业务校验 helper 仍复用，保持错误码和中文消息不变。
- 响应结构：
  - 成功响应返回 generated typed response object，例如 `gen.Ping200JSONResponse`、`gen.Login200JSONResponse`。
  - 成功 response envelope 由 generated `ApiResponseXxx` 表达，公共 `code/message/requestId/timestamp` 来自 `response.SuccessMeta(ctx)`。
  - 错误响应统一通过 strict server `ResponseErrorHandlerFunc` 和 request decode error handler 写回 generated `ErrorResponse` envelope。
  - actuator HEAD 使用 generated no-body response object。
- 错误码 / 异常场景：
  - 请求体 JSON 解码失败继续映射为 `COMMON-400` / `请求体格式不合法`。
  - query/path 绑定失败继续映射为 `COMMON-400` / `请求参数校验失败`。
  - 缺失或无效 token 继续映射为 `AUTH-401`。
  - 普通用户访问 admin route 继续映射为 `AUTH-403`。
  - handler/service 业务错误继续由 `apperror.FromErrorOrInternal` 收敛，已有 app error 保留原错误码，普通 error 包装为 `COMMON-500`。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Go 不使用注解扫描，仍以 YAML 为契约源。
  - Go 的 runtime route registration 从手写 chi route list 改为 generated chi wrapper。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 本次只改变 HTTP 路由注册和 handler request/response 映射，不改变事务边界、repository 行为或 sqlc generated model。

## 8. 关键流程
- 正常流程：
  1. `make openapi-generate` 读取 `api/openapi/oapi-codegen.yaml` 和 `api/openapi/eventhub.yaml`。
  2. 生成 `api/openapi/gen/eventhub.gen.go`，包含 strict server types 和 chi wrapper。
  3. `ProviderHTTP` 创建 docs handler，并把 system/auth/user handler 与 auth middleware 传入 `internal/http.NewRouter`。
  4. `NewRouter` 创建 chi router，注册全局 request id / recover middleware。
  5. `NewRouter` 调用 `registerOpenAPIRoutes` 接入 generated strict server。
  6. `registerOpenAPIRoutes` 创建生产 `openAPIAdapter`，再调用 `gen.NewStrictHandlerWithOptions` 和 generated chi `HandlerWithOptions` 注册 `eventhub.yaml` 声明的业务与 actuator routes。
  7. docs 路由在 `deps.OpenAPI != nil` 时继续手写注册 `/openapi.yaml` 和 `/swagger/*`。
  8. `NotFound` / `MethodNotAllowed` 继续统一映射为 `COMMON-404`。
- 异常流程：
  - request body JSON 解码失败：strict request error handler 写出 `COMMON-400`。
  - query/path 参数格式错误：generated chi wrapper error handler 写出 `COMMON-400`。
  - 受保护 route 缺少认证 middleware 能力：写出 `COMMON-404`，保留未配置数据库时不暴露业务能力的降级语义。
  - 某模块 handler 未装配但 route 已由 generated wrapper 注册：模块 server 方法返回 `COMMON-404`。
- 状态流转：
  - 不涉及业务状态机变化。
- handler / service / repository / sqlc/database 分工：
  - `router.go`：全局中间件编排、strict route adapter 接入、docs route、NotFound。
  - `openapi_routes.go`：generated route registration、BearerAuth/Admin middleware 适配、request body / response / 参数错误映射。
  - `openapi_adapter.go`：strict request/response object 与模块 handler 的聚合边界，实现 generated `StrictServerInterface`。
  - module handler：generated request body/query/path 到 service Command / Query / Result 的映射、字段校验和 generated 响应模型映射；旧 direct `net/http` 入口清理后，各模块生产入口集中在 `strict.go`。
  - `response`：只公开成功 meta 与统一错误写出入口，并为 middleware/router/docs handler 写出 generated `ErrorResponse`；错误响应构造细节保持包内私有，不再定义项目自有 `APIResponse`。
  - `apperror`：只保存稳定错误码、用户消息、`Details` 和 cause，不再接受任意 `data any`。
  - service：业务规则、事务边界、幂等和状态决策，不依赖 generated model。
  - repository/sqlc：不受影响。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无。
- 如何防重复提交：不涉及订单、库存或支付写入。
- 事务边界在哪里：不改变现有 service/repository 事务边界。
- 缓存放在哪里，为什么：不新增缓存。
- 并发考虑：
  - generated strict server 和 router 都只持有启动期依赖，不引入共享可变状态。

## 10. 权限与安全
- 哪些角色能访问：
  - public：register/login/refresh/system/actuator。
  - authenticated：logout/me。
  - admin：admin user list/update status。
- 鉴权与鉴别约束：
  - 继续复用 `middleware.Authenticate`，按 JWT `sub` 读取最新 principal。
  - admin route 继续叠加 `middleware.RequireRole("ADMIN")`。
  - 认证/授权 middleware 运行在 strict body decode 前，避免未认证请求因 body 格式先返回 `COMMON-400`。
- JWT claim 边界：
  - 不新增角色、邮箱、用户名、用户状态等动态 claim。
- 是否涉及敏感信息、审计或操作日志：
  - 不新增敏感字段输出或审计日志。

## 11. 测试策略
- 单元测试：
  - 新增或调整 `internal/http/router_test.go`，证明 OpenAPI 声明 route 由 generated strict server 注册，而不是依赖模块 handler 是否非空才注册。
  - 调整 `internal/apperror/error_test.go` 与 `internal/http/response/response_test.go`，锁定 `Details`、`SuccessMeta`、`WriteError` 的 generated `ErrorResponse` 行为，以及 response 包公共 API 面。
- service / repository 测试：
  - 既有 service/repository 测试继续覆盖业务规则；本次不新增数据库测试。
- migration / sqlc 验证：
  - 不适用；本次没有 SQL、schema 或 migration 变化。
- 接口验证：
  - 保留现有 router/auth integration 测试，验证 envelope、错误码、auth、RBAC、分页和 actuator 行为不变。
  - 通过编译和 HTTP/router 测试确认删除旧 direct 方法后，生产 strict route 仍覆盖所有 OpenAPI 声明 endpoint。
- OpenAPI validate：
  - `make openapi-lint`
  - `make openapi-check`
  - `make openapi-breaking-check`
- 异常场景验证：
  - malformed JSON、query/path 参数格式错误、缺失 token、普通用户访问 admin、未启用 docs route。
- Java-Go parity 验证：
  - 更新 parity matrix，记录 Go router source-of-truth 改为 generated strict server，旧补救型 tests 被 runtime source-of-truth 替代。
- 需要运行的命令：
  - `gofmt`
  - `git diff --check`
  - `go test ./...`
  - `go vet ./...`
  - `make openapi-lint`
  - `make openapi-check`
  - `make openapi-breaking-check`
  - `make quality-check`
  - `golangci-lint run` 或仓库 `make lint`

## 12. 风险与替代方案
- 当前方案的风险：
  - `oapi-codegen` strict handler 的 request decode 与现有 DTO decode helper 不完全同源，需要用 request error handler 和 handler 级校验保持错误码与消息等价。
  - generated response object 类型较多，handler 映射需要谨慎，避免把 generated model 泄漏到 service/domain/repository。
  - 删除 `internal/http/dto` 后，handler 对 generated model 的依赖更直接；必须守住 generated model 只停留在 HTTP 层，不进入 service/domain。
  - 删除项目自有 `APIResponse` 后，新增成功响应必须优先通过 OpenAPI schema 生成 typed envelope；如果绕过 OpenAPI 新增临时 HTTP 面，需要单独说明 response 形态。
  - 认证/授权 middleware 必须在 strict body decode 前执行，否则受保护写接口可能出现错误优先级漂移。
  - strict-server route 适配代码从 `router.go` 拆到同 package 文件后，需要避免把内部 helper 提升为跨 package API，防止无意义的可见性扩张。
  - 删除旧 direct `net/http` 方法后，如果测试或未来代码仍绕过 strict server 直接调用业务 handler 方法，会在编译期暴露，需要改为通过 router 或 strict 方法验证。
  - 业务模块内只保留 `strict.go` 并不意味着所有 helper 都塞进 `strict.go`；校验/解析 helper 继续分文件，否则 strict 文件会快速膨胀。
  - 删除 response contract test 后，真实响应 schema 覆盖将主要依赖 OpenAPI policy/generation/lint/check gates 和业务集成测试；后续如需要更强 response schema 验证，可基于 strict typed responses 重新设计。
- 备选方案：
  - 方案 A：只新增编译期 adapter，让现有 handler 满足 generated interface 但 runtime router 仍手写。
  - 方案 B：保留手写 router，并继续依赖 router/openapi contract tests。
  - 方案 C：新增 `api/openapi/genstrict` 实验目录，双链路评估后再迁移。
  - 方案 D：接入运行时 OpenAPI request validation middleware。
  - 方案 E：继续保留 `internal/http/dto` 与项目自有 `APIResponse`，只把 strict server 当作路由和 JSON decode 层。
  - 方案 F：把 auth/system/user handler 全部移动到 `internal/http/handler` 根 package，只保留多个 `strict.go`。
- 为什么不选备选方案：
  - 不选方案 A：不能解决运行时 source-of-truth，仍会留下手写 route list。
  - 不选方案 B：补救型测试能发现漂移，但不能消除双源维护。
  - 不选方案 C：用户明确要求一次性迁移到生产 strict-server，不保留实验链路。
  - 不选方案 D：本次目标是 router source-of-truth 迁移；request validation middleware 会扩大运行时行为变化面。
  - 不选方案 E：strict server 已提供适用的 request/response model，继续维护镜像 DTO 和 `APIResponse{Data any}` 会保留转换噪音和双轨契约。
  - 不选方案 F：会违反 handler 按业务模块子包组织的长期规则，并造成 `Handler`、response mapper、validation helper 等名称互相挤压。
- 后续可演进点：
  - 如果需要完整 incoming request validation，可在 strict-server 之外另行设计 OpenAPI request validation 或生成校验层，并单独评估错误码兼容。
  - 随 event/order/payment API 增加，继续以 `eventhub.yaml` 先行，再由 generated strict server 注册 runtime route。
  - 如果未来确需非 OpenAPI DTO，必须把它限制在 handler 层，并在设计文档说明 generated model 不适用的原因；service contract 和 domain model 仍保持独立。
