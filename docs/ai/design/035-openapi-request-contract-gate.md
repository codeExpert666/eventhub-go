# OpenAPI Request Contract Gate

## 1. 背景
- Go 版当前采用 spec-first OpenAPI，`api/openapi/eventhub.yaml` 是 HTTP API 契约源，`oapi-codegen` 生成 `models.gen.go` 与 `server.gen.go`，生产 router 已由 generated chi strict server wrapper 注册业务与 actuator routes。
- strict-server 解决了 route source-of-truth、typed request/response object 和 JSON body decode，但它不是完整 incoming request validation 引擎。path、query、header、cookie、request body schema、content-type 和 security requirement 仍需要一个运行时 contract gate 统一执行。
- 当前 `internal/http/validation` 实际只负责把 HTTP 入参问题构造成 `AppError`，不是一个真正的 validator。若新增 `internal/http/requestvalidation`，两个包名会同时表达 validation，容易混淆职责。
- 之前 OpenAPI 文档入口已经从 Go embed 改为本地静态资源：`api/openapi/assets.go` 只保存路径常量，`/openapi.yaml` 由 `OPENAPI_ASSET_ROOT` 指向的本地文件提供。request contract gate 不应恢复 embed，也不应恢复旧的 `SpecYAML()` 生产 API。
- Java 版参考语义来自 Spring MVC 参数绑定、Bean Validation、Spring Security filter chain、Springdoc OpenAPI 契约和 `GlobalExceptionHandler` 的统一错误响应。Go 版不逐行复刻 Spring，而是用 spec-first OpenAPI、`kin-openapi`、显式 middleware 和项目 `AppError` 映射复现同等契约治理能力。

## 2. 目标
- 新增运行时 OpenAPI request contract gate，覆盖：
  - path parameter。
  - query parameter。
  - header parameter。
  - cookie parameter。
  - request body required、JSON schema 与 `additionalProperties` 等 schema 规则。
  - `Content-Type` 与 OpenAPI `requestBody.content`。
  - operation-level security requirement。
  - `x-required-roles` 管理员角色约束。
- 将现有 `internal/http/validation` 重命名并收敛为 `internal/http/requesterror`，明确它只负责构造统一请求错误，不负责执行校验。
- 新增 `internal/http/contract` 作为 OpenAPI contract gate 包，负责 spec 加载、middleware、security requirement、body replay、violation 映射和错误转换。
- request contract gate 从显式 `OPENAPI_SPEC_PATH` 读取部署资产中的 `eventhub.yaml`，不使用 Go embed。
- `OPENAPI_ENABLED` 继续只控制 `/openapi.yaml` 和 `/swagger/*` 文档入口，不影响 request validation 是否启用。
- 新增 `OPENAPI_REQUEST_VALIDATION_ENABLED`，控制运行时 request contract gate 是否启用。
- 启用 request validation 时，在 app provider / bootstrap 阶段加载并 validate OpenAPI spec；路径缺失、spec 不合法或 security 配置不满足项目要求时启动失败。
- 保持 handler -> service -> repository -> sqlc/database 分层，service/domain/repository 不依赖 OpenAPI generated model、`kin-openapi` 或 HTTP contract 包。
- 按阶段落地，避免一次性大改导致执行质量下降。

