# Auth Middleware 依赖形态收敛设计

## 1. 背景
- 当前认证 middleware 的运行时语义是正确的：解析 Bearer access token 后，根据 JWT `sub` 通过 user service 加载最新用户状态和角色，再把 `security.Principal` 写入请求 context。
- 现有实现的结构问题在于依赖形态偏重：
  - `internal/http/middleware/auth.go` 暴露 `AccessTokenParser`、`PrincipalLoader`、`AuthMiddleware` 和 `NewAuth`。
  - `RouterDependencies` 暴露 `*middleware.AuthMiddleware`，router 需要调用 `.Middleware` 才能接入 chi。
  - `AuthDeps` 把 auth service、handler 和具体 middleware wrapper 聚合在一起，使 `ProviderHTTP` 到 router 的传递链路显得绕。
- Java 版对应语义 / 文档 / 代码来源：
  - Java `JwtAuthenticationFilter`
  - Java `AuthenticatedPrincipalService`
  - Go ADR-0011 `docs/ai/adr/0011-jwt-claim-boundary.md`
  - Go ADR-0013 `docs/ai/adr/0013-current-user-loading-strategy.md`
  - `docs/ai/design/009-dependency-organization-simplification.md`
  - `docs/ai/design/011-app-provider-runtime-structure.md`
- 业务上下文不变：认证 middleware 仍是受保护路由和管理员 RBAC 的入口，本次只收敛 Go 内部装配形态，不改变认证业务语义。

## 2. 目标
- 将认证 middleware 从具体 wrapper 对象收敛为 chi 可直接使用的 middleware 函数：
  - `middleware.Authenticate(tokens *jwt.Codec, principals *usersvc.Service) func(http.Handler) http.Handler`
- 让 `RouterDependencies` 只接收最终装配好的认证 middleware 函数：
  - `Authenticate func(http.Handler) http.Handler`
- 让 router 与其他 middleware 一样只使用认证 middleware，不知道其构造材料是 JWT codec 和 user service。
- 保持 `internal/app/providers` 作为 composition root：
  - `ProviderAuth` 创建 JWT codec、auth service、auth handler 和认证 middleware 函数。
  - `ProviderHTTP` 只把 `auth.Authenticate` 汇总进 `RouterDependencies`。
- 成功标准：
  - 对外 API、错误码、JWT claim、当前用户加载、RBAC、OpenAPI 契约和数据库模型均不变。
  - `go test ./...`、`go vet ./...`、`make test`、`make vet` 和可用的 lint / diff 检查通过或说明不可运行原因。

## 3. 非目标
- 不修改 `POST /api/v1/auth/register`、`POST /api/v1/auth/login`、`POST /api/v1/auth/refresh`、`POST /api/v1/auth/logout`、`GET /api/v1/me` 或管理员用户接口。
- 不修改 JWT claim 边界；access token 仍不写角色、邮箱、用户名或用户状态。
- 不修改当前用户加载策略；受保护请求仍每次按 `sub` 回库加载最新用户和角色。
- 不修改 refresh token、auth session、logout no-op、RBAC 或管理员接口语义。
- 不修改数据库表、migration、sqlc query、OpenAPI YAML 或生成代码。
- 不把 `jwt.Codec`、`usersvc.Service` 或 repository 依赖传入 router 让 router 自行创建认证 middleware。
- 不引入新 interface、DI 容器、代码生成器或第三方依赖。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `internal/http/middleware`
  - `internal/http/router.go`
  - `internal/app/providers`
  - `internal/http` 测试夹具
  - `docs/ai/design`
  - `docs/ai/implementation`
  - `docs/ai/parity`
- 不涉及 API / 表 / 缓存 / 外部接口。
- 影响 `docs/ai/parity/java-go-parity-matrix.md`：是。本次属于 Go 内部依赖组织与装配边界收敛，需要记录这是 Go idiom 下的实现形态差异，不改变 Java 业务契约。

## 5. 领域建模
- 本次没有新增业务领域实体。
- 现有对象语义保持：
  - `jwt.Codec`：JWT access token 技术组件，负责签发、解析和验签。
  - `user.Service`：当前用户和认证主体加载用例，负责按用户 ID 加载最新状态与角色。
  - `security.Principal`：认证完成后写入 request context 的当前主体。
  - `Authenticate` middleware：HTTP 传输层认证链路，连接 JWT 解析、主体加载和统一错误响应。
- 与 Java 版领域对象的对应关系：
  - Java `JwtAuthenticationFilter` 仍对应 Go `Authenticate` middleware。
  - Java `AuthenticatedPrincipalService.loadByUserId` 仍对应 Go `user.Service.LoadPrincipal`。
  - Go 不迁移 Spring Security filter bean 装配方式，而是由 app provider 显式创建 middleware 函数。

## 6. API 设计
- 对外 HTTP API 不变。
- 内部构造 API 调整：
  - 删除 `middleware.NewAuth(tokens, principals) *AuthMiddleware`。
  - 新增 `middleware.Authenticate(tokens *jwt.Codec, principals *usersvc.Service) func(http.Handler) http.Handler`。
  - `RouterDependencies.AuthMiddleware *middleware.AuthMiddleware` 改为 `RouterDependencies.Authenticate func(http.Handler) http.Handler`。
  - `AuthDeps.Middleware *middleware.AuthMiddleware` 改为 `AuthDeps.Authenticate func(http.Handler) http.Handler`。
- 错误码 / 异常场景不变：
  - 缺失 token、格式错误、过期、签名错误、用户不存在或禁用：`AUTH-401`。
  - 主体加载出现非认证业务错误：`COMMON-500`。
