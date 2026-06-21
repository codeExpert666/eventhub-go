# OpenAPI policy 测试实现说明

## 1. 本次改动解决了什么问题

本次为 Go 版 EventHub 新增 OpenAPI policy 测试，把团队 API 规范从人工审查规则落到 `go test`：

- operation 必须有 `operationId`、可读说明和 tag。
- 业务接口必须声明 `application/json` 响应。
- 业务接口 2xx JSON 响应必须在最外层直接引用 `ApiResponse`，或通过最外层 `allOf` 组合 `ApiResponse`。
- 非 2xx 业务错误响应必须集中引用 `components.responses`，且被引用的组件响应必须统一使用 `ErrorResponse` schema。
- `ErrorResponse` schema 自身必须顶层复用 `ApiResponse` envelope。
- `/api/v1/admin/**` 必须声明 `BearerAuth` 与 `x-required-roles: [ADMIN]`。
- 当前认证边界保持与 router 一致：`register/login/refresh` 无 `BearerAuth`，`logout/me` 有 `BearerAuth`。

这解决了 admin 角色仅写在 description 中、后续无法机器校验的问题，也为后续新增接口提供防漂移门禁。

## 2. 改动内容
- 新增了什么
  - `api/openapi/openapi_policy_test.go`：
    - 使用 `github.com/getkin/kin-openapi/openapi3` 加载并 validate `eventhub.yaml`。
    - 遍历 paths/operations，执行 API policy 检查。
    - 通过 `schemaUsesTopLevelComponent` 校验 2xx 响应最外层 envelope，只沿 `$ref` 和顶层 `allOf` 展开。
    - 通过 `componentErrorResponseViolation` 校验业务非 2xx 引用到的 `components.responses.*` 统一指向 `ErrorResponse`。
    - 通过 `errorResponseEnvelopeViolation` 校验 `ErrorResponse` 自身顶层复用 `ApiResponse`。
    - 新增 helper 单测覆盖直接 `$ref`、顶层 `allOf` 和嵌套属性误判场景。
    - 新增 helper 单测覆盖组件响应 schema 漂移和 `ErrorResponse` envelope 漂移。
    - 失败信息包含 method、path、operationId。
  - 设计文档：`docs/ai/design/022-openapi-policy-test.md`。
  - 实现说明：`docs/ai/implementation/022-openapi-policy-test.md`。
- 修改了什么
  - `api/openapi/eventhub.yaml`：
    - `GET /api/v1/admin/users` 增加 `x-required-roles: [ADMIN]`。
    - `PATCH /api/v1/admin/users/{userId}/status` 增加 `x-required-roles: [ADMIN]`。
  - `docs/ai/design/022-openapi-policy-test.md`：
    - 补充错误响应组件策略、`ErrorResponse` envelope 策略和对应测试策略。
  - `go.mod` / `go.sum`：
    - 新增 `github.com/getkin/kin-openapi v0.131.0` 作为测试解析依赖，版本与 Makefile 的 `KIN_OPENAPI_VERSION` 一致。
  - `docs/ai/parity/java-go-parity-matrix.md`：
    - OpenAPI / Swagger 行新增 `openapi_policy_test.go`、admin role vendor extension、错误响应组件统一性和 Go test policy 门禁索引。
- 删除了什么
  - 未删除文件。
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次触及 OpenAPI 契约质量门禁、admin 角色契约表达和 Go spec-first 与 Java Springdoc/Spring Security 的刻意差异。

## 3. 为什么这样设计
- 关键设计原因
  - 直接使用 `kin-openapi` 解析 OpenAPI 语义对象，避免对 YAML 做字符串匹配。
  - policy test 放在 `api/openapi` 包内，靠 `runtime.Caller` 定位同目录 `eventhub.yaml`，不依赖测试运行时工作目录。
  - `schemaUsesTopLevelComponent` 只检查直接 `$ref`、组件引用展开和顶层 `allOf`，使 `ApiResponseUserInfo` 这类组合 schema 能通过统一 envelope 校验，同时避免 properties/items 等子 schema 中出现过 `ApiResponse` 就被误判为合格。
  - 错误响应先要求 operation 非 2xx 指向 `components.responses`，再要求被引用组件统一指向 `ErrorResponse`，避免只检查 `$ref` 前缀后把漂移风险转移到组件定义里。
  - `ErrorResponse` 自身继续通过顶层 `allOf` 复用 `ApiResponse`，让错误响应和成功响应共享 `code/message/data/requestId/timestamp` 外层结构。
  - admin 角色用 OpenAPI vendor extension `x-required-roles` 表达，保持和运行时 `RequireRole("ADMIN")` 同步，又不改变 generated server interface 或运行时逻辑。
  - auth security policy 用显式表记录当前事实，防止后续给 register/login/refresh 误加 BearerAuth，也防止误删 logout/me 的 BearerAuth。
- 与 Go 项目当前阶段的匹配点
  - 本次只触碰 OpenAPI 契约、测试和文档，不进入 handler/service/repository/sqlc/database 分层。
  - 使用现有 Go test 质量门禁，不引入 Node/Spectral 工具链。
  - `make openapi-check` 仍负责标准 validate、生成和 generated file 漂移检查；policy test 负责团队规范。
- 与 Java 版业务语义的对齐方式
  - Java controller 注解中的 operation/tag/summary 语义由 Go OpenAPI operation policy 固化。
  - Java 统一响应与分页语义由 Go `ApiResponse` / `PageResponse` schema policy 固化。
  - Java/Spring Security 管理员角色约束由 Go router 的 RBAC middleware 执行，OpenAPI 通过 `x-required-roles` 做机器可验证记录。

