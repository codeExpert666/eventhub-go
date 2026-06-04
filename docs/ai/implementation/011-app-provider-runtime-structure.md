# App Provider 运行时代码落地实现说明

## 1. 本次改动解决了什么问题

本次把 `docs/ai/design/010-app-provider-constructor-rules.md` 中记录的结构债落到运行时代码：

- 删除 `internal/service/auth` 中仅用于字段转移的 `Dependencies` struct。
- 把 `internal/app/bootstrap.go` 中的细节装配拆到 `internal/app/providers`。
- 新增 `internal/app/application.go`，集中应用级资源持有和 `Close` 释放逻辑。
- 将 `Bootstrap()` 改为 `Bootstrap(ctx context.Context)`，让启动期 DB 初始化使用调用方传入的进程 context。
- 把带依赖字段的 auth middleware 收敛为 `AuthMiddleware` 结构体加 `NewAuth` 构造函数。
- 补齐启动期依赖装配错误和运行期主错误的可观测性：`Bootstrap` 为 provider 错误增加装配阶段上下文，`Run` 为 HTTP server 非预期错误增加生命周期阶段上下文，`main` 对 `app.Run()` 返回的未处理错误做结构化日志记录后再退出。

本次不改变 HTTP API、错误码、数据库模型、repository 语义、JWT claim、refresh token hash 或 auth 业务流程。

## 2. 改动内容
- 新增了什么
  - 新增 `internal/app/application.go`：放置 `Application`、`NewApplication`、`Close`。
  - 新增 `internal/app/providers/platform.go`：加载 config，初始化 logger/clock，并按 DSN 创建可选 MySQL DB。
  - 新增 `internal/app/providers/system.go`：装配 system service 和 handler。
  - 新增 `internal/app/providers/user.go`：在 DB 可用时装配 user repository、service、handler。
  - 新增 `internal/app/providers/auth.go`：在 DB 和 user 能力可用时装配 auth service、handler、auth middleware。
  - 新增 `internal/app/providers/http.go`：汇总 `RouterDependencies`，创建 router 和 HTTP server。
  - 新增 `internal/app/providers/http_test.go`：验证 DB 为空时只注册 system routes，auth 和 `/api/v1/me` 继续返回统一 404。
  - 新增 `internal/app/bootstrap_test.go`：验证 platform provider 启动失败时，`Bootstrap` 返回带装配阶段上下文的错误。
  - 新增 `internal/app/lifecycle_test.go`：验证 HTTP 端口被占用导致 server 启动失败时，`Run` 返回带 `run http server` 阶段上下文的错误。
  - 新增设计文档 `docs/ai/design/011-app-provider-runtime-structure.md` 和本实现说明。
- 修改了什么
  - `cmd/eventhub/main.go` 在 `app.Run()` 返回错误时写出结构化错误日志，再以非零状态码退出。
  - `internal/app/bootstrap.go` 只保留主装配流程，后续 provider 失败时会关闭已打开的 DB。
  - `internal/app/bootstrap.go` 对 `ProviderPlatform`、`ProviderAuth` 返回的错误增加 `provide <module> dependencies` 上下文，避免启动错误裸返回。
  - `internal/app/lifecycle.go` 先创建进程信号 context，再调用 `Bootstrap(ctx)`。
  - `internal/app/lifecycle.go` 对 HTTP server 非预期停止错误返回 `run http server: ...`，不再在 `Run` 内记录同一个主错误，避免与 `main` 重复日志。
  - `internal/service/auth.NewService` 改为显式参数，删除 `authsvc.Dependencies`。
  - `internal/http/middleware.NewAuth` 改为返回 `*AuthMiddleware`，通过 `Middleware` 方法接入 router。
  - `internal/http/router.go` 的 `RouterDependencies.AuthMiddleware` 改为 `*middleware.AuthMiddleware`，router 仍不创建 service。
  - 更新 auth service 测试、auth middleware 测试、HTTP auth 集成测试的构造方式。
- 删除了什么
  - 删除 `internal/service/auth.Dependencies`。
  - 删除 `bootstrap.go` 内部的 `buildRouterDependencies` 细节装配函数。