## 3. 非目标
- 不修改业务 API path、method、请求字段、响应字段、错误码或分页语义。
- 不重新引入 Go embed，不恢复 `api/openapi/spec.go:SpecYAML()` 这类生产读取 API。
- 不让文档入口 `OPENAPI_ENABLED` 兼任 request validation 开关。
- 不把业务规则全部迁入 OpenAPI schema；账号是否存在、用户状态、refresh token replay、数据库唯一约束等仍属于 service/repository 语义。
- 不让 service/domain/repository 依赖 `internal/http/contract`、`internal/http/requesterror` 或 `api/openapi/gen`。
- 不引入重量级 web framework 或替换 `oapi-codegen` strict-server。
- 不直接暴露 `http.FileServer` 或扩大 OpenAPI asset 公开范围。
- 不在第一阶段删除所有 handler 内防御式字段校验；应在 contract gate 稳定后按 endpoint 逐步收敛重复校验。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `internal/http/validation`：重命名为 `internal/http/requesterror`。
  - `internal/http/requesterror`：保留 `FieldErrors`、malformed body、body validation、parameter validation 错误构造能力，并调整函数命名。
  - `internal/http/contract`：新增 OpenAPI request contract gate。
  - `internal/http/openapi_routes.go`：接入 contract middleware，并移除基于 generated context 和 path 前缀的临时 security 判断。
  - `internal/http/handler/{auth,system,user}`：更新 request error 包名；后续逐步收敛重复字段校验。
  - `internal/http/response`：继续统一写出错误 envelope，不新增 request validation 逻辑。
  - `internal/config`：新增 `OPENAPI_SPEC_PATH` 与 `OPENAPI_REQUEST_VALIDATION_ENABLED`。
  - `internal/app/providers`：启动期加载 OpenAPI spec 并装配 contract middleware。
  - `api/openapi/eventhub.yaml`：可能需要补充 header/cookie/security/role policy 所需的显式声明或 extension，但本设计不主动改变业务字段。
  - `api/openapi/openapi_policy_test.go`：补充 request validation 所需 spec policy。
  - `docs/ai/parity/java-go-parity-matrix.md`：更新 OpenAPI / Swagger 与统一错误映射行。
- 涉及 API / 表 / 缓存 / 外部接口：
  - 对外 API 契约原则上不变；运行时会更严格执行已有 OpenAPI 契约。
  - 不涉及数据库、migration、sqlc query、Redis 或外部服务。
- 是否影响 parity matrix：
  - 影响。Go 端新增运行时 OpenAPI request contract gate，属于 Java-Go 契约治理与安全边界的重要差异索引。

## 5. 领域建模
- `RequestError`
  - Go package：`internal/http/requesterror`。
  - 职责：构造项目统一 `AppError`，例如 malformed body、invalid body、invalid parameter。
  - 不依赖 `kin-openapi`、generated OpenAPI model 或 service。
  - 推荐公共函数：
    - `MalformedBody() *apperror.AppError`
    - `InvalidBody(fields FieldErrors) *apperror.AppError`
    - `InvalidParameters(fields FieldErrors) *apperror.AppError`
    - `InvalidHeaders(fields FieldErrors) *apperror.AppError`
    - `InvalidCookies(fields FieldErrors) *apperror.AppError`
    - `UnsupportedContentType(contentType string) *apperror.AppError`
- `OpenAPIContract`
  - 物理文件：部署资产 `eventhub.yaml`，默认路径由 `OPENAPI_SPEC_PATH` 指定。
  - 不编译进二进制，不依赖文档入口是否开启。
  - 启动期由 provider 加载、resolve refs、validate。
- `RequestContractGate`
  - Go package：`internal/http/contract`。
  - 职责：在 generated strict handler 前执行 operation matching 和 request validation。
  - 使用 `kin-openapi/openapi3filter` 作为 OpenAPI 语义执行核心。
  - 负责读取 request body 后回放，保证 strict-server 后续仍能 decode。
  - 负责把 OpenAPI validation error 映射成项目稳定 `AppError`。
- `SecurityRequirement`
  - OpenAPI `components.securitySchemes` 和 operation `security` 是认证要求的事实来源。
  - `x-required-roles` 是项目角色扩展元数据，当前用于 ADMIN-only operation。
  - JWT 仍只携带稳定身份 claim；角色、邮箱、用户名、状态必须服务端查询或通过受控缓存获得。
- 与 Java 版领域对象的对应关系：
  - Java Spring MVC 参数绑定 / Bean Validation 对应 Go `contract` + `requesterror`。
  - Java Spring Security filter chain 对应 Go `contract` security requirement + 现有 auth principal loading。
  - Java `GlobalExceptionHandler` 对应 Go `apperror` + `response.WriteError` + `requesterror`。

## 6. API 设计
- 对外接口列表：
  - 不新增业务 endpoint。
  - 不改变 `/openapi.yaml`、`/swagger/*` 文档入口。
