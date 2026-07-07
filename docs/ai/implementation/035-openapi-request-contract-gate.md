# OpenAPI Request Contract Gate 实现说明

## 1. 本次改动解决了什么问题

本文记录 design 035 / ADR-0027 的阶段化落地。阶段一已完成 `internal/http/validation` 到 `internal/http/requesterror` 的命名与职责收口；阶段二已完成 `OPENAPI_REQUEST_VALIDATION_ENABLED`、`OPENAPI_SPEC_PATH`、文件系统 spec loader 和 provider 启动期加载校验；阶段三已完成 path/query/body/content-type request contract gate。

本次阶段四完成 OpenAPI security requirement runtime 化：

- `internal/http/contract` 新增 AuthenticationFunc / security bridge，operation `security: BearerAuth` 成为触发现有认证能力的事实来源。
- `x-required-roles` 在 contract gate 中读取并执行；当前 ADMIN-only operation 要求当前服务端 principal 拥有 `ROLE_ADMIN`。
- 认证发生在 request body validation 和 strict-server body decode 前；缺失或无效 token 统一映射 `AUTH-401`，缺少 ADMIN 统一映射 `AUTH-403`。
- `internal/http/openapi_routes.go` 移除基于 generated context `BearerAuthScopes` 和 `/api/v1/admin/` 路径前缀的临时 security middleware。
- provider 在装配到认证能力时会创建 OpenAPI security bridge；`OPENAPI_REQUEST_VALIDATION_ENABLED` 只控制 path/query/body/content-type 等 request validation，不能关闭认证/授权边界。

本次不改变对外 API path/method/字段、成功响应 envelope、JWT claim、service/domain/repository 或数据库行为；不恢复 Go embed，不恢复 `api/openapi/spec.go:SpecYAML()`。

## 2. 改动内容
- 新增了什么
  - `contract.WithAuthentication`：注入现有 `middleware.Authenticate` 作为 OpenAPI `AuthenticationFunc` 的认证桥。
  - `contract.WithRequestValidation`：允许 provider 在 request validation disabled 时仍保留 security-only contract middleware。
  - `contract` 内部的 `requiredRoles`、role normalize、principal authority 检查和 security error -> `AppError` 映射。
  - OpenAPI policy 检查：`components.securitySchemes.BearerAuth` 必须存在且为 `type: http`、`scheme: bearer`。
  - 真实 router 测试：public operation、missing token、invalid token、普通用户访问 admin、admin 用户访问 admin，以及 `/api/v1/me` 临时标 ADMIN 时普通用户被拒绝，证明运行时不再依赖路径前缀。
- 修改了什么
  - `RequestValidator` 使用注入的 `AuthenticationFunc` 替代阶段三的 `NoopAuthenticationFunc`。
  - `RequestValidator.Middleware` 在 request validation enabled 时执行完整 `ValidateRequest`；在 disabled 但已注入认证能力时只执行 OpenAPI security requirement。
  - `ProviderHTTP` 在 `OPENAPI_REQUEST_VALIDATION_ENABLED=true` 或已装配 `auth.Authenticate` 时加载 `OPENAPI_SPEC_PATH` 并创建 contract middleware。
  - `RouterDependencies` 删除 `Authenticate` 字段，router 不再持有临时 security middleware 所需依赖。
  - `auth_integration_test` 的真实 router fixture 接入 request contract gate，认证/授权路径与生产装配一致。
  - README 与 `configs/*.env.example` 更新 `OPENAPI_SPEC_PATH` 说明：它同时服务 request validation 和 OpenAPI security bridge。
- 删除了什么
  - 删除 `openAPISecurityMiddleware`。
  - 删除 `requiresOpenAPIBearerAuth`。
  - 删除 `requiresOpenAPIAdminRole`。
  - 删除 `openapi_routes.go` 对 `internal/http/middleware.RequireRole("ADMIN")` 和路径前缀判断的直接依赖。
- 是否更新 Java-Go parity 记录
  - 已更新。parity matrix 记录阶段四已将 BearerAuth 和 `x-required-roles` 迁入 contract gate，并保留 JWT 不携带角色等动态属性的边界。

## 3. 为什么这样设计
- 关键设计原因
  - `kin-openapi` 的 `ValidateRequest` 会先执行 security requirement，再校验参数和 request body；这正好满足“未认证请求不应先因为 body 格式错误返回 COMMON-400”的安全顺序。
  - 通过桥接现有 `middleware.Authenticate`，JWT 解析、principal 加载、禁用用户判断和角色来源继续复用既有认证能力，避免在 contract 包重新实现 token 语义。
  - role 判断读取认证 middleware 写入 context 的 `security.Principal.Authorities`，因此角色仍来自服务端 principal/role 查询结果，不写入 JWT。
  - security-only 模式避免 `OPENAPI_REQUEST_VALIDATION_ENABLED=false` 误伤认证/授权；该开关只影响请求参数和 body 契约校验。
