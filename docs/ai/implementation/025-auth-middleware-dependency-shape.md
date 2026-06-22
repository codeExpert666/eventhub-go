# Auth Middleware 依赖形态收敛实现说明

## 1. 本次改动解决了什么问题

本次解决认证 middleware 装配形态与项目其他 middleware 不一致的问题：

- `internal/http/middleware/auth.go` 不再暴露 `AuthMiddleware` wrapper、`NewAuth` 构造函数以及仅服务测试的局部 capability interface。
- `RouterDependencies` 不再接收 `*middleware.AuthMiddleware` 并调用 `.Middleware`。
- `AuthDeps` 不再把具体 middleware wrapper 作为字段层层传给 HTTP provider 和 router。

本次只收敛 Go 内部依赖组织，不改变认证业务语义、HTTP API、错误码、JWT claim、当前用户加载策略、RBAC、数据库模型、OpenAPI 契约或 Java-Go 对外契约。

## 2. 改动内容
- 新增了什么
  - 新增 `docs/ai/design/025-auth-middleware-dependency-shape.md`。
  - 新增本实现说明。
  - 新增 `middleware.Authenticate(tokens *jwt.Codec, principals *usersvc.Service) func(http.Handler) http.Handler`。
- 修改了什么
  - `internal/http/middleware/auth.go`：
    - 将认证 middleware 改为直接返回 chi 可使用的 middleware 函数。
    - 保留 Bearer token 解析、JWT 解析、principal 加载、context 注入和错误映射逻辑。
  - `internal/http/router.go`：
    - `RouterDependencies.AuthMiddleware` 改为 `RouterDependencies.Authenticate`。
    - protected group 使用 `protected.Use(deps.Authenticate)`。
  - `internal/app/providers/auth.go`：
    - `AuthDeps.Middleware` 改为 `AuthDeps.Authenticate`。
    - `ProviderAuth` 负责调用 `middleware.Authenticate(jwtCodec, user.Service)` 创建最终 middleware 函数。
  - `internal/app/providers/http.go`：
    - 汇总 `auth.Authenticate` 到 router dependencies。
  - `internal/http/auth_integration_test.go`：
    - `testAuthRouter(t)` 改为传入 `Authenticate` middleware 函数。
  - `internal/http/middleware/auth_test.go`：
    - 从 fake principal loader 调整为真实 user service + fake repository，覆盖认证 middleware 与主体加载边界。
- 删除了什么
  - 删除 `AccessTokenParser`。
  - 删除 `PrincipalLoader`。
  - 删除 `AuthMiddleware`。
  - 删除 `NewAuth`。
- 是否更新 Java-Go parity 记录
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
  - 本次属于 Go 内部依赖组织与装配边界收敛，不改变 Java 对外契约。

## 3. 为什么这样设计
- 关键设计原因
  - router 应只绑定 URL、HTTP method、handler 方法和最终 middleware 函数，不应知道认证 middleware 由 JWT codec 与 user service 构造。
  - provider 是 composition root，适合连接 `jwt.Codec`、`user.Service` 和认证 middleware。
  - `Authenticate` 返回 `func(http.Handler) http.Handler` 后，router 使用方式与 `RequestID`、`Recover`、`RequireRole` 更一致。
  - `jwt.Codec` 和 `usersvc.Service` 是当前稳定的具体依赖，没有必要为测试便利在 middleware 包内额外定义局部接口。
- 与 Go 项目当前阶段的匹配点
  - 保持 `handler -> service -> repository -> sqlc/database`。
  - 保持 `internal/app/providers` 装配对象、`internal/http/router.go` 只注册路由和 middleware。
  - 测试使用真实 service + fake repository，符合当前仓库避免生产代码过度接口化的方向。
- 与 Java 版业务语义的对齐方式
  - Java `JwtAuthenticationFilter` 仍对应 Go `Authenticate` middleware。
  - Java `AuthenticatedPrincipalService.loadByUserId` 仍对应 Go `user.Service.LoadPrincipal`。
  - Go 版不迁移 Spring filter bean 装配方式，而是用 provider 创建最终 middleware 函数。

## 4. 替代方案
- 方案 A：保留 `AuthMiddleware` wrapper，仅把字段名改为 `Authenticate`。
  - 没有采用。router 仍需要知道具体 wrapper 和 `.Middleware` 方法，过度包装问题没有解决。