- 请求参数：
  - path/query/header/cookie 均以 OpenAPI parameters 为运行时 contract 来源。
  - 业务 header/cookie 必须进入 OpenAPI spec；`Authorization` 不作为普通 header parameter 重复声明，而由 security scheme 表达。
  - 阶段五策略：只严格校验 OpenAPI 已声明的 query/header/cookie parameter；未知 query/header/cookie 暂不拒绝，以兼容客户端追踪字段、灰度字段、浏览器/代理注入 header 或历史调用方透传 cookie。后续如改为拒绝未知字段，必须补 router/contract 测试证明不影响既有客户端契约。
- request body：
  - `requestBody.required`、media type、schema、required fields、enum、min/max、pattern、format、`additionalProperties` 由 contract gate 执行。
  - contract gate 读取 body 后必须重置 `r.Body`，避免破坏 generated strict decoder。
  - 空 body、非法 JSON、类型不匹配、未知字段等都映射成统一 `COMMON-400`。
- content-type：
  - body operation 必须匹配 OpenAPI `requestBody.content`，例如 `application/json`，允许标准 charset 参数。
  - 不带 body 的 GET/HEAD 不强制 content-type。
- security requirement：
  - public operation：OpenAPI `security` 为空或显式空数组时放行。
  - authenticated operation：OpenAPI 声明 `BearerAuth` 时执行认证。
  - admin operation：同时要求 `BearerAuth` 与 `x-required-roles: [ADMIN]`。
  - 认证失败映射 `AUTH-401`，授权失败映射 `AUTH-403`。
- 响应结构：
  - request contract violation 继续输出统一错误 envelope：`code/message/data/requestId/timestamp`。
  - `data` 推荐使用稳定结构：

```json
{
  "violations": [
    {
      "location": "query",
      "name": "page",
      "path": "page",
      "rule": "minimum",
      "message": "page 必须大于等于 1"
    }
  ]
}
```

- 错误码 / 异常场景：
  - malformed body：`COMMON-400`，`message=请求体格式不合法`。
  - body schema violation：`COMMON-400`，`message=请求体参数校验失败`。
  - path/query violation：`COMMON-400`，`message=请求参数校验失败`。
  - header violation：`COMMON-400`，`message=请求头参数校验失败`。
  - cookie violation：`COMMON-400`，`message=Cookie 参数校验失败`。
  - unsupported content-type：`COMMON-400`，`message=请求内容类型不支持`。
  - missing/invalid token：`AUTH-401`。
  - missing required role：`AUTH-403`。
  - spec load/validate 失败：启动失败，不暴露 HTTP 响应。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Java 通过框架自动绑定和 Bean Validation 执行多数请求校验；Go 通过 spec-first OpenAPI contract gate 显式执行。
  - Go 的 spec 文件是部署资产，不是二进制内嵌资源。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 本次不改变事务边界或持久化行为。
  - 更严格的 request validation 会在进入 handler/service 前拒绝不符合 OpenAPI contract 的输入，减少无效请求进入业务层。

## 8. 关键流程
- 正常流程：
  1. `config.Load()` 读取 `OPENAPI_REQUEST_VALIDATION_ENABLED` 与 `OPENAPI_SPEC_PATH`。
  2. HTTP provider 在启用 request validation 时从 `OPENAPI_SPEC_PATH` 加载 `eventhub.yaml`，执行 OpenAPI doc validate。
  3. provider 创建 `contract.RequestValidator`，注入认证能力、角色检查能力和错误映射策略。
  4. `NewRouter` 注册 request id / recover middleware。
  5. `registerOpenAPIRoutes` 将 contract middleware 放到 generated strict server 之前。
  6. 请求进入 contract gate，匹配 operation 并校验 parameters、body、content-type 和 security requirement。
  7. body 被读取后回放，generated strict handler 继续 decode typed request object。
  8. module strict handler 映射 generated request 到 service Command / Query，调用 service 并返回 generated typed response。
- 异常流程：
  - spec 路径为空、缺失或 OpenAPI validate 失败：启动失败。
  - request contract violation：contract gate 写出统一 error envelope，不进入 generated strict handler。
  - 认证失败：contract gate 或其注入的 authentication bridge 写出 `AUTH-401`。
  - 授权失败：角色检查返回 `AUTH-403`。
  - body 读取失败或超过上限：写出 `COMMON-400` 或后续可扩展为请求体过大错误码。
