# Dependency Organization Simplification 设计

## 1. 背景
- 当前 Go 版已实现 auth 注册、登录、`/me`、JWT access token、refresh token hash 和 auth session 最小闭环，但依赖组织仍带有早期演进痕迹：
  - `internal/http/handler/auth` 和 `internal/http/handler/user` 在 handler 包内重复定义 service 接口。
  - `internal/service/auth/service.go` 为 password、JWT、refresh token、user reader 定义局部接口，接口不属于稳定业务边界。
  - `internal/http/router.go` 使用 `RouterOption` / `WithAuth` 注入 auth 依赖，并在 router 内部创建 system service。
  - `internal/http/server.go` 通过 variadic router options 间接创建 router。
- Java 版对应语义 / 文档 / 代码来源：
  - `docs/ai/parity/java-auth-api-contract.md`
  - Java `AuthController`、`UserController`、`AuthServiceImpl`、`TokenServiceImpl`、`AuthenticatedPrincipalService`
  - Go 既有 `docs/ai/design/008-auth-register-login-access-token.md`
  - Go 既有 `docs/ai/implementation/008-auth-register-login-access-token.md`
  - ADR-0002、ADR-0005、ADR-0010、ADR-0011、ADR-0012、ADR-0013
- 业务上下文不变：这是认证和当前用户模块的内部工程结构收敛，不改变 EventHub 对客户端暴露的接口、错误码、JWT claim、refresh token hash、数据库模型或 Java-Go parity 语义。

## 2. 目标
- 删除 handler 包内重复 service 接口：
  - `auth.Handler` 直接依赖 `*authsvc.Service`。
  - `user.Handler` 直接依赖 `*usersvc.Service`。
- 收敛 auth service 内部局部接口：
  - 删除 `PasswordHasher`、`TokenIssuer`、`RefreshTokenManager`、`UserReader` 等局部接口。
  - password、JWT、refresh token 直接依赖能力所属 package 的具体类型。
  - 当前用户读取直接依赖 `*usersvc.Service`。
  - 事务运行接口如需保留，放在 `internal/platform/db`，因为它是平台事务能力边界，已由 ADR-0010 固化。
- 保留 `internal/repository` 下 repository interface，作为 service 到持久化层的稳定边界。
- 移除 router functional options：
  - 删除 `RouterOption`、`routerOptions`、`WithAuth`。
  - 新增显式 `RouterDependencies`，由调用方传入已构建好的 handler 和 middleware。
- router 不再创建 service：
  - `internal/app/bootstrap.go` 创建 system/auth/user service、handler、middleware。
  - `internal/http/router.go` 只负责中间件和路由绑定。
- 调整 server 构造：
  - `NewServer` 接收已构建好的 `http.Handler`。
  - server 只管理监听地址、超时和生命周期，不再了解 router dependencies。
- 成功标准：
  - API 契约、错误码、JWT claims、refresh token hash 格式、数据库模型和现有测试覆盖不下降。
  - `gofmt`、`go test ./...`、`go vet ./...`、`make test`、可用的 `golangci-lint run`、`git diff --check` 完成或说明不可运行原因。

## 3. 非目标
- 不改变 `POST /api/v1/auth/register`、`POST /api/v1/auth/login`、`GET /api/v1/me` 的路径、HTTP 方法、请求字段、响应字段、状态码或错误消息。
- 不改变 JWT claim 边界；仍只包含 `iss/sub/iat/exp/jti/sid/typ=access`。
- 不改变 refresh token 明文格式、hash 格式或 TTL 语义。
- 不改变 `users`、`roles`、`user_roles`、`auth_sessions` 表结构、索引、唯一约束、migration 或 sqlc query。
- 不实现 refresh/logout/admin user 等新业务能力。
- 不把 handler 改成直接访问 repository、sqlc、`database/sql` 或 Redis。
- 不引入新的 Web 框架、DI 容器或 mock 生成工具。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `internal/app`
  - `internal/http/router.go`
  - `internal/http/server.go`
  - `internal/http/handler/auth`
  - `internal/http/handler/user`
  - `internal/http/auth_integration_test.go`
  - `internal/http/router_test.go`
  - `internal/platform/db`
  - `internal/security/password`
  - `internal/service/auth`
  - `docs/ai/design`
  - `docs/ai/implementation`
  - `docs/ai/parity`
- 不触及：
  - `internal/repository/mysql`
  - `internal/repository/mysql/queries`
  - `internal/repository/mysql/sqlc`
  - `migrations`
  - `api/openapi`
  - `internal/security/jwt` 的 claim 语义
  - `internal/security/refresh` 的 token/hash 语义