- 方案 B：把 `jwt.Codec` 和 `user.Service` 传到 router，由 router 调用 `middleware.Authenticate(...)`。
  - 没有采用。router 会知道认证链路的构造材料，偏离“router 只绑定路由和 middleware”的边界。
- 方案 C：新增 `SecurityDeps` 并一路传到 router。
  - 没有采用。当前问题只需要传最终 middleware 函数，新增安全聚合结构会扩大装配 API。
- 方案 D：保留 `AccessTokenParser` / `PrincipalLoader` 接口。
  - 没有采用。这两个接口主要服务 mock，不是稳定业务边界；当前测试可以用真实 user service + fake repository 覆盖。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./internal/http/middleware` 失败，失败原因是 `Authenticate` 尚未实现。
  - GREEN：`go test ./internal/http/middleware ./internal/http ./internal/app/providers` 通过。
  - `go test ./...` 通过。
  - `make test` 通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w internal/http/middleware/auth.go internal/http/middleware/auth_test.go internal/http/router.go internal/app/providers/auth.go internal/app/providers/http.go internal/http/auth_integration_test.go`：已运行。
  - `go vet ./...`：通过。
  - `make vet`：通过。
  - `make lint`：通过，输出 `0 issues.`。
  - `git diff --check`：通过。
  - `sqlc generate`：未运行，本次未修改 SQL、migration 或 `sqlc.yaml`。
  - migration 测试：未单独运行，本次未修改 migration。
  - OpenAPI validate / generate：未运行，本次未修改 OpenAPI 契约或生成配置。
- 手工验证了哪些场景
  - `rg` 检查生产 Go 代码中不再存在 `NewAuth`、`AuthMiddleware`、`AuthMiddleware:`、`auth.Middleware` 或 `.Middleware` 的认证 wrapper 调用。
  - `rg` 旧符号扫描脚本输出 `no old auth middleware wrapper symbols in production code`。
  - 检查 `router.go` 只使用 `deps.Authenticate`，不接触 `jwt.Codec` 或 `usersvc.Service`。
  - 检查 `ProviderAuth` 仍是创建 JWT codec、auth service、handler 和认证 middleware 的唯一运行时装配点。
- Java-Go parity 如何验证
  - 对照 Java `JwtAuthenticationFilter` 与 `AuthenticatedPrincipalService`，确认 token 解析、按 `sub` 加载主体、禁用用户旧 token 返回 `AUTH-401` 的语义不变。
  - 更新 parity matrix，把本次 Go 内部装配形态收敛索引到 design/implementation 025。
- 结果如何
  - 可运行验证均通过；本次不涉及 SQL、migration 或 OpenAPI 生成。

## 6. 已知限制
- 当前版本还缺什么
  - 认证 middleware 仍不校验 `sid` 对应 auth session status，也没有 access token denylist；这与现有 logout no-op 设计一致。
  - 当前用户和角色加载仍每次回库，没有 Redis 短缓存。
- 哪些地方后面需要继续演进
  - 如果后续实现 session revoke、denylist 或 principal cache，应在 provider 中装配新的认证能力，router 仍只接收最终 middleware 函数。
  - 如果未来 JWT codec 被更多 provider 共享，可以新增 provider 内部 `SecurityDeps`，但不把它传入 router。
- 与 Java 版仍有哪些差距
  - Java 使用 Spring Security filter bean 装配；Go 版刻意使用显式 provider 和 chi middleware 函数。
  - 该差异是 Go idiom 下的装配方式差异，不改变业务语义。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 认证链路装配边界更清楚：provider 创建依赖，router 只使用最终 middleware。
  - 删除测试驱动的局部接口和 wrapper，代码更贴近企业级 Go 项目的显式依赖风格。
- 对微服务 / 云原生演进的影响
  - 未来认证链路加入缓存、session status 或 denylist 时，可以在 provider 内演进，不需要扩大 router 对安全组件的认知。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续新增业务 middleware 时优先判断是否有业务依赖；有依赖的 middleware 由 provider 装配成最终函数后传给 router。
  - SQL、migration、sqlc、OpenAPI 不受本次影响。
