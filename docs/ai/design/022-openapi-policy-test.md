# OpenAPI policy 测试设计

## 1. 背景
- 当前 Go 版已经采用 spec-first OpenAPI，`api/openapi/eventhub.yaml` 是业务接口契约源，并通过 `make openapi-check` 做 validate/generate 漂移检查。
- 现有业务接口约定统一返回 `ApiResponse` envelope，分页接口约定使用 `PageResponse` / `PageResponseUserInfo`，错误响应集中在 `components.responses`。
- `/actuator/*` 保留 Spring Boot Actuator 风格，是不包 `ApiResponse` 的例外。
- Java 版通过 Spring Security 与 controller 注解表达认证、角色和接口说明；Go 版运行时管理员授权由 router 中的 `RequireRole("ADMIN")` middleware 执行。
- Go 版 OpenAPI 需要用 `x-required-roles` 把管理员角色要求从自然语言 description 升级为机器可验证元数据；description 继续保留给人阅读，但 policy test 不只信任描述文本。
- 本次以 Go test 固化团队 API policy，避免后续新增或修改接口时漏掉 operationId、tag、统一响应 envelope、管理员角色声明，或把 ADMIN 角色误标到非管理员接口。

## 2. 目标
- 新增 `api/openapi/openapi_policy_test.go`，使用 `kin-openapi` 加载并验证 `api/openapi/eventhub.yaml`。
- policy 测试覆盖：
  - 每个 operation 必须有 `operationId`。
  - 每个 operation 必须有 `summary` 或 `description`。
  - 每个 operation 必须至少有一个 tag。
  - 除 `/actuator/*` 外，业务接口必须声明 `application/json` 响应。
  - 除 `/actuator/*` 外，2xx JSON 响应必须在最外层直接引用 `ApiResponse`，或通过最外层 `allOf` 组合 `ApiResponse`。
  - 除 `/actuator/*` 外，非 2xx 响应必须引用 `components.responses`，且被引用的组件响应必须统一使用 `ErrorResponse` schema。
  - `ErrorResponse` schema 本身必须在顶层复用 `ApiResponse` envelope，避免错误响应结构漂移。
  - `/api/v1/admin/**` 接口必须声明 `BearerAuth` security。
  - `/api/v1/admin/**` 接口必须声明 `x-required-roles: [ADMIN]`。
  - 非 `/api/v1/admin/**` 接口不得声明 `ADMIN` 角色，除非后续业务设计明确需要并同步更新 policy。
  - 当前认证策略与 router 保持一致：`register/login/refresh` 不声明 `BearerAuth`，`logout/me/admin` 继续声明 `BearerAuth`。
- 核验并保留 `api/openapi/eventhub.yaml` 中当前 admin operation 的 `x-required-roles: [ADMIN]`。
- 让 `go test ./...` 与 `make openapi-check` 通过。

## 3. 非目标
- 不修改业务 handler、service、repository、middleware 或 JWT 行为。
- 不新增运行时 OpenAPI 请求校验 middleware。
- 不改变 register/login/refresh/logout 的实际认证行为；尤其 `logout` 当前位于受保护路由组，本次不把它改为公开接口。
- 不引入 Spectral、Node 工具链或额外脚本。
- 不调整生成代码结构，不接入 generated chi server interface。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `api/openapi/openapi_policy_test.go`：新增 OpenAPI policy test。
  - `go.mod` / `go.sum`：新增或显式记录 `github.com/getkin/kin-openapi` 测试解析依赖。
  - `api/openapi/eventhub.yaml`：为 admin operation 补充 `x-required-roles`。
  - `docs/ai/implementation/022-openapi-policy-test.md`：记录实现与验证。
  - `docs/ai/parity/java-go-parity-matrix.md`：更新 OpenAPI / Swagger 行，索引 Go 版 policy test。
- 不涉及 API handler、DTO、service contract、domain、repository、sqlc、migration、Redis 或数据库。
- 涉及 API 契约：
  - 只新增 OpenAPI vendor extension，并加固 OpenAPI policy test；不改变 HTTP 路径、方法、请求字段、响应字段、状态码或错误码。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 影响。该测试属于 Go 版 spec-first 契约质量门禁，补足 Java 注解和安全配置在 Go OpenAPI 中的机器可验证表达。

## 5. 领域建模
- `OpenAPIPolicy`
  - 一组构建期测试规则，不是运行时领域对象。
  - 目标是约束 `eventhub.yaml` 的结构质量与团队 API 约定。
