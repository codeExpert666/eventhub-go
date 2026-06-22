# OpenAPI 响应契约测试设计

## 1. 背景
- 当前 Go 版以 `api/openapi/eventhub.yaml` 作为 spec-first API 契约源，运行时 router 和 handler 仍由 Go 代码显式注册与实现。
- 现有测试已经覆盖 `/openapi.yaml`、`/swagger/*` 文档路由开关，以及 `internal/http/router_contract_test.go` 中 router method/path 与 OpenAPI paths/methods 的一致性。
- 现有测试尚未证明真实 handler 写出的 HTTP status、`Content-Type` 和 response body 能通过 OpenAPI response schema 校验。
- Java 版依赖 controller、DTO 和 Springdoc 生成契约；Go 版手写 YAML 与手写 router/handler 并存，因此需要额外 contract test 连接真实响应与 OpenAPI schema。

## 2. 目标
- 新增 `internal/http/openapi_contract_test.go`，从真实 router 发起 HTTP 请求。
- 加载并 validate `api/openapi/eventhub.yaml`。
- 使用 `kin-openapi/openapi3filter.ValidateResponse` 校验真实响应。
- 首批覆盖 5 条低外部依赖关键路径：
  - `GET /actuator/health`
  - `GET /actuator/info`
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET /api/v1/me` 未带 access token 返回 `401`
- 每个用例至少校验：
  - OpenAPI route 能匹配 method/path。
  - 真实响应 status code 已在 spec 中声明。
  - 响应 `Content-Type` 能匹配声明的 media type。
  - response body 能匹配对应 schema。
- 保证 `go test ./...` 和 `make openapi-check` 通过。

## 3. 非目标
- 不改变生产 router、handler、service、repository、middleware 或 app provider 逻辑。
- 不为所有业务接口一次性补齐 response contract 测试。
- 不校验复杂数据库成功路径、管理员分页或 refresh token 轮换等需要更完整夹具的业务流。
- 不接入运行时 OpenAPI validation middleware。
- 不修改 OpenAPI schema 语义，除非测试暴露出真实契约错误。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `internal/http/openapi_contract_test.go`：新增真实响应契约测试。
  - `docs/ai/design/024-openapi-response-contract-test.md`：记录设计。
  - `docs/ai/implementation/024-openapi-response-contract-test.md`：记录实现和验证。
  - `docs/ai/parity/java-go-parity-matrix.md`：补充 OpenAPI response contract 测试策略。
- 重要 package 刻意不触碰：
  - `internal/http/router.go`：不改变路由注册。
  - `internal/http/handler/*`：不改变响应写出逻辑。
  - `internal/service/*`、`internal/repository/*`、`internal/repository/mysql/sqlc`：不改变业务和持久化行为。
- 不涉及数据库表、migration、sqlc query、缓存、JWT claim 结构或运行时授权策略变化。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 影响。该改动属于 Go 版 spec-first OpenAPI 与真实响应之间的测试策略补齐。

## 5. 领域建模
- 不新增业务领域对象。
- 测试内部使用 `openAPIResponseContractCase` 描述一个真实响应契约用例：
  - `method`
  - `path`
  - `body`
  - `headers`
  - `wantStatus`
- 与 Java 版领域对象的对应关系：
  - Java controller/DTO 响应与 Springdoc schema 的绑定，在 Go 版对应为 handler/dto/service result 写出的真实 JSON 与 `eventhub.yaml` response schema 的一致性。

## 6. API 设计
- 本次不新增或修改运行时 HTTP API。
- 覆盖接口和期望响应：
  - `GET /actuator/health`：`200 application/json`，body 匹配 `HealthResponse`。
  - `GET /actuator/info`：`200 application/json`，body 匹配 `InfoResponse`。
  - `GET /api/v1/system/ping`：`200 application/json`，body 匹配 `ApiResponsePing`。
  - `POST /api/v1/system/echo`：`200 application/json`，body 匹配 `ApiResponseEcho`。
  - `GET /api/v1/me` 未带 token：`401 application/json`，body 匹配 `ErrorResponse`。
- 错误码 / 异常场景：
  - 不新增错误码。
  - `/api/v1/me` 缺失认证信息继续由 auth middleware 映射为 `AUTH-401`，测试只验证其 status 和响应结构符合 OpenAPI `401` response。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Java 版更依赖注解生成契约；Go 版保留 spec-first YAML，因此用测试显式校验真实响应。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 测试覆盖不需要真实数据库事务。

## 8. 关键流程
- 正常流程：
  1. 测试通过现有 `testAuthRouter(t)` 构造完整真实 router，包含 system、auth、user 和 auth middleware。
  2. 使用 `openapi3.Loader` 加载 `api/openapi` 包嵌入的 `eventhub.yaml`，并执行 `doc.Validate(context.Background())`。
  3. 使用 `legacyrouter.NewRouter(doc)` 建立 OpenAPI route matcher。
  4. 对每个用例用 `httptest` 向真实 router 发请求。
  5. 先断言真实 status 等于用例期望值，避免 schema 错误掩盖状态码漂移。
  6. 用 OpenAPI router 查找对应 operation 和 path params。
  7. 构造 `openapi3filter.RequestValidationInput` 与 `ResponseValidationInput`。
  8. 调用 `ValidateResponse`，并设置 `IncludeResponseStatus: true`，确保未声明 status 直接失败。
  9. 失败信息包含 method/path/status/body 片段，便于定位是路由、状态码、media type 还是 schema 问题。
- 异常流程：
  - spec 加载或 validate 失败时测试失败。
  - OpenAPI router 无法匹配 method/path 时测试失败。
  - status 未声明、`Content-Type` 不匹配或 body 不满足 schema 时测试失败。
- 状态流转：
  - 不涉及业务状态机。
- handler / service / repository / sqlc/database 分工：
  - 测试从 HTTP 层外部观察真实响应。
  - 不改变 handler/service/repository/sqlc 分层边界。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无。
- 如何防重复提交：无运行时写入。
- 事务边界在哪里：不涉及数据库事务。
- 缓存放在哪里，为什么：不涉及缓存。

## 10. 权限与安全
- 哪些角色能访问：
  - actuator 与 system 测试路径无需认证。
  - `/api/v1/me` 测试刻意不带 token，验证未认证失败响应契约。
- 鉴权与鉴别约束：
  - 复用真实 auth middleware，不绕过安全逻辑。
  - 不新增 token、session 或角色测试夹具。
- JWT claim 边界：
  - 不修改 JWT claim，不把角色、邮箱、用户名或用户状态写入 JWT。
- 是否涉及敏感信息、审计或操作日志：
  - 不涉及敏感信息持久化或审计日志。

## 11. 测试策略
- 单元测试：
  - 新增 table-driven `TestOpenAPIResponseContractsValidateRealRouterResponses`。
- service / repository 测试：
  - 不新增；本次只从真实 HTTP router 验证已存在响应。
- migration / sqlc 验证：
  - 不运行；没有数据库、migration 或 sqlc 变化。
- 接口验证：
  - 每个用例都先发真实 HTTP 请求，再做 OpenAPI response 校验。
- OpenAPI validate：
  - 测试内 validate spec。
  - 最终运行 `make openapi-check`。
- 异常场景验证：
  - 覆盖 `/api/v1/me` 未带 token 的 `401` 失败响应。
- Java-Go parity 验证：
  - 更新 parity matrix，说明 Go 版新增真实响应 schema contract 测试。
- 需要运行的命令：
  - `gofmt -w internal/http/openapi_contract_test.go`
  - `go test ./internal/http -run TestOpenAPIResponseContractsValidateRealRouterResponses -count=1`
  - `go test ./...`
  - `go vet ./...`
  - `make openapi-check`

## 12. 风险与替代方案
- 当前方案的风险：
  - `openapi3filter` 的错误信息较底层，因此测试需要包装 method/path/status/body 片段来提升定位效率。
  - 首批只覆盖 5 条低依赖路径，不能替代完整业务流 contract suite。
  - OpenAPI server URL 为 `http://localhost:8080`，测试请求需要使用同源绝对 URL 才能稳定匹配 OpenAPI router。
- 备选方案：
  - 方案 A：为所有 OpenAPI paths 自动生成请求并校验响应。
  - 方案 B：只用 schema validation helper 直接校验手写 JSON fixture。
  - 方案 C：接入运行时 OpenAPI validation middleware。
- 为什么不选备选方案：
  - 不选方案 A：复杂业务流需要数据库、认证状态和请求数据编排，超出“先覆盖少量关键接口”的目标。
  - 不选方案 B：fixture 不能证明真实 router/handler 的响应符合契约。
  - 不选方案 C：会改变生产运行逻辑和中间件边界，本次只需要测试门禁。
- 后续可演进点：
  - 后续可逐步加入 auth register/login、admin user、refresh/logout 等更复杂真实业务路径。
  - 后续可在测试 helper 中加入 request validation，但本次重点是 response contract。
