# App Provider 运行时代码落地设计

## 1. 背景
- `docs/ai/design/010-app-provider-constructor-rules.md` 已经把依赖组织规则固化为长期约束，但运行时代码仍保留两处结构债：
  - `internal/service/auth.NewService` 仍接收仅用于字段转移的 `authsvc.Dependencies`。
  - `internal/app/bootstrap.go` 同时承载配置、logger、DB、repository、service、handler、middleware、router 和 server 的细节装配。
- Java 版对应来源是 `EventhubApplication.java`、Spring Bean 构造器注入、`SecurityConfig.java`、`AuthController.java`、`AuthServiceImpl.java` 和 `JwtAuthenticationFilter.java`。Java 通过 Spring DI 自动装配 Controller、Service、Mapper 和安全过滤器；Go 版不迁移 Spring 容器，而是用显式 composition root 保持同等分层语义。
- 本次是 Go 工程组织调整，不改变注册、登录、当前用户、system/actuator 的业务契约。

## 2. 目标
- 删除 auth service 中字段转移型 `Dependencies` struct，改为显式构造参数。
- 将 `internal/app/bootstrap.go` 中的细节装配拆分到 `internal/app/providers`。
- 新增 `internal/app/application.go`，放置 `Application`、`NewApplication` 和 `Close` 等应用级资源生命周期逻辑。
- 保持 `internal/app/lifecycle.go` 负责 `Run` 与进程信号生命周期，并让启动装配使用调用方传入的 context。
- 让依赖装配过程中产生的启动期错误能被开发者明确感知：
  - provider 只返回错误，不决定进程级日志策略。
  - `Bootstrap` 为 provider 错误补充装配阶段上下文。
  - `Run` 为 HTTP server 非预期停止等运行期主错误补充生命周期阶段上下文。
  - `main` 在进程最终边界统一记录 `app.Run()` 返回的未处理错误，再以非零状态码退出。
- 保持未配置 MySQL DSN 时只注册 system routes 的现有行为。
- 成功标准：
  - router 不创建 service。
  - provider 只做依赖装配，不写业务规则。
  - `handler -> service -> repository -> sqlc/database` 分层不变。
  - 现有 auth service 测试、middleware 测试、HTTP 集成测试和 router 测试继续通过。

## 3. 非目标
- 不新增或修改 HTTP API 路径、请求字段、响应字段、状态码、错误码或 OpenAPI 契约。
- 不修改数据库表、migration、sqlc query、repository 持久化语义或事务边界。
- 不修改 auth 注册、登录、refresh token hash、access token claim、当前用户加载策略或权限规则。
- 不引入 DI 容器、代码生成器或新的第三方依赖。
- 不拆分 auth service 的业务职责；构造参数较多的问题作为后续服务职责演进点记录。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `internal/app`
  - `internal/app/providers`
  - `cmd/eventhub/main.go`
  - `internal/service/auth`
  - `internal/http/middleware`
  - `internal/http/router.go`
  - 相关测试文件
- 不涉及 API / 表 / 缓存 / 外部接口。
- 影响 `docs/ai/parity/java-go-parity-matrix.md`：是。矩阵中已有“Go 内部依赖组织与装配边界”条目，本次需要把 `authsvc.Dependencies` 与 `Bootstrap()` 结构债更新为已落地状态。

## 5. 领域建模
- 本次没有新增业务领域实体。
- 工程组织对象：
  - `Application`：进程级应用对象，持有 logger、HTTP server 和数据库连接池等生命周期资源。
  - `PlatformDeps`：app/provider 内部装配期聚合，包含 config、logger、clock 和可选 MySQL DB。
  - `SystemDeps`：system service 与 handler 的装配结果。
  - `UserDeps`：user repository、service、handler 的装配结果；DB 为空时为空能力。
  - `AuthDeps`：auth service、handler 和认证 middleware 的装配结果；DB 为空时为空能力。
  - `HTTPDeps`：router 和 HTTP server 的装配结果。
- 与 Java 版领域对象的对应关系：
  - Java 的 Spring Bean 不作为 Go 领域对象迁移。
  - Go 通过 provider 聚合结构体复现 Spring composition 的装配职责，但业务实体和 service 语义不变。

