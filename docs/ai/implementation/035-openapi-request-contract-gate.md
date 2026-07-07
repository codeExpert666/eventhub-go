# OpenAPI Request Contract Gate 实现说明

## 1. 本次改动解决了什么问题

本文记录 design 035 / ADR-0027 的阶段化落地。阶段一已完成 `internal/http/validation` 到 `internal/http/requesterror` 的命名与职责收口；阶段二已完成 `OPENAPI_REQUEST_VALIDATION_ENABLED`、`OPENAPI_SPEC_PATH`、文件系统 spec loader 和 provider 启动期加载校验。

本次阶段三完成运行时 request contract gate：

- 在 `internal/http/contract` 中新增 `RequestValidator`，基于 `kin-openapi/openapi3filter` 执行 operation match 和 request validation。
- 将 contract middleware 作为 chi route-level middleware 接到 generated strict server wrapper 之前，覆盖 generated 参数绑定前的 path/query 校验。
- 覆盖 path、query、request body schema、`requestBody.required`、`Content-Type` 校验，并保留 body replay，确保 strict handler 后续仍能 decode。
- 将 kin-openapi validation error 映射到 `requesterror.InvalidParameters`、`requesterror.InvalidBody`、`requesterror.MalformedBody`、`requesterror.UnsupportedContentType`，再通过 `response.WriteError` 输出统一 envelope。
- 本阶段 security validation 采用 no-op bridge：`BearerAuth` / `x-required-roles` 暂不由 contract gate 执行，现有 generated security middleware、`Authenticate` 和 `RequireRole("ADMIN")` 继续保留，阶段四再迁移。

本次不改变对外 API path/method/字段、成功响应 envelope、JWT claim、service/domain/repository 或数据库行为；不恢复 Go embed，不恢复 `api/openapi/spec.go:SpecYAML()`。

## 2. 改动内容
- 新增了什么
  - `internal/http/contract.RequestValidator` 与 `NewRequestValidator`。
  - `RequestValidator.Middleware`：匹配 OpenAPI operation，构造 `openapi3filter.RequestValidationInput`，执行 request validation，并在失败时写出项目统一错误响应。
  - `requesterror.UnsupportedContentType`：只构造 HTTP request 相关 `AppError`，不执行校验。
  - 真实 router 测试：invalid query、invalid path、unsupported content-type、malformed JSON、required body、schema violation、valid request body replay、validation disabled 行为。
  - provider 测试：`OPENAPI_REQUEST_VALIDATION_ENABLED=true` 时实际启用 contract gate，false 时即使 spec path 缺失也保持当前 strict-server 行为。
- 修改了什么
  - `ProviderHTTP` 在 request validation enabled 时先 `LoadSpec`，再创建 `RequestValidator`，并把 middleware 传入 `RouterDependencies`。
  - `RouterDependencies` 新增 `RequestContract func(http.Handler) http.Handler`，只表达 router 所需最终 middleware，不负责创建对象。
  - `registerOpenAPIRoutes` 使用 `router.With(deps.RequestContract)` 包住 generated routes，保证 contract gate 先于 generated wrapper 的 path/query 参数绑定执行。
  - `contract.NewRequestValidator` 基于 spec 文档浅拷贝清空 `servers` 后建立 matcher，避免运行时请求因 doc server URL 与 `httptest` 或部署 host 不一致而无法匹配。
  - `docs/ai/parity/java-go-parity-matrix.md` 更新阶段三状态。
- 删除了什么
  - 没有删除生产代码。
  - 没有恢复 Go embed，也没有恢复 `SpecYAML()`。
- 是否更新 Java-Go parity 记录
  - 已更新。runtime request contract gate 已进入阶段三已落地状态；security requirement / `x-required-roles` 迁移仍标为后续阶段。

## 3. 为什么这样设计
- 关键设计原因
  - Contract gate 放在 generated wrapper 外层，才能在 oapi-codegen path/query 参数绑定前拦截 schema 不合法的 path/query；如果放进 `ChiServerOptions.Middlewares`，会晚于 generated 参数绑定。
  - 使用 `openapi3filter.ValidateRequest` 保持 OpenAPI 语义执行集中化，handler 内现有字段校验暂不删除，避免本阶段扩大行为改动。
  - Body validation 后依赖 kin-openapi 的 body replay 机制重置 `r.Body` / `GetBody`，valid request 仍进入 strict handler 并完成 generated decode。
  - Security 使用 `NoopAuthenticationFunc` 是明确的阶段三临时策略；当前认证/授权仍由现有 middleware 执行，阶段四再接管 `BearerAuth` 和 `x-required-roles`。
- 与 Go 项目当前阶段的匹配点
  - 新能力只落在 `internal/http/contract`、`internal/http/requesterror`、provider 和 router 装配层，service/domain/repository 不依赖 HTTP contract 或 generated OpenAPI model。
  - `OPENAPI_REQUEST_VALIDATION_ENABLED=false` 时不传入 contract middleware，保持当前 strict-server 行为。
  - 继续复用既有 `kin-openapi` 依赖，不引入新 web framework。
