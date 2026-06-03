# App Provider 与构造函数规则补强设计

## 1. 背景
- 当前 Go 版已经通过 `docs/ai/design/009-dependency-organization-simplification.md` 收敛了过度接口化、router functional options 和 router 内部装配问题：
  - handler 默认依赖具体 service。
  - repository interface 保留为 service 到持久化层的稳定边界。
  - `internal/app/bootstrap.go` 负责创建 service、handler、middleware。
  - `internal/http/router.go` 只绑定 URL、HTTP method、中间件和 handler 方法。
- 但最新依赖组织规则还没有完整沉淀：
  - `AGENTS.md` 当前仍允许构造函数使用 `Dependencies` struct，容易被误用到 service 包中。
  - `internal/service/auth.Service` 当前使用 `authsvc.Dependencies`，该结构体主要做字段转移，暴露了后续需要收敛的结构债。
  - `Bootstrap()` 当前内部使用 `context.Background()`，对后续 DB、Redis、外部探活等启动初始化不够显式。
  - 当 bootstrap 装配继续变大时，需要明确 `internal/app/providers` 的长期落点，避免把装配细节迁回 router、service 或 handler。
- Java 版对应语义是 Spring 容器装配、Controller 依赖 Service、Service 依赖 Mapper/Repository 的分层方向；Go 版不迁移 Spring DI 容器，而是用显式 composition root 复现同等依赖边界。

## 2. 目标
- 在 `AGENTS.md` 中补强长期规则：
  - service、handler、具有依赖字段的 middleware 等业务组件保持“目标结构体 + 对应 New 函数”。
  - service、handler、domain、repository 包不使用仅用于字段转移的 `Dependencies` / `Deps` / `Options`。
  - 构造函数优先显式参数；参数过多时优先审视职责边界，而不是简单包一层依赖结构体。
  - `internal/app/providers` 可以使用 `PlatformDeps`、`AuthDeps`、`HTTPDeps` 等聚合结构体，但这些结构体只属于 composition root 装配过程。
  - `bootstrap.go` 只表达整体装配流程，具体细节拆到 `internal/app/providers`。
  - `Bootstrap` 优先使用 `Bootstrap(ctx context.Context)`，避免启动初始化隐式使用 `context.Background()`。
- 在 backend-design-first skill 中增加执行检查点，保持 skill 轻量，不复制 `AGENTS.md` 的详细规则。
- 更新 parity matrix，把这次规则补强记录为 Go 内部依赖组织的进一步固化。
- 成功标准：
  - 文档规则与既有 `handler -> service -> repository -> sqlc/database` 分层一致。
  - 不大范围重写历史文档。
  - 本次不改运行时代码，后续可用单独重构任务把 `authsvc.Dependencies` 和 `Bootstrap()` 落地为新规则。

## 3. 非目标
- 本次不重构 `internal/app/bootstrap.go` 到 `internal/app/providers`。
- 本次不修改 `authsvc.NewService` 签名，也不删除 `authsvc.Dependencies`。
- 本次不改变任何 HTTP API 路径、请求字段、响应字段、状态码或错误码。
- 本次不改变数据库表、migration、sqlc query、repository 行为、JWT claim 或 refresh token 语义。
- 本次不引入 DI 容器、代码生成器或新的第三方依赖。
- 本次不回写修改历史 implementation note，把历史记录改成“当时已经这么做”。

## 4. 影响范围
- 涉及文档：
  - `AGENTS.md`
  - `.agents/skills/backend-design-first/SKILL.md`
  - `docs/ai/design/010-app-provider-constructor-rules.md`
  - `docs/ai/implementation/010-app-provider-constructor-rules.md`
  - `docs/ai/parity/java-go-parity-matrix.md`
- 不触及运行时代码：
  - `internal/app`
  - `internal/service`
  - `internal/http`
  - `internal/repository`
  - `internal/security`
- 不涉及 API / 表 / 缓存 / 外部接口。
- 影响 `docs/ai/parity/java-go-parity-matrix.md`：是。本次固化 Go 内部依赖组织规则，属于为了 Go 自然写法而刻意偏离 Java Spring DI 结构，但保持业务语义一致。

## 5. 领域建模
- 本次没有新增业务领域实体。
- 规则层面的核心概念：
  - `Application`：进程生命周期内共享组件的聚合对象。
  - `Bootstrap(ctx)`：应用启动装配入口，接收调用方控制的 context。
  - `providers`：composition root 内部的模块化装配包，只负责创建和连接依赖。
  - `PlatformDeps` / `AuthDeps` / `HTTPDeps`：仅限 app/providers 使用的装配期聚合结构体。
  - `Service` / `Handler` / middleware 结构体：业务组件自身持有真实依赖，构造函数优先显式参数。
- 与 Java 版对应关系：
  - Java 的 Spring Bean wiring 不逐行迁移。
  - Go 使用显式 `NewXxx` 和 app/providers 作为 composition root，保持 Controller/Service/Repository 分层语义。