## 6. API 设计
- 对外 HTTP API 不变。
- 内部构造 API 调整：
  - `authsvc.NewService(users, roles, sessions, transactor, passwords, tokens, refreshToken, userService, clock)`。
  - `middleware.NewAuth(tokens, principals)` 返回带依赖字段的 `*AuthMiddleware`，router 使用其 middleware 方法注册保护路由。
  - `app.Bootstrap(ctx context.Context) (*Application, error)`。
  - `app.NewApplication(logger, server, database)`。
- 错误码 / 异常场景：
  - 对外 HTTP 错误码不变。
  - 启动期装配错误不是 HTTP 业务错误，不进入统一响应 envelope。
  - 必需依赖失败时采用 fail fast：provider 返回 `error`，`Bootstrap` 用 `fmt.Errorf("<stage>: %w", err)` 补充装配阶段，`main` 用结构化日志记录最终错误并退出进程。
  - HTTP server 非预期停止时，`Run` 用 `fmt.Errorf("run http server: %w", err)` 补充生命周期阶段，不直接记录同一个主错误，避免与 `main` 重复日志。
  - 未配置 MySQL DSN 是当前允许的降级模式，不作为 error；user/auth 能力为空，router 只注册 system routes。
- 与 Java 版 OpenAPI / controller 契约的差异：无对外契约差异。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - service 仍持有事务边界，auth provider 只创建 `platform/db.Transactor`。
  - DB 连接池在 `Application.Close` 中释放；若 provider 后续装配失败，bootstrap 负责关闭已创建的 DB，避免启动失败时泄漏连接池。

## 8. 关键流程
- 正常启动流程：
  1. `Run` 创建进程信号 context。
  2. `Bootstrap(ctx)` 调用 `providers.ProviderPlatform(ctx)` 加载 config、logger、clock，并在 DSN 存在时初始化 MySQL。
  3. `providers.ProviderSystem` 装配 system service 和 handler。
  4. `providers.ProviderUser` 在 DB 存在时装配 user repository、service、handler；DB 为空时返回空能力。
  5. `providers.ProviderAuth` 在 DB 和 user 能力存在时装配 session repository、transactor、JWT codec、password hasher、refresh manager、auth service、auth handler 和 auth middleware；DB 为空时返回空能力。
  6. `providers.ProviderHTTP` 汇总 `RouterDependencies`，创建 router 和 HTTP server。
  7. `Bootstrap` 用 `NewApplication` 返回应用对象。
- 启动期装配错误流程：
  1. 必需依赖装配失败时，provider 返回明确错误；例如 DB 存在但 user 能力缺失时，auth provider 返回依赖缺失错误。
  2. `Bootstrap` 不吞错，不在 provider 内做进程级日志策略，而是为错误补充阶段上下文，例如 `provide auth dependencies: ...`。
  3. `Run` 继续把启动失败返回给调用方。
  4. `main` 作为进程最终边界记录结构化错误日志，再 `os.Exit(1)`，避免静默退出。
- 运行期主错误流程：
  1. `Run` 启动 HTTP server 并阻塞到服务停止。
  2. `http.ErrServerClosed` 表示优雅关闭，不作为错误返回。
  3. 其他 HTTP server 错误由 `Run` 包装为 `run http server: ...` 后返回，不在 `Run` 内记录同一个主错误。
  4. `main` 统一记录最终错误，避免同一个运行期错误打印两次。
  5. `Application.Close` 失败属于 defer 清理旁路错误，当前不合并进主返回值，仍由 `Run` 内部记录，避免资源释放失败静默丢失。
- 未配置 MySQL DSN 流程：
  1. `PlatformDeps.Database` 为空。
  2. user/auth provider 不创建 repository、service、handler 或 auth middleware。
  3. `RouterDependencies` 只包含 system handler。
  4. auth/user 路由不注册，继续返回统一 `COMMON-404`。
- handler / service / repository / sqlc/database 分工：
  - handler：HTTP 入参、DTO 映射、响应写出。
  - service：业务规则、事务边界和失败语义。
  - repository：持久化语义接口。
  - repository/mysql/sqlc：数据库访问细节。
  - provider：只装配对象，不承载业务判断。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及库存、订单、支付、幂等键或缓存。