- 是否更新 Java-Go parity 记录
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
  - 本次属于 Go 内部 composition root 和构造函数边界落地，不改变 Java 业务契约。

## 3. 为什么这样设计
- 关键设计原因
  - 显式构造参数让 auth service 的真实依赖直接可见，避免字段转移型 `Dependencies` 把 composition root 形状下沉到业务层。
  - `internal/app/providers` 能降低 `bootstrap.go` 的膨胀，同时把 provider 聚合结构体限制在 composition root 内部。
  - `Application` 单独成文件后，应用级资源释放不再和启动装配细节混在一起。
  - `Bootstrap(ctx)` 对 DB ping 等启动期阻塞操作更明确，后续接入 Redis、外部探活或启动超时时可复用同一模式。
  - auth middleware 有真实依赖字段，使用结构体 + `NewAuth` 更符合当前依赖组织规则。
  - 启动期必需依赖失败采用 fail fast，但必须可观测：provider 只返回错误，`Bootstrap` 补充阶段上下文，`main` 作为进程最终边界记录日志并退出。
  - 运行期 HTTP server 非预期错误同样采用“中间生命周期层包装、进程入口统一记录”的规则，避免同一个主错误打印两次。
- 与 Go 项目当前阶段的匹配点
  - 保持 `handler -> service -> repository -> sqlc/database`。
  - repository interface 仍是 service 到持久化层的边界。
  - router 只注册 URL、HTTP method、handler 和 middleware，不创建业务对象。
  - 未配置 MySQL DSN 时，user/auth provider 返回空能力，HTTP provider 只注册 system routes。
  - 装配错误不进入 HTTP 统一响应；它们属于进程启动失败，而不是业务请求错误。
- 与 Java 版业务语义的对齐方式
  - Java 使用 Spring DI 和 Lombok 构造器注入；Go 不迁移容器模型。
  - Go 通过显式 composition root 复现 Controller/Service/Mapper 分层语义，并保持 auth API、错误码、JWT 与数据库语义不变。
  - Java 版启动失败由 Spring Boot 与 Logback 输出；Go 版用 `slog` 在 `main` 兜底记录启动/运行失败，这是 Go 进程边界实现差异，不改变业务契约。
- ADR 记录
  - 本次未新增 ADR。composition root、provider 聚合边界、显式构造参数优先和启动 context 边界已经在既有 AGENTS 规则、ADR-0005、design/implementation 009/010 中确定；启动错误日志属于这些规则下的可观测性细化，不是新的关键架构取舍。

## 4. 替代方案
- 方案 A：保留 `authsvc.Dependencies`，只拆 `internal/app/providers`。
  - 没有采用。用户目标明确要求移除业务组件中仅用于字段转移的依赖结构体。
- 方案 B：引入 DI 容器或代码生成器模拟 Spring 自动装配。
  - 没有采用。当前项目规模下显式 provider 更可读，也避免新增重量级依赖。
- 方案 C：把 `PlatformDeps/AuthDeps/UserDeps` 放到 service 或 handler 包。
  - 没有采用。provider 聚合结构体只应存在于 composition root；下沉会污染业务层构造 API。
- 方案 D：顺手拆分 auth token/session service，减少 `authsvc.NewService` 参数数量。
  - 没有采用。本次范围是装配边界落地；业务职责拆分会扩大风险，应等 refresh/logout 或 session 用例继续迁移时单独设计。
- 方案 E：只在 provider 或 `Run` 内记录启动装配错误，`main` 继续只退出。
  - 没有采用。provider 内记录会把进程级可观测策略下沉到装配细节；`main` 是最稳定的最终兜底边界，能覆盖启动期和运行期所有未处理错误。
- 方案 F：`Run` 对 HTTP server 非预期错误先记录日志，再把原错误返回给 `main`。
  - 没有采用。这会让同一个运行期主错误在 `Run` 和 `main` 各打印一次；`Run` 只包装上下文、`main` 统一记录更一致。