- `OperationPolicy`
  - operation 必须具备可追踪的 `operationId`、可读说明和 tag。
  - 失败信息包含 method、path、operationId，便于定位具体接口。
- `ResponseEnvelopePolicy`
  - 业务接口 JSON 2xx 响应必须以 `ApiResponse` 为统一 envelope。
  - 允许 schema 直接 `$ref` 到 `ApiResponse`，也允许 `$ref` 到通过 `allOf` 组合 `ApiResponse` 的具体 schema，例如 `ApiResponseUserInfo`。
  - 不把 properties、items、oneOf、anyOf 等子树中出现过 `ApiResponse` 视为合格，避免内层 payload 偶然复用 `ApiResponse` 时绕过最外层 envelope 约束。
- `ErrorResponsePolicy`
  - 业务接口非 2xx 响应必须先集中引用 `components.responses`，再由组件响应统一引用 `ErrorResponse`。
  - `ErrorResponse` 是错误响应的唯一 OpenAPI schema 入口；该 schema 继续通过顶层 `allOf` 复用 `ApiResponse`，保持 `code/message/data/requestId/timestamp` 结构一致。
  - 测试只检查业务接口实际引用到的组件响应，避免未使用组件影响当前接口门禁。
- `AdminRolePolicy`
  - `/api/v1/admin/**` operation 必须同时声明 `BearerAuth` 与精确的 `x-required-roles: [ADMIN]`。
  - 非 `/api/v1/admin/**` operation 默认不得声明 `ADMIN`，避免把管理端权限要求误传播到公开接口或普通登录用户接口。
  - `x-required-roles` 是 OpenAPI vendor extension，用于机器校验文档与 RBAC 中间件语义是否一致；真实授权仍以 middleware/service 为准。
- 与 Java 版领域对象的对应关系：
  - Java 的 controller `@Operation/@Tag` 对应 Go OpenAPI operationId、summary/description 和 tags。
  - Java 的统一响应与分页 VO 对应 Go OpenAPI `ApiResponse`、`PageResponse`、`PageResponseUserInfo`。
  - Java 的管理员角色安全约束对应 Go router 中 `RequireRole("ADMIN")` 与 OpenAPI `x-required-roles`。

## 6. API 设计
- 本次不新增或修改运行时 HTTP API。
- OpenAPI spec 变化：
  - `GET /api/v1/admin/users`
    - 增加 `x-required-roles: [ADMIN]`。
  - `PATCH /api/v1/admin/users/{userId}/status`
    - 增加 `x-required-roles: [ADMIN]`。
- 错误码 / 异常场景：
  - 运行时错误码不变。
  - policy test 失败时使用 `t.Errorf` 聚合多个违规项，错误信息包含 method/path/operationId 和具体规则。
  - 如果业务接口引用的 `components.responses.*` 缺少 `application/json`、缺少 schema、未引用 `ErrorResponse`，或者 `ErrorResponse` 不再以 `ApiResponse` 为顶层 envelope，policy test 失败。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Java 版没有 `x-required-roles` 字段；这是 Go spec-first 模式下为机器校验 RBAC 文档一致性增加的 vendor extension。
  - 不改变 Java-Go 业务语义，只补充 Go 版契约可验证性。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 不涉及业务数据。
  - `make openapi-check` 会在 YAML 更新后重新生成并检查 `api/openapi/gen/eventhub.gen.go` 是否有必要漂移。

## 8. 关键流程
- 正常流程：
  1. `go test ./...` 运行 `api/openapi` 包测试。
  2. 测试通过 `kin-openapi` 加载 `api/openapi/eventhub.yaml` 并执行 OpenAPI 自身 validate。
  3. 测试遍历 `doc.Paths.Map()` 中所有 operation。
  4. 对每个 operation 执行元信息、响应 envelope、错误响应集中引用、admin security、admin roles 和 auth 边界规则检查。
  5. 收集业务非 2xx 响应引用到的 `components.responses` 名称，并检查这些组件响应统一使用 `ErrorResponse`。
  6. 单独检查 `ErrorResponse` schema 顶层复用 `ApiResponse`。
  7. 如发现 operation 级违规，失败信息打印 `METHOD path (operationId=...)`；如发现组件级违规，失败信息打印组件名。
- 异常流程：
  - YAML 语法或 OpenAPI 结构非法时，加载或 validate 直接失败。
  - 业务接口缺少 JSON 响应或 2xx JSON schema 未在最外层使用 `ApiResponse` envelope 时，policy test 失败。
  - 非 2xx 响应虽然引用了 `components.responses`，但组件响应内部没有统一指向 `ErrorResponse` 时，policy test 失败。
  - admin operation 未声明 `x-required-roles`、不是精确 `[ADMIN]`，或非 admin operation 误声明 `ADMIN` 时，policy test 失败。