- 本次需要更新 `docs/ai/parity/java-go-parity-matrix.md`，说明 Go 端内部依赖组织收敛，不改变 Java 对外契约。
- 本次预计不新增 ADR：依赖组织方向已由 AGENTS.md 7.8、ADR-0005 和 ADR-0010 固化，本次是落地既有决策。如实现中出现新的架构取舍，再补 ADR。

## 5. 领域建模
- 领域对象不变：
  - `User`：仍通过 repository model 承接持久化字段，service 输出使用 `usersvc.UserResult`。
  - `Role`：仍使用 `roles.code`，HTTP 返回 `USER`，`Principal.Authorities` 使用 `ROLE_` 前缀。
  - `AuthSession`：登录成功仍创建 `ACTIVE` session，`session_id` 写入响应和 JWT `sid`。
  - `AccessToken`：仍是最小 JWT claim。
  - `RefreshToken`：仍是 opaque token，DB 保存 `sha256:<hex>`。
- 本次只调整依赖关系：
  - handler -> 具体 service。
  - auth service -> repository interface、平台事务能力、security 具体类型、user service。
  - app/bootstrap -> 负责具体对象创建和注入。
  - router -> 负责 URL / method / middleware / handler 方法绑定。

## 6. API 设计
- 对外 API 不变：
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `GET /api/v1/me`
  - system 和 actuator 端点不变。
- 请求参数、响应结构和错误码不变。
- 内部构造 API 调整：
  - `authhandler.NewHandler(auth *authsvc.Service) *Handler`
  - `userhandler.NewHandler(users *usersvc.Service) *Handler`
  - `apphttp.NewRouter(logger *slog.Logger, deps RouterDependencies) http.Handler`
  - `apphttp.NewServer(cfg config.Config, logger *slog.Logger, handler http.Handler) *Server`
- `RouterDependencies` 计划包含：
  - `System *systemhandler.Handler`：必需，用于 system / actuator 路由。
  - `Auth *authhandler.Handler`：可选，存在时注册 auth register/login。
  - `User *userhandler.Handler`：可选，配合 `AuthMiddleware` 注册 `/api/v1/me`。
  - `AuthMiddleware func(http.Handler) http.Handler`：可选，存在时保护当前用户路由。
- 未配置数据库时，bootstrap 不创建 auth/user service 与 middleware，router 只注册 system / actuator 路由，保持现有“不注册 auth routes”的行为。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无变化。
- 唯一约束：无变化。
- migration 计划：无新增 migration。
- sqlc query / generated model 影响：无变化，不需要 `sqlc generate`。
- 数据一致性考虑：
  - 注册仍由 auth service 在事务内创建用户并绑定默认角色。
  - 登录仍由 auth service 在事务内创建 ACTIVE auth session 后签发 token。
  - service 继续通过 repository interface 访问持久化语义，不 import `repository/mysql`、sqlc generated code 或 `database/sql`。

## 8. 关键流程
- 启动装配流程：
  1. `Bootstrap` 加载配置和 logger。
  2. `Bootstrap` 创建 system service 和 system handler。
  3. 如果未配置 MySQL DSN，返回只包含 system handler 的 `RouterDependencies`。
  4. 如果配置 MySQL DSN，`Bootstrap` 创建 DB、repository/mysql、user service、auth service、JWT codec、auth middleware、auth handler、user handler。
  5. `Bootstrap` 调用 `NewRouter(logger, deps)` 构建 `http.Handler`。
  6. `Bootstrap` 调用 `NewServer(cfg, logger, router)` 构建 server。
- router 注册流程：
  - 始终注册 requestId 和 recover middleware。
  - 使用 `deps.System` 注册 system 和 actuator 路由。
  - `deps.Auth != nil` 时注册 register/login。
  - `deps.User != nil && deps.AuthMiddleware != nil` 时注册 `/api/v1/me` protected group。
- handler / service / repository / sqlc/database 分工：
  - handler：HTTP decode/validate、DTO 与 command/query/result 映射、响应写出。
  - service：业务规则、事务边界、token/session 创建。
  - repository interface：持久化语义边界。
  - repository/mysql：sqlc row 与 repository model 映射。
  - router：路由绑定，不创建 service。
  - server：HTTP 生命周期，不创建 router 依赖。

## 9. 并发 / 幂等 / 缓存
- 并发语义不变：
  - 注册唯一性仍由 DB unique constraint 兜底。
  - 登录仍允许多 session。
  - refresh token hash 和 session id 碰撞仍由唯一约束兜底。
- 幂等语义不变：
  - 注册不是幂等接口。
  - 登录不是幂等接口。