## 4. 替代方案
- 方案 A：只依赖 `make openapi-validate`。
  - 没有采用。OpenAPI 标准校验不能表达团队统一 envelope、admin role、错误响应集中引用等业务规范。
- 方案 B：引入 Spectral ruleset。
  - 没有采用。当前仓库主要质量门禁是 Go test / Makefile；Spectral 会增加 Node 工具链和 CI 维护成本。
- 方案 C：改造 router/handler，通过运行时元数据生成 OpenAPI。
  - 没有采用。会扩大到运行时注解模型或 generated interface 接入，超出本次 policy test 的目标。
- 方案 D：把 logout 当公开接口移除 BearerAuth。
  - 没有采用。当前 `internal/http/router.go` 中 logout 位于 `AuthMiddleware` 保护组，OpenAPI 必须反映现有实际行为。
- 方案 E：只检查 operation 是否引用 `components.responses`，不检查组件定义内部 schema。
  - 没有采用。该方案无法阻止 `components.responses.BadRequest` 漂移到 `ApiResponseVoid`、内联 object 或缺少统一错误 schema。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./api/openapi -run TestSchemaUsesTopLevelComponentRequiresEnvelope -count=1` 在收紧 helper 前失败，明确暴露：
    - `nested ApiResponse property is not a top-level envelope` 得到 `true`，期望 `false`。
    - `inline property with ApiResponse is not a top-level envelope` 得到 `true`，期望 `false`。
  - GREEN：收紧 helper 后同一命令通过。
  - RED：`go test ./api/openapi` 在补 spec 前失败，明确报：
    - `GET /api/v1/admin/users (operationId=listAdminUsers) must declare x-required-roles as a string array`
    - `PATCH /api/v1/admin/users/{userId}/status (operationId=updateAdminUserStatus) must declare x-required-roles as a string array`
  - GREEN：补充 `x-required-roles` 后 `go test ./api/openapi` 通过。
  - RED：`go test ./api/openapi -run TestCentralizedErrorResponsesRejectsComponentSchemaDrift -count=1` 初始失败，暴露当前 helper 未发现组件响应 schema 漂移。
  - GREEN：补充 `componentErrorResponseViolation` 并接入 `assertCentralizedErrorResponses` 后，同一命令通过。
  - RED：`go test ./api/openapi -run TestErrorResponseSchemaRequiresApiResponseEnvelope -count=1` 初始失败，暴露缺少 `ErrorResponse` 自身 envelope 校验 helper。
  - GREEN：补充 `errorResponseEnvelopeViolation` 并接入 `TestOpenAPIPolicy` 后，同一命令通过。
  - `go test ./api/openapi -count=1`：通过。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `make lint`：通过，输出 `0 issues.`。
  - `make openapi-check`：通过，包含 validate、generate 和 generated file diff 检查。
  - `git diff --check`：通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w api/openapi/openapi_policy_test.go`：已运行。
  - `go test ./api/openapi -count=1`：已运行，通过。
  - `go test ./...`：已运行，通过。
  - `go vet ./...`：已运行，通过。
  - `make lint`：已运行，通过。
  - `make openapi-check`：已运行，通过。
  - `git diff --check`：已运行，通过。
  - `sqlc generate`：未运行；本次没有 SQL、schema 或 sqlc 配置变化。
  - migration 测试：未单独运行；本次没有 migration 变化。
- 手工验证了哪些场景
  - 检查 `internal/http/router.go`，确认 `register/login/refresh` 不经过 `AuthMiddleware`，`logout/me/admin` 经过 `AuthMiddleware`，admin 再叠加 `RequireRole("ADMIN")`。
  - 检查 `make openapi-check` 没有造成 `api/openapi/gen/eventhub.gen.go` 漂移。
- Java-Go parity 如何验证
  - 对照 Java Springdoc/Spring Security 的 operation/tag 与管理员角色约束，在 Go spec-first 契约中用 policy test 和 `x-required-roles` 表达。
  - 已更新 parity matrix 的 OpenAPI / Swagger 行。
- 结果如何
  - policy test 能阻止后续漏掉 operationId、tag、业务 JSON 响应、最外层统一响应 envelope、错误响应集中引用、admin BearerAuth 和 ADMIN 角色声明。

## 6. 已知限制
- 当前版本还缺什么
  - `x-required-roles` 只用于 OpenAPI policy 测试和文档表达，不参与运行时鉴权。
  - policy test 目前只为 admin API 强制角色声明，未来其他角色或权限粒度需要按业务模块继续扩展。
- 哪些地方后面需要继续演进
  - 后续新增 event/order/payment 等业务 API 时，可在 policy test 中加入分页、幂等 header、状态码或错误码细分规则。
  - 如果未来存在新的非 `ApiResponse` 业务特例，应先补设计说明，再在 policy test 中增加明确例外。
- 与 Java 版仍有哪些差距
  - Java 版依赖 Springdoc 注解扫描与 Spring Security 配置；Go 版继续使用 spec-first YAML 和 Go test policy。
  - Java OpenAPI 没有 `x-required-roles` 这个 vendor extension；这是 Go 版为机器校验 RBAC 文档一致性新增的工程约束。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - API 契约质量门禁更完整，能展示 spec-first、统一响应、RBAC 文档一致性和 Go test 自动化治理。
- 对微服务 / 云原生演进的影响
  - OpenAPI policy 可作为未来服务拆分、网关校验、SDK 生成和契约测试的前置规范。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续 API 变更需要同步 `eventhub.yaml` 并通过 `go test ./...` 与 `make openapi-check`。
  - 不影响 migration、sqlc 或 repository 分层。