## 6. API 设计
- 对外 HTTP API 不变。
- 内部构造规则新增：
  - `NewService(dep1, dep2, ...) *Service` 优先于 `NewService(Dependencies{...})`。
  - `NewHandler(service *svc.Service) *Handler` 继续作为 handler 默认样式。
  - 具有状态或依赖字段的 middleware 可以使用目标结构体 + New 函数；纯函数型 middleware 保持普通 `NewXxx(...) func(http.Handler) http.Handler` 也可接受。
  - `Bootstrap(ctx context.Context) (*Application, error)` 作为后续启动装配演进目标。
- 错误码 / 异常场景：不变。
- 与 Java 版 OpenAPI / controller 契约的差异：无对外契约差异。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 本次只改规则文档，不改变事务边界。
  - 后续把 Bootstrap 改为接收 context 后，DB/Redis 初始化应沿用调用方 context，避免启动流程出现不可取消的阻塞。

## 8. 关键流程
- 后续推荐启动装配流程：
  1. `cmd/eventhub/main.go` 或 `app.Run()` 创建进程生命周期 context。
  2. 调用 `app.Bootstrap(ctx)`。
  3. `bootstrap.go` 加载 config、logger，并调用 providers。
  4. `providers/platform.go` 创建 DB、Redis、logger 等平台能力。
  5. `providers/system.go`、`providers/auth.go`、`providers/user.go` 创建对应 service、handler、middleware 依赖。
  6. `providers/http.go` 汇总 `RouterDependencies` 并创建 router/server 所需对象。
  7. provider 返回装配结果，bootstrap 组装 `Application`。
- handler / service / repository / sqlc/database 分工保持：
  - handler：HTTP decode/validate、DTO 映射、响应写出。
  - service：业务规则、事务边界、状态流转。
  - repository：持久化语义接口。
  - repository/mysql 与 sqlc/database：数据库访问细节。
  - providers：依赖装配，不承载业务规则。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及库存、订单、支付、幂等键或缓存。
- context 规则影响启动并发控制：
  - 启动期 DB/Redis 连接、ping、migration 检查或外部依赖探活应使用调用方 context。
  - 不在 bootstrap 内隐式创建不可取消的 `context.Background()`。
- 事务边界不变：
  - service 仍决定业务事务边界。
  - provider 只负责创建 transactor 或 repository，不决定业务流程如何提交或回滚。

## 10. 权限与安全
- 本次不改变认证、授权、JWT claim、refresh token 或安全上下文。
- 仍保持 JWT 不写角色、邮箱、用户名和用户状态。
- app/providers 不应成为权限规则的放置点：
  - 权限判断在 middleware/service 中完成。
  - provider 只连接认证 middleware、token codec、user service 等依赖。

## 11. 测试策略
- 单元测试：
  - 本次文档规则变更不新增单元测试。
- service / repository 测试：
  - 不适用，本次不改 Go 代码。
- migration / sqlc 验证：
  - 不适用，本次不改 SQL、migration 或 sqlc 配置。
- 接口验证：
  - 不适用，本次不改 HTTP API。
- OpenAPI validate：
  - 不适用，Go 版当前没有 OpenAPI 契约文件，本次也不新增。
- 异常场景验证：
  - 通过文档和 grep 检查确认规则落点存在。
- Java-Go parity 验证：
  - 更新 parity matrix 中 Go 内部依赖组织与装配边界记录。
- 需要运行的命令：
  - `git diff --check`
  - `go test ./...`
  - `go vet ./...`
  - `make test`
  - `golangci-lint run` 如工具可用

## 12. 风险与替代方案
- 当前方案风险：
  - 规则先于代码落地，短期会出现 `authsvc.Dependencies` 和 `Bootstrap()` 与新规则不完全一致的结构债。
  - providers 目录还未创建，后续重构时需要避免一次性制造空 package。
  - middleware 有纯函数型和结构体型两类形态，规则需要允许无状态 middleware 保持函数式构造。
- 备选方案：
  - 方案 A：只改 `AGENTS.md`，不写 docs/ai 记录。
  - 方案 B：立即同步重构 app/providers、`Bootstrap(ctx)` 和 auth service 构造函数。
  - 方案 C：继续允许 service 使用 `Dependencies` struct。
  - 方案 D：把 provider 聚合结构体放到各业务 service 包。
- 为什么不选备选方案：
  - 不选 A：依赖组织规则变更属于非微小修改，需要可追溯的设计和实现说明。
  - 不选 B：本次目标是沉淀规则，运行时代码重构会扩大变更范围，应单独设计和验证。
  - 不选 C：service 里的字段转移型 `Dependencies` 会掩盖职责过大问题，也容易把 composition root 的装配形状下沉到业务层。
  - 不选 D：会让 service/handler/domain/repository 承担装配职责，削弱 composition root 边界。
- 后续可演进点：
  - 新开一次实现任务，把 `Bootstrap()` 改为 `Bootstrap(ctx context.Context)`。
  - 按实际依赖增长拆出 `internal/app/providers`，不要创建空 Go package。
  - 将 `authsvc.NewService(authsvc.Dependencies{...})` 收敛为显式参数或更小职责的构造函数。