- 启动阶段并发控制：
  - DB 初始化和 ping 使用 `Run` 传入的进程 context。
  - 收到退出信号时，启动期阻塞操作可以被取消。
- 事务边界不变：
  - `auth.Service.Register` 和 `auth.Service.Login` 继续通过 `platform/db.TxRunner` 控制事务。

## 10. 权限与安全
- 本次不改变认证、授权、JWT claim、refresh token 或安全上下文。
- JWT 仍只包含 `iss/sub/iat/exp/jti/sid/typ` 等稳定身份与技术 claim，不写角色、邮箱、用户名或用户状态。
- auth middleware 仍在每次请求中解析 Bearer token 后通过 user service 加载最新用户状态与角色。
- provider 不放权限判断，只连接 token codec、principal loader 和路由注册所需 middleware。

## 11. 测试策略
- 单元测试：
  - 更新 auth service 测试以使用显式构造参数。
  - 更新 auth middleware 测试以覆盖结构体型 middleware。
- service / repository 测试：
  - auth service fake repository 测试继续验证注册、登录、重复账号、禁用用户等失败语义。
- migration / sqlc 验证：
  - 不适用，本次不改 SQL、migration 或 sqlc 配置。
- 接口验证：
  - 更新 HTTP auth 集成测试构造方式。
  - 增加 provider/http 测试验证 DB 为空时只注册 system routes。
- OpenAPI validate：
  - 不适用，本次不改 API 契约，Go 版当前也没有 OpenAPI 文件。
- 异常场景验证：
  - provider 失败时关闭已打开 DB 的路径通过代码检查和 Go 测试覆盖可运行路径。
- Java-Go parity 验证：
  - 更新 parity matrix 中 Go 内部依赖组织与装配边界记录。
- 需要运行的命令：
  - `gofmt` / `make fmt`
  - `go test ./...`
  - `go vet ./...`
  - `golangci-lint run` 如工具可用
  - `make test`
  - `make vet`

## 12. 风险与替代方案
- 当前方案风险：
  - auth service 构造参数较多，虽然比字段转移型 `Dependencies` 更显式，但暴露出后续可拆分 token/session 业务能力的结构压力。
  - 新增 providers 包后，后续新增模块需要避免把业务判断写入 provider。
  - `Bootstrap(ctx)` 改签名会影响 app lifecycle 调用点，需要同步测试。
  - `Application.Close` 失败当前仍在 `Run` 的 defer 中记录，而不是用 `errors.Join` 合并到主错误；这保持实现简单，但主错误和清理错误仍是两条独立日志。
- 备选方案：
  - 方案 A：保留 `authsvc.Dependencies`，只拆 providers。
  - 方案 B：引入 DI 容器或代码生成来模拟 Spring 装配。
  - 方案 C：把 provider 聚合结构体下沉到 service 或 handler 包。
  - 方案 D：本次顺手拆出 TokenService / SessionService 降低 auth service 构造参数。
  - 方案 E：只在 `Run` 或 provider 内记录装配错误，`main` 继续静默退出。
  - 方案 F：`Run` 既记录 HTTP server 非预期错误，又返回给 `main`。
- 为什么不选备选方案：
  - 不选 A：这会留下用户明确要求删除的字段转移型依赖结构。
  - 不选 B：Go 版目标是显式 composition root，不迁移 Spring 容器模型，也不为当前规模引入重量级依赖。
  - 不选 C：会污染业务层构造 API，违反 provider 聚合结构体仅限 composition root 的规则。
  - 不选 D：会扩大业务重构范围；本次目标是装配边界落地，业务职责拆分应另开设计。
  - 不选 E：provider 内记录会把进程可观测策略下沉到装配细节；`Run` 在启动失败时可能还没有 `Application` logger，`main` 是最稳定的兜底边界。
  - 不选 F：会让同一个运行期主错误在 `Run` 和 `main` 各打印一次；包装后交给 `main` 统一记录更一致。
- 后续可演进点：
  - auth refresh/logout 落地时评估是否拆分 token/session use case，降低 auth service 构造参数。
  - 后续接入 Redis、OpenAPI 或更多业务模块时继续只在 providers 内做对象连接。
  - 后续如引入启动重试、readiness 或外部依赖健康状态，应保持 fail fast 与显式降级模式的边界清晰。