- 状态流转：
  - 不涉及业务状态机。
- handler / service / repository / sqlc/database 分工：
  - `internal/http/contract`：OpenAPI request contract gate。
  - `internal/http/requesterror`：HTTP 入参错误构造。
  - `internal/http/openapi_routes.go`：generated route registration 与 middleware 编排。
  - module handler：保留 generated request -> service command/query 映射和少量业务组合校验。
  - service/repository/sqlc：不感知 OpenAPI contract gate。
- 建议阶段：
  1. 阶段一：将 `internal/http/validation` 重命名为 `internal/http/requesterror`，只做命名和调用点收敛。
  2. 阶段二：新增 config/provider/spec loader，不接入 runtime middleware。
  3. 阶段三：新增 `internal/http/contract` middleware，覆盖 path/query/body/content-type，接入 router。
  4. 阶段四：将 BearerAuth 与 `x-required-roles` 从现有 `openAPISecurityMiddleware` 迁入 contract gate。
  5. 阶段五：补齐 header/cookie policy、未知参数策略和 handler 重复校验收敛。
     - header/cookie 已声明参数的 violation 由 `internal/http/contract` 映射为更具体的请求头 / Cookie 参数错误。
     - 未知 query/header/cookie 当前允许通过；本阶段仅固化该兼容策略，不提前收紧。
     - handler 中仅删除 OpenAPI schema 完整覆盖且已有 router contract 测试证明的 transport 校验，业务组合规则和 service 兜底校验继续保留。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无。
- 如何防重复提交：不涉及订单、库存或支付写入。
- 事务边界在哪里：不改变现有 service/repository 事务边界。
- 缓存放在哪里，为什么：
  - OpenAPI doc 在启动期解析后可常驻内存，属于只读 contract metadata，不是业务缓存。
  - principal / role 加载策略沿用现有认证 middleware；如未来引入角色缓存，必须另行设计 JWT claim 与缓存一致性边界。
- 并发考虑：
  - `contract.RequestValidator` 应只持有启动期构建完成的只读 OpenAPI doc、路由器和配置。
  - body replay 使用 per-request buffer，不共享可变状态。

## 10. 权限与安全
- 哪些角色能访问：
  - public operation：不需要身份。
  - authenticated operation：需要有效 Bearer token。
  - admin operation：需要有效 Bearer token 且当前用户拥有 ADMIN 角色。
- 鉴权与鉴别约束：
  - OpenAPI `security` 是 operation 是否需要认证的事实来源。
  - `x-required-roles` 必须与 `BearerAuth` 同时出现；policy test 应防止 ADMIN metadata 泄漏到 public operation 或 admin operation 漏声明。
  - 认证应发生在 strict JSON decode 前，避免未认证请求因 body 格式错误先返回 `COMMON-400`。
- JWT claim 边界：
  - 不把角色、邮箱、用户名、用户状态写入 JWT。
  - JWT 只保留用户 ID、session ID、jti、typ、iss、iat、exp 等稳定技术 claim。
- 敏感信息、审计或操作日志：
  - request validation error 不应暴露底层 Go error、文件路径或 schema 内部路径细节之外的敏感信息。
  - 启动期 spec load 错误可记录本地路径，面向部署者，不作为 HTTP 响应返回。
  - request body buffer 不应被日志直接记录。

## 11. 测试策略
- 单元测试：
  - `internal/http/requesterror`：
    - malformed body、invalid body、invalid parameters、unsupported content-type 的 `AppError` code/message/details。
  - `internal/http/contract`：
    - spec loader 从文件系统加载，路径缺失或 spec 非法时失败。
    - body replay 后 strict decoder 仍可读取。
    - path/query/header/cookie/body/content-type violation 映射稳定 `AppError`。
    - header parameter violation 映射 `请求头参数校验失败`。
    - cookie parameter violation 映射 `Cookie 参数校验失败`。
    - 未知 query/header/cookie 在当前阶段允许通过。
    - security requirement bridge 区分 public、authenticated、admin。
