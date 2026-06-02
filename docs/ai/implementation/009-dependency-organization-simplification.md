# Dependency Organization Simplification 实现说明

## 1. 本次改动解决了什么问题

本次解决 Go 版 auth/user/router 依赖组织中过度接口化和 option 装配模式带来的复杂度：

- handler 包不再重复声明只由单一 service 满足的 `AuthService` / `UserService` 接口。
- auth service 不再为 password、JWT、refresh token 和当前用户读取定义局部接口。
- router 不再使用 `RouterOption` / `WithAuth`，也不再创建 system service。
- server 不再接收 variadic router options，而是接收 app/bootstrap 已构建好的 `http.Handler`。

本次只收敛 Go 内部依赖组织，不改变 Java 对外契约、API 路径、错误码、JWT claims、refresh token hash 格式、数据库模型或 auth/session 业务语义。

## 2. 改动内容
- 新增了什么
  - `docs/ai/design/009-dependency-organization-simplification.md`。
  - `docs/ai/implementation/009-dependency-organization-simplification.md`。
  - `internal/http.RouterDependencies`，用于显式传入已构建好的 system/auth/user handler 和 auth middleware。
  - `internal/platform/db.TxRunner`，把事务运行能力接口放到能力所属 package。
  - `password.NewBCryptHasherWithCost`，用于测试中用具体类型和低 cost BCrypt，避免为了测试保留 auth service 局部接口。
- 修改了什么
  - `internal/http/handler/auth.Handler` 直接依赖 `*authsvc.Service`。
  - `internal/http/handler/user.Handler` 直接依赖 `*usersvc.Service`。
  - `internal/service/auth.Service` 直接依赖 `*password.BCryptHasher`、`*jwt.Codec`、`*refresh.Manager`、`*usersvc.Service`，事务依赖改为 `platformdb.TxRunner`。
  - `internal/app/bootstrap.go` 创建 system/auth/user service、handler、middleware，并调用 `NewRouter` 和 `NewServer`。
  - `internal/http/router.go` 只注册全局 middleware、路由和 handler 方法。
  - `internal/http/server.go` 接收已构建好的 `http.Handler`，只管理标准库 HTTP server 生命周期。
  - `internal/http/auth_integration_test.go` 改为真实 handler + 真实 service + fake repository。
  - `internal/service/auth/service_test.go` 改为真实 security 具体类型 + fake repository。
  - `docs/ai/parity/java-go-parity-matrix.md` 增加 Go 内部依赖组织收敛记录。
- 删除了什么
  - 删除 handler 包内 `AuthService` / `UserService` 接口。
  - 删除 auth service 内部 `PasswordHasher`、`TokenIssuer`、`RefreshTokenManager`、`UserReader` 局部接口。
  - 删除 `RouterOption`、`routerOptions`、`WithAuth`。
- 是否更新 Java-Go parity 记录
  - 已更新。新增记录说明这是 Go 端内部依赖组织收敛，不改变 Java 版 API、错误码、安全和数据库契约。
- ADR 判断
  - 未新增 ADR。本次没有引入新的架构方向，而是落实 AGENTS.md 7.8、ADR-0005 package layout 和 ADR-0010 service 控制事务边界的既有决策。

## 3. 为什么这样设计
- 关键设计原因
  - handler 默认依赖具体 service，避免重复表达 service contract。
  - password/JWT/refresh token 已有能力所属 package 和具体类型，auth service 无需再定义局部接口。
  - repository interface 保留，因为它是 service 到持久化层的稳定边界，能屏蔽 repository/mysql、sqlc 和 `database/sql`。
  - 事务运行接口放到 `internal/platform/db`，对应 ADR-0010 的平台事务能力边界，同时让 service 单元测试可以继续使用 fake repository + no-op transaction runner。
  - bootstrap 作为 composition root 负责对象创建，router 只绑定 URL、HTTP method、中间件和 handler 方法。
  - server 接收 `http.Handler` 后职责更单一：监听地址、超时、启动和优雅关闭。
- 与 Go 项目当前阶段的匹配点
  - 认证模块已经进入真实 service/repository/security 分层，继续保留临时接口会让后续 refresh/logout/admin users 装配越来越绕。
  - 显式 `RouterDependencies` 比 functional options 更适合内部应用装配，也更容易在测试中看清哪些路由被注册。