- 与 Go 项目当前阶段的匹配点
  - 改动只落在 `internal/http/contract`、HTTP provider/router 装配、OpenAPI policy test、真实 router 测试和 docs/parity。
  - service/domain/repository 不依赖 `api/openapi/gen`、`internal/http/contract` 或 `internal/http/requesterror`。
  - `internal/http/requesterror` 仍只构造 HTTP request 相关 `AppError`，不执行校验。
- 与 Java 版业务语义的对齐方式
  - Java/Spring Security filter chain 根据受保护 operation 执行认证/授权；Go 端现在由 OpenAPI operation security metadata 驱动现有认证 middleware。
  - Java 动态权限不写入 JWT；Go 端继续用 JWT 稳定身份 claim 回库加载 principal 与 roles。

## 4. 替代方案
- 方案 A：继续保留 `openAPISecurityMiddleware`，只把 admin path prefix 改成更复杂的 path/method 表。
- 方案 B：在 contract 包中重新解析 JWT、查询用户和角色。
- 方案 C：把角色写入 JWT，AuthenticationFunc 只读 token claim。
- 为什么没有采用
  - 不采用方案 A：仍会让 OpenAPI `x-required-roles` 只是文档元数据，运行时与 spec 可能漂移。
  - 不采用方案 B：会复制现有认证能力，容易让 token 解析、禁用用户判断和 principal 加载语义分叉。
  - 不采用方案 C：违反 AGENTS.md 和 ADR-0027 的 JWT claim 边界；角色、邮箱、用户名、状态等动态属性必须服务端查询或受控缓存获得。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./internal/http -run 'TestOpenAPIContractSecurityBridge' -count=1` 先失败，失败原因为缺 token + malformed body 返回 `COMMON-400`，以及 `/api/v1/me` 临时标 ADMIN 后普通用户仍可访问。
  - `go test ./api/openapi ./internal/http/contract ./internal/http -count=1`：通过。
  - `go test ./internal/app/providers -count=1`：通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt`：已对本次修改的 Go 文件执行。
  - 后续本轮最终验证继续运行 `go test ./...`、`go vet ./...`、`make openapi-check`、`make openapi-lint`、`git diff --check`。
  - `sqlc generate`：未运行，本次不涉及 SQL、schema、repository 或 sqlc 配置。
- 手工验证了哪些场景
  - 通过真实 router 测试验证 public operation 不需要 token。
  - 验证 missing token 在 body decode 前返回 `AUTH-401`。
  - 验证 invalid token 返回 `AUTH-401`。
  - 验证普通 USER token 访问 ADMIN operation 返回 `AUTH-403`。
  - 验证 ADMIN token 可访问 ADMIN operation。
  - 验证角色判断来自 operation `x-required-roles`，不是 `/api/v1/admin/` 路径前缀。
- Java-Go parity 如何验证
  - 主 parity matrix 已记录 OpenAPI security requirement 与 `x-required-roles` runtime 化，并说明 Go 端仍复用服务端 principal/role 查询结果。
- 结果如何
  - 阶段四定向测试与相关 provider 测试已通过；全量质量门禁结果见本轮最终总结。

## 6. 已知限制
- header/cookie 参数当前没有新增业务覆盖；如果后续 spec 声明 header/cookie 参数，`ValidateRequest` 可执行对应校验，但测试仍主要覆盖 path/query/body/content-type/security。
- validation error 的中文字段级消息当前是稳定分类消息加字段名，未做完整本地化规则映射；后续可按 endpoint/字段补更细的用户提示。
- 未删除 handler 内现有字段校验，后续需要在 contract gate 稳定后按 endpoint 逐步评估是否收敛重复校验。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - OpenAPI 已从文档和代码生成源推进为运行时请求契约与安全策略事实来源，能展示 spec-first 后端治理能力。
- 对微服务 / 云原生演进的影响
  - 显式文件系统 `OPENAPI_SPEC_PATH` 便于容器、Kubernetes ConfigMap/镜像资产、网关或 sidecar 共享同一份 spec。
  - contract gate 可在服务内作为 defense-in-depth，也可与未来 API gateway request validation 协同。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续如收紧未知 query/header/cookie 策略，应补 OpenAPI policy 和真实 router 测试。
  - 不影响 migration、sqlc 或 OpenAPI generated code。