## 5. 测试与验证
- 跑了哪些测试
  - `go test ./...`：通过。
  - `make test`：通过，当前 target 执行 `go test ./...`。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w internal/app internal/http internal/service/auth`：已运行。
  - `gofmt -w cmd/eventhub/main.go internal/app/bootstrap.go internal/app/bootstrap_test.go`：已运行，用于本次启动错误可观测性补强。
  - `gofmt -w internal/app/lifecycle.go internal/app/lifecycle_test.go`：已运行，用于本次运行期主错误包装调整。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `make test`：通过。
  - `make vet`：通过。
  - `git diff --check`：通过。
  - `golangci-lint run`：未运行，当前环境 `command -v golangci-lint` 退出码为 1，工具不可用。
  - `sqlc generate`：未运行，本次未修改 SQL、migration 或 `sqlc.yaml`。
  - migration 测试：未单独运行，本次未修改 migration；`go test ./...` 中 repository/mysql 集成测试按环境策略执行。
  - OpenAPI validate：未运行，本次未修改 API 契约，Go 版当前没有 OpenAPI 文件。
- 手工验证了哪些场景
  - `rg` 检查 Go 代码中已无 `authsvc.Dependencies`、`Dependencies struct` 或 `Bootstrap()` 调用。
  - 检查 `internal/app/bootstrap.go` 只表达 provider 调用主流程。
  - 检查 `ProviderPlatform(ctx)` 是当前唯一接收启动期 context 的 provider；纯对象装配 provider 不接收或保存启动 `ctx`。
  - 检查 `cmd/eventhub/main.go` 会记录 `app.Run()` 返回的错误后再退出。
  - 检查 `internal/app/lifecycle.go` 不再记录 HTTP server 非预期主错误，只包装后返回给 `main`；`Application.Close` 失败仍在 defer 内记录，避免清理错误丢失。
  - 检查 provider 没有业务规则，auth/user DB 为空时返回空能力。
  - 检查 router 继续只使用传入的 handler/middleware。
- Java-Go parity 如何验证
  - 对照 Java `EventhubApplication.java`、`SecurityConfig.java`、`AuthController.java`、`AuthServiceImpl.java` 和 `JwtAuthenticationFilter.java`，确认本次只是 Go 显式装配方式调整。
  - 已更新 parity matrix 的“Go 内部依赖组织与装配边界”行，说明结构债已落地。
  - 对照 Java `logback-spring.xml`，确认 Java 版日志由 Spring Boot / Logback 负责；Go 版在 `main` 兜底记录未处理错误属于进程边界实现差异，不改变 API、错误码、数据库模型、JWT 或业务语义。
- 结果如何
  - 可运行验证均通过；lint 因本机工具缺失未运行。

## 6. 已知限制
- 当前版本还缺什么
  - auth service 构造参数较多，显示出 auth 注册、登录、token、session 能力后续可以继续拆分。
  - provider 暂未接入 Redis、OpenAPI、事件/订单等后续模块。
- 哪些地方后面需要继续演进
  - refresh/logout 迁移时评估拆分 token/session use case，降低 auth service 构造复杂度。
  - 后续新增 Redis 或外部探活时继续使用 `Bootstrap(ctx)` 传入的进程 context。
  - 后续如引入启动重试、readiness、外部依赖健康状态或分级降级模式，应继续区分“必需依赖 fail fast”和“显式允许的降级能力”。
  - 更多业务模块加入时继续按 `providers/<module>.go` 只做对象连接。
- 与 Java 版仍有哪些差距
  - Java 的 Spring 容器装配不会迁移到 Go，这是刻意差异。
  - Go 版仍需用显式代码维护装配可读性；这比 Spring 自动装配更冗长，但边界更直接。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - composition root、provider 分工、显式构造参数和启动 context 边界更加清晰，能体现 Go 后端工程组织能力。
- 对微服务 / 云原生演进的影响
  - `Bootstrap(ctx)`、providers 拆分和启动错误结构化日志方便后续接入启动超时、依赖探活、模块裁剪、服务拆分和可观测性。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 新业务模块应由 provider 装配 service/handler/middleware，router 不创建 service。
  - service/handler/domain/repository 继续避免字段转移型 `Dependencies/Deps/Options`。
  - SQL、migration、OpenAPI 未受本次影响；未来相关变更仍需运行对应生成和校验命令。