- 状态流转：
  - 不涉及业务状态机。
- handler / service / repository / sqlc/database 分工：
  - 不触碰 handler、service、repository 或 sqlc/database。
  - 测试只读取 OpenAPI spec 文件，不调用运行时 router。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无。
- 如何防重复提交：无运行时写操作。
- 事务边界在哪里：无数据库事务。
- 缓存放在哪里，为什么：不涉及缓存。

## 10. 权限与安全
- 哪些角色能访问：
  - admin API 仍由运行时 `RequireRole("ADMIN")` 控制。
  - OpenAPI spec 中用 `x-required-roles: [ADMIN]` 表达机器可验证的管理员角色需求。
  - `x-required-roles` 是文档和治理元数据，不参与运行时鉴权决策。
- 鉴权与鉴别约束：
  - `register/login/refresh` 当前不经过 `AuthMiddleware`，OpenAPI 不声明 `BearerAuth`。
  - `logout` 当前经过 `AuthMiddleware`，OpenAPI 保留 `BearerAuth`。
  - `/api/v1/me` 和 `/api/v1/admin/**` 保留 `BearerAuth`。
- JWT claim 边界：
  - 不修改 JWT，不把角色、邮箱、用户名或用户状态写入 JWT。
  - 角色仍由服务端 principal loader / RBAC middleware 判断。
- 是否涉及敏感信息、审计或操作日志：
  - 不新增敏感信息字段。
  - `x-required-roles` 只暴露 API 所需角色，符合现有 description 已公开的信息。

## 11. 测试策略
- 单元测试：
  - 新增 `TestOpenAPIPolicy`，集中校验 spec policy。
  - 新增 `TestSchemaUsesTopLevelComponentRequiresEnvelope`，验证直接 `$ref` 和顶层 `allOf` 合格，同时验证嵌套属性中的 `ApiResponse` 不能通过 envelope policy。
  - 新增错误响应组件策略用例，先构造组件响应指向非 `ErrorResponse` 的文档，验证测试 helper 能识别该漂移。
- service / repository 测试：
  - 不新增；本次不触碰业务服务和持久化。
- migration / sqlc 验证：
  - 不运行；本次没有数据库、SQL 或 sqlc 配置变化。
- 接口验证：
  - policy test 静态验证 OpenAPI 契约；不做 HTTP handler 集成测试。
- OpenAPI validate：
  - 测试内调用 `doc.Validate(context.Background())`。
  - 最终运行 `make openapi-check`。
- 异常场景验证：
  - 先写测试并运行，预期当前 helper 在 admin roles 不是精确 `[ADMIN]` 或非 admin 误标 `ADMIN` 时无法报错。
  - 补充 policy helper 后重新运行目标测试与全量 `go test ./...`。
- Java-Go parity 验证：
  - 检查 parity matrix 的 OpenAPI / Swagger 行是否索引本次 policy test。
- 需要运行的命令：
  - `gofmt -w api/openapi/openapi_policy_test.go`
  - `go test ./api/openapi`
  - `go test ./...`
  - `make openapi-check`

## 12. 风险与替代方案
- 当前方案的风险：
  - policy test 过严可能在后续新增特殊接口时需要显式例外；当前只保留 `/actuator/*` 例外，避免规则泛化过度。
  - `x-required-roles` 是 vendor extension，生成代码不会直接使用它；它主要服务于契约审查、文档治理和测试门禁。
- 备选方案：
  - 方案 A：只用 `make openapi-validate`，不新增 Go policy test。
  - 方案 B：引入 Spectral ruleset。
  - 方案 C：在运行时 router 或 handler 上增加注解式元数据再生成 OpenAPI。
- 为什么不选备选方案：
  - 不选方案 A：OpenAPI 标准校验无法表达团队 envelope、role 和 tag/operationId policy。
  - 不选方案 B：当前 Go 仓库已有 Go test 质量门禁，引入 Node/Spectral 会增加额外工具链和 CI 成本。
  - 不选方案 C：会扩大到运行时元数据设计和生成链路改造，超出本次防漂移测试目标。
- 后续可演进点：
  - 后续新增业务模块时，可把分页 schema、错误响应集中引用、公开/受保护接口安全策略继续加入 policy table。
  - 如未来出现新的非 `ApiResponse` 特例，应先写设计说明，再在测试中增加明确例外。