- 缓存策略不变：
  - 当前用户加载仍每次回 DB。
  - 本次不引入 Redis 缓存、denylist 或 session status 校验。
- 事务边界：
  - 事务能力接口收敛到 `internal/platform/db`，auth service 不再定义局部 `Transactor` 接口。
  - repository 测试和 service 单元测试可继续使用 fake repository + no-op transaction runner，不改变生产事务语义。

## 10. 权限与安全
- 匿名访问和鉴权边界不变：
  - `POST /api/v1/auth/register`、`POST /api/v1/auth/login` 匿名可访问。
  - `GET /api/v1/me` 需要合法 Bearer access token。
- JWT claim 边界不变：
  - 写入 `iss/sub/iat/exp/jti/sid/typ=access`。
  - 不写角色、权限、邮箱、用户名、用户状态、密码哈希。
- refresh token 安全边界不变：
  - 明文只返回一次。
  - DB 只保存 `sha256:<hex>`。
- 当前用户加载策略不变：
  - middleware 解析 JWT 后按 `sub` 加载最新用户状态和角色。
  - 禁用用户旧 token 继续返回 `AUTH-401`。

## 11. 测试策略
- 单元测试：
  - 更新 auth service 测试，使用真实 `*password.BCryptHasher`、`*jwt.Codec`、`*refresh.Manager` 和 fake repository。
  - 为测试速度可给 BCrypt 增加具体类型的低 cost 构造函数，不新增生产接口。
- service / repository 测试：
  - 保持 repository interface fake，用于验证注册、重复注册、unique constraint 映射、登录、错误密码、禁用用户。
  - repository/mysql 集成测试不受影响。
- migration / sqlc 验证：
  - 本次不改 SQL/migration，不运行 `sqlc generate`；如质量门禁中已有迁移测试则随 `go test ./...` 执行或跳过。
- 接口验证：
  - HTTP auth 集成测试改为真实 handler + 真实 service + fake repository，而不是 handler 依赖 fake service 接口。
  - 保持注册、重复注册、登录、错误密码、`/me`、禁用用户旧 token 覆盖。
- OpenAPI validate：
  - Go 版仍没有 OpenAPI 文件，本次不运行。
- 异常场景验证：
  - 缺失或无效 token 仍返回 `AUTH-401`。
  - 登录失败不创建 session。
- Java-Go parity 验证：
  - 对照 `docs/ai/parity/java-auth-api-contract.md` 和 parity matrix。
  - 明确记录本次是 Go 内部依赖组织收敛，不改变 Java 契约。
- 需要运行的命令：
  - `gofmt`
  - `go test ./...`
  - `go vet ./...`
  - `make test` 如 Makefile 可用
  - `golangci-lint run` 如工具可用
  - `git diff --check`

## 12. 风险与替代方案
- 当前方案风险：
  - handler 直接依赖具体 service 后，HTTP 测试需要更真实的 service/repository fixture，测试代码会略长。
  - `NewServer` 接收 `http.Handler` 后，调用方必须先显式创建 router；这提高了 bootstrap 责任，也让 server 更纯粹。
  - 事务 runner interface 移到 `platform/db` 后，需要命名清楚，避免它变成泛化依赖容器。
- 备选方案：
  - 方案 A：保留 handler 内部 service interface，只删除 router options。
  - 方案 B：auth service 继续保留 password/JWT/refresh 局部接口，便于测试 mock。
  - 方案 C：`NewServer` 接收 `RouterDependencies` 并在 server 内调用 `NewRouter`。
  - 方案 D：router 接收 service 并创建 handler。
  - 方案 E：auth service 直接依赖 `repository/mysql` 具体实现。
- 为什么不选备选方案：
  - 不选 A：handler 包内接口只是被单一具体 service 满足，重复表达 service contract，增加阅读和构造成本。
  - 不选 B：这些能力已有所属 package 和具体类型，局部接口只为测试 mock 服务，不是稳定业务边界。
  - 不选 C：server 的职责会继续掺入 router 依赖装配，和“server 只管理生命周期”的目标不一致。
  - 不选 D：router 会继续承担对象装配职责，不利于 `internal/app` 作为 composition root。
  - 不选 E：会破坏 `service -> repository -> sqlc/database` 边界，让 sqlc/database 细节向 service 扩散。
- 后续可演进点：
  - 管理员用户 API 落地时继续沿用 `RouterDependencies` 的显式 handler 注入。
  - refresh/logout 落地时继续由 bootstrap 创建 handler/service/middleware，不恢复 functional options。
  - 如果未来模块很多，可在 `internal/app` 内拆分 `buildAuthDependencies`、`buildSystemDependencies` 等装配函数，但不把装配迁回 router。
