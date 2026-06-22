# OpenAPI 响应契约测试实现说明

## 1. 本次改动解决了什么问题

本次为少量关键 API 增加真实 router response contract 测试，补齐了“运行时 handler 实际写出的 status、`Content-Type` 和 JSON body 是否匹配 `api/openapi/eventhub.yaml` response schema”的验证空白。

此前已有 `/openapi.yaml`、`/swagger/*` 路由测试，以及 router method/path 与 OpenAPI paths/methods 的一致性测试，但这些测试不能证明业务接口真实响应体符合 OpenAPI schema。本次新增测试把真实 HTTP 请求、真实 router、OpenAPI route matcher 和 `openapi3filter.ValidateResponse` 串起来，作为后续 API 演进的回归门禁。

## 2. 改动内容
- 新增了什么
  - `internal/http/openapi_contract_test.go`
    - 新增 `TestOpenAPIResponseContractsValidateRealRouterResponses`。
    - 通过现有 `testAuthRouter(t)` 构造真实 router。
    - 加载 `api/openapi` 包嵌入的 `eventhub.yaml`，并执行 OpenAPI validate。
    - 使用 `legacyrouter.NewRouter(doc)` 匹配 OpenAPI operation。
    - 使用 `openapi3filter.ValidateResponse` 校验真实响应。
    - 设置 `IncludeResponseStatus: true`，保证未声明 status code 会失败。
    - 覆盖 5 条真实响应路径：
      - `GET /actuator/health` -> `200`
      - `GET /actuator/info` -> `200`
      - `GET /api/v1/system/ping` -> `200`
      - `POST /api/v1/system/echo` -> `200`
      - `GET /api/v1/me` 未带 token -> `401`
    - 失败信息包含 method、path、status、`Content-Type` 和压缩后的 body 片段，便于定位 schema 问题。
  - `docs/ai/design/024-openapi-response-contract-test.md`。
  - `docs/ai/implementation/024-openapi-response-contract-test.md`。
- 修改了什么
  - `internal/http/auth_integration_test.go`
    - `testAuthRouter(t)` 的 system config 补充 `Env: config.EnvTest`。
    - 这是测试夹具修正，用于让 `/actuator/info` 返回值满足 OpenAPI 中 `env: dev|test|prod` 的契约；不改变生产配置加载或 handler 逻辑。
  - `docs/ai/parity/java-go-parity-matrix.md`
    - OpenAPI / Swagger 行补充真实响应 schema contract 测试索引。
- 删除了什么
  - 未删除文件。
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次补强 Go spec-first OpenAPI 与真实 HTTP 响应之间的测试策略，属于 Java-Go 契约治理差异的索引项。

## 3. 为什么这样设计
- 关键设计原因
  - 真实请求打到真实 router，避免只校验手写 JSON fixture。
  - `openapi3filter.ValidateResponse` 能同时覆盖 status、media type 和 schema，比手写字段断言更贴近 OpenAPI contract。
  - 首批只覆盖无需数据库成功路径的接口和一个认证失败路径，符合“先覆盖少量关键接口”的范围。
  - `GET /api/v1/me` 未带 token 复用真实 auth middleware，能验证安全失败响应 envelope 与 OpenAPI `401` 定义一致。
  - 使用 `http://localhost:8080` 构造测试请求，匹配 OpenAPI `servers` 中声明的本地服务地址。
- 与 Go 项目当前阶段的匹配点
  - 不接入运行时 OpenAPI middleware，不改变生产调用链。
  - 不引入新模块版本；`kin-openapi` 已是当前直接依赖。
  - 测试文件放在 `internal/http`，因为被验证对象是运行时 HTTP router 与 response。
- 与 Java 版业务语义的对齐方式
  - Java controller/DTO 与 Springdoc 的响应契约绑定更紧密；Go 版用 spec-first YAML 和手写 handler，因此通过 contract test 固定真实响应与 schema 的一致性。

## 4. 替代方案
- 方案 A：自动遍历所有 OpenAPI operation 并生成请求。
  - 没有采用。复杂业务成功路径需要数据库状态、认证会话、角色、分页参数和写入顺序，超出本次“少量关键接口”的目标。
- 方案 B：只用 schema helper 校验手写响应 fixture。
  - 没有采用。fixture 不能证明真实 router、middleware 和 handler 写出的响应符合契约。
- 方案 C：接入运行时 OpenAPI validation middleware。
  - 没有采用。该方案会改变生产运行逻辑和中间件边界，本次只需要测试门禁。
- 方案 D：扩展既有 `router_contract_test.go`。
  - 没有采用。既有文件职责是 method/path 漂移检测；新文件单独承载 response schema contract，边界更清楚。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./internal/http -run TestOpenAPIResponseContractsValidateRealRouterResponses -count=1`
    - 失败在 `GET /actuator/info`，真实 body 中 `app.env` 为空串，不满足 OpenAPI enum `dev/test/prod`。
  - GREEN：补齐 `testAuthRouter(t)` 的 `Env: config.EnvTest` 后，目标测试通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w internal/http/openapi_contract_test.go internal/http/auth_integration_test.go`：已运行。
  - `go test ./internal/http -run TestOpenAPIResponseContractsValidateRealRouterResponses -count=1`：已运行，通过。
  - `go test ./...`：已运行，通过。
  - `go vet ./...`：已运行，通过。
  - `make openapi-check`：已运行，通过，包含 OpenAPI validate、生成代码和 generated diff 检查。
  - `make lint`：已运行，通过，输出 `0 issues.`。
  - `git diff --check`：已运行，通过。
  - `sqlc generate`：不运行；本次没有 SQL、schema 或 sqlc 配置变化。
  - migration 测试：不运行；本次没有 migration 变化。
- 手工验证了哪些场景
  - 检查新测试覆盖 5 条真实响应路径，包含 4 条无需认证成功响应和 1 条认证失败响应。
  - 检查 `testAuthRouter(t)` 的 Env 补齐只影响测试夹具，不影响生产配置加载。
- Java-Go parity 如何验证
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md` 的 OpenAPI / Swagger 行，记录真实响应 schema contract 测试。
- 结果如何
  - 当前 5 条响应路径均能通过 OpenAPI schema 校验。

## 6. 已知限制
- 当前版本还缺什么
  - 只覆盖 5 条低依赖路径，尚未覆盖 register/login/refresh/logout、admin user 等复杂业务响应。
  - 当前测试重点是 response contract，没有调用 `ValidateRequest` 校验请求体契约。
- 哪些地方后面需要继续演进
  - 后续可以逐步增加认证成功路径、管理员分页、用户状态更新等响应 contract 用例。
  - 如未来 OpenAPI `servers` 调整，需要同步调整测试请求 URL 或 router matcher 配置。
- 与 Java 版仍有哪些差距
  - Java 版仍主要依赖 Spring MVC/Springdoc 生成侧绑定；Go 版当前通过测试门禁补齐 spec-first 与手写响应之间的差异。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 展示了真实 HTTP 响应与 OpenAPI schema 的自动化契约验证能力。
- 对微服务 / 云原生演进的影响
  - 后续 SDK 生成、网关配置或服务拆分依赖 OpenAPI 时，关键响应结构更不容易漂移。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续修改关键接口响应字段、状态码或 envelope 时，需要同步更新 OpenAPI schema，否则 contract test 会失败。
  - 不影响 migration、sqlc 或 repository 分层。