- 与 Java 版 OpenAPI / controller 契约的差异：无对外契约差异。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 受保护请求仍回库读取最新用户状态和角色。
  - 本次不改变事务边界；认证 middleware 只读用户和角色，不开启业务事务。

## 8. 关键流程
- 正常流程：
  1. `ProviderAuth` 创建 `jwt.Codec`。
  2. `ProviderAuth` 创建 auth service 和 auth handler。
  3. `ProviderAuth` 调用 `middleware.Authenticate(jwtCodec, user.Service)` 得到最终 middleware 函数。
  4. `ProviderHTTP` 将该函数放入 `RouterDependencies.Authenticate`。
  5. `router.go` 在 protected route group 中调用 `protected.Use(deps.Authenticate)`。
  6. 请求进入认证 middleware 后解析 Bearer token，按 `claims.SubjectID` 加载 principal，再进入 handler。
- 异常流程：
  - 未配置数据库时，`ProviderUser` 和 `ProviderAuth` 返回空能力，`RouterDependencies.Authenticate` 为空，auth/user routes 不注册，保持当前 404 降级行为。
  - token 或 principal 加载认证失败时，认证 middleware 继续写 `AUTH-401`。
  - principal 加载出现非认证错误时，认证 middleware 继续写 `COMMON-500`。
- 状态流转：
  - 不改变用户状态、auth session 状态或 token 生命周期状态机。
- handler / service / repository / sqlc/database 分工：
  - router：只绑定 URL、HTTP method、handler 方法和 middleware 函数。
  - middleware：只处理 HTTP 认证链路、context 注入和错误响应。
  - service：继续承载用户状态与角色加载规则。
  - repository/sqlc：不受影响。
  - provider：创建并连接对象，不承载业务规则。

## 9. 并发 / 幂等 / 缓存
- 不涉及库存、订单、支付、幂等键或防重复提交。
- 不新增缓存。
- 不改变当前用户加载策略；每次受保护请求仍直接读取持久化层。
- middleware 函数闭包只持有启动期不可变依赖，不引入共享可变状态。

## 10. 权限与安全
- 认证与授权边界不变：
  - register/login/refresh 不走认证 middleware。
  - logout/me/admin routes 走认证 middleware。
  - admin routes 继续叠加 `RequireRole("ADMIN")`。
- JWT claim 边界不变：
  - 继续只使用稳定身份与技术 claim。
  - 不写角色、邮箱、用户名、用户状态或密码哈希。
- 敏感信息不新增日志或响应输出。
- 本次不实现 access token denylist、session status 校验或 Redis principal cache。

## 11. 测试策略
- 单元测试：
  - 更新 `internal/http/middleware/auth_test.go`，用 `Authenticate` 函数和真实 user service + fake repository 验证缺失 token、principal context、禁用用户旧 token、过期 token。
- service / repository 测试：
  - 不修改 service/repository 行为；既有测试继续覆盖。
- migration / sqlc 验证：
  - 不适用，本次不改 SQL、migration 或 sqlc 配置，不运行 `sqlc generate`。
- 接口验证：
  - 更新 `testAuthRouter(t)`，确保完整 router 仍穿过真实认证 middleware 函数。
  - 保持 auth integration、router contract、OpenAPI response contract 相关测试通过。
- OpenAPI validate：
  - 不适用，本次不改 OpenAPI 契约；如执行仓库 openapi 门禁则应无生成漂移。
- 异常场景验证：
  - 缺失 token：`AUTH-401`。
  - 过期 token：`AUTH-401`。
  - 禁用用户旧 token：`AUTH-401`。
  - 普通用户访问管理员接口：`AUTH-403`，由既有 RBAC 测试覆盖。
- Java-Go parity 验证：
  - 对照 Java `JwtAuthenticationFilter` 和 `AuthenticatedPrincipalService`，确认主体加载和错误语义不变。
  - 更新 parity matrix 中 Go 内部依赖组织记录。
- 需要运行的命令：
  - `gofmt`
  - `go test ./...`
  - `go vet ./...`
  - `make test`
  - `make vet`
  - `golangci-lint run` 或 `make lint` 如工具可用
  - `git diff --check`

## 12. 风险与替代方案
- 当前方案的风险：
  - `auth_test.go` 从局部 fake loader 转为真实 user service + fake repository 后，测试夹具略重。
  - 历史文档中仍会出现 `AuthMiddleware` 作为当时阶段的记录；本次不回写历史，只在新 implementation note 和 parity matrix 说明当前形态。
- 备选方案：
  - 方案 A：保持 `AuthMiddleware` wrapper，只把字段名从 `AuthMiddleware` 改为 `Authenticate`。
  - 方案 B：把 `jwt.Codec` 和 `user.Service` 传入 router，由 router 调用 `middleware.Authenticate(...)`。
  - 方案 C：新增 `SecurityDeps` 并传给 router。
  - 方案 D：保留 `AccessTokenParser` / `PrincipalLoader` 接口，让 `Authenticate` 接收接口。
- 为什么不选备选方案：
  - 不选 A：具体 wrapper 对象和 `.Middleware` 方法仍会泄漏到 router，不能解决过度包装。
  - 不选 B：router 会知道认证链路构造材料，偏离“router 只绑定路由和 middleware”的职责。
  - 不选 C：对当前问题过度抽象，且仍可能把安全装配细节泄漏到 router。
  - 不选 D：两个接口主要服务测试便利，不是稳定业务边界；当前仓库规则优先使用具体类型。
- 后续可演进点：
  - 如果未来认证链路需要 session status 校验、denylist 或 principal cache，可在 provider 中构造新的认证能力对象或函数，但 router 仍只接收最终 middleware 函数。
  - 如果未来多个模块共享 JWT codec，可新增 provider 内部 `SecurityDeps`，但避免把它传入 router。