- service / repository 测试：
  - 不涉及。
- migration / sqlc 验证：
  - 不涉及。
- 接口验证：
  - 真实 router 集成测试覆盖：
    - invalid path param。
    - invalid query param。
    - unsupported content-type。
    - malformed JSON。
    - body schema violation。
    - missing token。
    - invalid token。
    - non-admin token 访问 admin operation。
    - valid request 仍进入 strict handler。
    - schema 已接管的 handler 重复校验，例如 echo message/tag 长度、管理员用户 status enum、更新用户状态 body enum。
- OpenAPI validate：
  - `make openapi-check`。
  - `make openapi-lint`。
  - `api/openapi/openapi_policy_test.go` 增加 request contract 所需 policy。
- 异常场景验证：
  - `OPENAPI_REQUEST_VALIDATION_ENABLED=false` 时 router 仍保持当前 strict-server 行为。
  - `OPENAPI_REQUEST_VALIDATION_ENABLED=true` 且 `OPENAPI_SPEC_PATH` 缺失时启动失败。
  - `OPENAPI_ENABLED=false` 但 request validation enabled 时业务 API contract gate 仍可运行，文档入口仍不注册。
- Java-Go parity 验证：
  - 更新 parity matrix，记录 Go 通过 OpenAPI contract gate 显式执行 Java/Spring 对应的请求绑定、校验和安全 requirement。
- 需要运行的命令：
  - `gofmt`。
  - `go test ./internal/http/requesterror ./internal/http/contract ./internal/http -count=1`。
  - `go test ./...`。
  - `go vet ./...`。
  - `make openapi-check`。
  - `make openapi-lint`。
  - `make lint` 或 `golangci-lint run ./...`。
  - `git diff --check`。

## 12. 风险与替代方案
- 当前方案的风险：
  - runtime request validation 会比当前 strict-server 更严格，可能暴露既有 spec 与 handler 容忍行为之间的差异。
  - body replay 如果处理不当，会导致 strict decoder 读不到 body 或内存占用过高。
  - security requirement 从 path 前缀迁入 OpenAPI metadata 后，spec 漏声明会变成安全风险，因此必须配套 policy test。
  - `OPENAPI_SPEC_PATH` 与 `OPENAPI_ASSET_ROOT` 语义相近，文档和配置示例必须解释清楚：前者服务 runtime contract gate，后者服务 docs assets。
- 备选方案：
  - 方案 A：直接使用 `github.com/oapi-codegen/nethttp-middleware` 默认 middleware。
  - 方案 B：继续只依赖 strict-server 和 handler 内手写校验。
  - 方案 C：新增 `internal/http/requestvalidation`，保留现有 `internal/http/validation`。
  - 方案 D：重新把 `eventhub.yaml` embed 到二进制用于 runtime validation。
  - 方案 E：把 OpenAPI request validation 放到每个业务 handler 内。
- 为什么不选备选方案：
  - 不选方案 A：默认 middleware 不足以承载项目统一 error envelope、`AUTH-401/403`、`x-required-roles`、body replay 策略和后续审计扩展；可以参考其实现，但项目应持有边界。
  - 不选方案 B：无法覆盖 content-type、header、cookie、完整 schema 和 security requirement，继续让 OpenAPI contract 与 runtime 行为分离。
  - 不选方案 C：`validation` 与 `requestvalidation` 命名过近，且当前 `validation` 本身不是 validator，会放大历史包袱。
  - 不选方案 D：与本仓库已决策的本地静态资源 / 显式部署资产方向冲突。
  - 不选方案 E：会把 transport contract 逻辑分散到模块 handler，破坏 spec-first 的集中治理价值。
- 后续可演进点：
  - 引入更细的 violation localization 和中英文错误消息映射。
  - 对未知 query/header/cookie 增加分级策略：allow -> warn -> reject；进入 reject 前必须补兼容性测试。
  - 增加 request validation 指标，例如 reject count、operationId、violation location。
  - 随 event/order/payment API 增长，逐步把更多 transport 约束迁入 OpenAPI schema。