- 与 Java 版业务语义的对齐方式
  - Java/Spring 通过 MVC 参数绑定、Bean Validation 和统一异常处理完成请求契约治理。
  - Go 端通过 spec-first OpenAPI contract gate、`requesterror` 和 `response.WriteError` 显式复现 path/query/body/content-type 校验与统一错误 envelope。

## 4. 替代方案
- 方案 A：直接使用 `openapi3filter.Validator.Middleware` 或 `ValidationHandler`。
- 方案 B：把 contract gate 放入 generated `ChiServerOptions.Middlewares`。
- 方案 C：本阶段同时迁移 BearerAuth 与 `x-required-roles`。
- 方案 D：删除 handler 内已有字段校验，完全依赖 OpenAPI schema。
- 为什么没有采用
  - 不采用方案 A：默认 middleware 同时考虑 response validation，错误输出和项目 `AppError` envelope 不一致；项目需要持有错误映射、security bridge 和后续指标审计边界。
  - 不采用方案 B：generated middleware 晚于参数绑定，无法覆盖 invalid path 这类阶段三目标。
  - 不采用方案 C：用户明确要求本阶段暂不迁移认证/授权；阶段四再处理 BearerAuth 和角色授权。
  - 不采用方案 D：handler 校验还承载少量业务友好消息和防御性边界，本阶段只新增 contract gate，不扩大行为收敛范围。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./internal/http -run 'TestOpenAPIRequestContractGate' -count=1` 先失败，失败原因为 `contract.NewRequestValidator` 和 `RouterDependencies.RequestContract` 尚不存在。
  - `go test ./internal/http/contract ./internal/http/requesterror ./internal/http -run 'TestLoadSpec|TestUnsupportedContentType|TestOpenAPIRequestContractGate' -count=1`：通过。
  - `go test ./internal/app/providers -run 'TestProviderHTTP(AppliesOpenAPIContract|SkipsOpenAPIContract|LoadsOpenAPIContract|RejectsMissingOpenAPIContract)' -count=1`：通过。
  - `go test ./internal/http/contract ./internal/http ./internal/app/providers -count=1`：通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt`：已对本次修改的 Go 文件执行。
  - `go test ./internal/http/contract ./internal/http -count=1`：通过。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `make openapi-check`：通过；本次未修改 OpenAPI YAML 或 generated code，命令确认生成物无漂移。
  - `git diff --check`：通过。
  - `sqlc generate`：未运行，本次不涉及 SQL、schema、repository 或 sqlc 配置。
- 手工验证了哪些场景
  - 通过真实 router 测试验证 invalid query、invalid path 在现有 auth 之前由 contract gate 统一返回 `COMMON-400`。
  - 验证 unsupported content-type 映射为 `请求内容类型不支持`。
  - 验证 malformed JSON 和缺失 required body 映射为 `请求体格式不合法`。
  - 验证 body schema violation 映射为 `请求体参数校验失败`。
  - 验证 valid `application/json; charset=utf-8` 请求仍进入 strict handler 并正确回显，证明 body replay 生效。
  - 验证 request validation disabled 时 `text/plain` + JSON body 仍保持当前 strict-server 可解码行为。
- Java-Go parity 如何验证
  - 主 parity matrix 已记录阶段三新增 runtime request contract middleware、统一错误映射和当前 security no-op bridge。
  - 本阶段没有改变 Java-Go 对外 API 契约，只让 Go 端更严格执行既有 OpenAPI 契约。
- 结果如何
  - 阶段三定向测试、全量 Go 测试、vet、OpenAPI check 和 diff whitespace check 均已通过。

## 6. 已知限制
- 本阶段还没有迁移 BearerAuth 和 `x-required-roles`；contract gate 里的 security bridge 是明确 no-op，现有 security middleware 继续保留。
- header/cookie 参数当前没有新增业务覆盖；如果后续 spec 声明 header/cookie 参数，`ValidateRequest` 可执行对应校验，但本阶段测试只覆盖 path/query/body/content-type。
- validation error 的中文字段级消息当前是稳定分类消息加字段名，未做完整本地化规则映射；后续可按 endpoint/字段补更细的用户提示。
- 未删除 handler 内现有字段校验，后续需要在 contract gate 稳定后按 endpoint 逐步评估是否收敛重复校验。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - OpenAPI 已从文档和代码生成源推进为运行时契约执行边界，能展示 spec-first 后端治理能力。
- 对微服务 / 云原生演进的影响
  - 显式文件系统 `OPENAPI_SPEC_PATH` 便于容器、Kubernetes ConfigMap/镜像资产、网关或 sidecar 共享同一份 spec。
  - contract gate 可在服务内作为 defense-in-depth，也可与未来 API gateway request validation 协同。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 阶段四应把 BearerAuth 和 `x-required-roles` 从现有 generated security middleware 迁入 `internal/http/contract`，并保持 JWT claim 不包含角色、邮箱、用户名和用户状态。
  - 后续如收紧未知 query/header/cookie 策略，应补 OpenAPI policy 和真实 router 测试。
  - 不影响 migration、sqlc 或 OpenAPI generated code。