- 与 Java 版业务语义的对齐方式
  - Java 的 Spring DI / Controller / Service 结构不逐行迁移；Go 只保持业务语义和 API 契约一致。
  - register/login/me 的请求、响应、错误码、JWT claim 和 refresh token hash 仍由现有测试和 parity contract 锁定。

## 4. 替代方案
- 方案 A：保留 handler 内部 service interface，只移除 router options。
  - 没有采用。handler 内接口不是稳定边界，且会继续驱动 HTTP 测试使用 fake service，而不是真实 handler + service。
- 方案 B：保留 auth service 内部 password/JWT/refresh/user reader 接口。
  - 没有采用。这些接口主要服务 mock，能力所属 package 已有明确具体类型。
- 方案 C：让 `NewServer` 接收 `RouterDependencies` 并内部调用 `NewRouter`。
  - 没有采用。这样 server 仍知道 router 依赖形状，生命周期和路由装配职责没有完全分开。
- 方案 D：让 router 接收 service 并创建 handler。
  - 没有采用。router 会继续承担对象装配职责，不符合 bootstrap 作为 composition root 的方向。
- 方案 E：auth service 直接依赖 `repository/mysql` 具体实现。
  - 没有采用。会破坏 `service -> repository -> sqlc/database` 边界，并让 sqlc/database 细节向 service 泄漏。

## 5. 测试与验证
- 跑了哪些测试
  - `go test ./...`：通过。
  - `make test`：通过，当前 Makefile target 执行 `go test ./...`。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w`：已对改动 Go 文件执行。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `make test`：通过。
  - `git diff --check`：通过。
  - `golangci-lint run`：未运行，当前环境 `command -v golangci-lint` 无输出且退出码为 1。
  - `sqlc generate`：未运行，本次未修改 SQL、migration 或 `sqlc.yaml`。
  - OpenAPI validate：未运行，Go 版当前仍没有 OpenAPI 契约文件。
- 手工验证了哪些场景
  - 通过 HTTP 集成测试验证注册、重复注册、登录、错误密码、`/me`、禁用用户旧 token。
  - 通过 grep 确认生产代码中不再存在 `RouterOption`、`WithAuth`、handler 内 `AuthService` / `UserService`、auth service 局部 password/JWT/refresh/user reader 接口。
- Java-Go parity 如何验证
  - 对照 `docs/ai/parity/java-auth-api-contract.md`。
  - 本次未改任何 API/JWT/refresh token/DB 契约；parity matrix 只新增 Go 内部依赖组织说明。
- 结果如何
  - 可运行验证均通过；lint 因工具缺失未运行。

## 6. 已知限制
- 当前版本还缺什么
  - 仍未实现 refresh/logout/admin users。
  - Go OpenAPI YAML 仍未落地。
- 哪些地方后面需要继续演进
  - 后续 auth refresh/logout 增加 handler/service 时继续由 bootstrap 装配，通过 `RouterDependencies` 显式传入 router。
  - 如果 bootstrap 装配继续变大，可拆 `buildSystemDependencies`、`buildAuthDependencies` 等私有函数，但不把装配迁回 router。
  - 如果未来更多 service 需要事务运行能力，继续复用 `platformdb.TxRunner`，不要在每个 service package 内重复定义局部接口。
- 与 Java 版仍有哪些差距
  - Java 的 Spring 容器装配不是 Go 版迁移目标；Go 版采用显式 composition root。
  - Java 已有更多 auth 管理端和 token 生命周期能力，Go 本次仍只维护 register/login/me/access token 阶段。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 依赖组织更贴近企业级 Go 后端：composition root 显式装配、router 只绑路由、handler 依赖具体 service、repository interface 作为持久化边界。
- 对微服务 / 云原生演进的影响
  - `NewServer` 接收标准 `http.Handler`，后续服务模板、网关或测试 harness 更容易替换 router。
  - `RouterDependencies` 明确表达模块路由启用条件，便于不同 profile 或服务拆分阶段复用。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 新增业务模块时优先创建 service/handler/middleware，再由 bootstrap 显式传入 router。
  - repository/mysql、sqlc、migration 不受本次影响；后续新增 SQL 仍需 `sqlc generate` 和 migration 测试。
  - HTTP 测试优先使用真实 handler + service + fake repository，而不是为了 mock 在生产 handler 中新增接口。
