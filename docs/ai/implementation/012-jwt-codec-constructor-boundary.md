# JWT Codec 构造与 TTL 边界收敛实现说明

## 1. 本次改动解决了什么问题

本次解决 `internal/security/jwt` 中两个边界问题：

- `jwt.Config` 与 `jwt.Codec` 字段高度重合，并把 issuer、signing secret、access token TTL 和 `clock.Clock` 混在一个字段搬运结构体中。
- `Codec.AccessTokenTTL()` 让 JWT 技术组件承担了配置查询门面职责；登录响应 `expiresIn` 实际属于 auth 登录用例语义。

本次不改变 HTTP API、错误码、JWT claim、refresh token hash、数据库模型、auth session 创建或认证 middleware 行为。

## 2. 改动内容
- 新增了什么
  - 新增设计文档 `docs/ai/design/012-jwt-codec-constructor-boundary.md`。
  - 新增 auth service 构造期 access token TTL 校验测试：`TestNewServiceRejectsNonPositiveAccessTokenTTL`。
- 修改了什么
  - `jwt.NewCodec` 改为显式参数：`NewCodec(issuer, signingSecret, tokenClock)`。
  - `jwt.Codec.IssueAccessToken` 改为由调用方显式传入 TTL。
  - `auth.Service` 新增 `accessTTL` 字段，用于登录签发 access token 和填充 `LoginResult.ExpiresIn`。
  - `auth.NewService` 新增 `accessTokenTTL` 参数，并改为返回 `(*Service, error)`，在启动装配期拒绝非正数 TTL。
  - `ProviderAuth` 从 `platform.Config.AuthToken` 拆出 issuer、signing secret、access token TTL 和 refresh token TTL，并显式传给 JWT codec 与 auth service。
  - 更新 JWT 单元测试、auth service 测试、auth HTTP 集成测试和 auth middleware 测试的构造方式。
- 删除了什么
  - 删除 `internal/security/jwt.Config`。
  - 删除 `Codec.AccessTokenTTL()`。
  - 删除 `Codec.IssueAccessTokenWithTTL()`，统一使用显式 TTL 的 `IssueAccessToken`。
- 是否更新 Java-Go parity 记录
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
  - 本次属于 Go 内部构造 API 与 JWT 配置边界收敛，不改变 Java 对外契约。

## 3. 为什么这样设计
- 关键设计原因
  - 显式构造参数让 `jwt.Codec` 的真实启动参数直接可见，避免 `Config` 变成只做字段转移的结构体。
  - `jwt.Codec` 只保存 issuer、签名密钥和时间来源，保持 JWT 签发、解析和验签技术边界。
  - access token TTL 由 auth service 持有，可以保证签发 token 时使用的 TTL 和登录响应 `expiresIn` 来自同一份业务参数。
  - `auth.NewService` 返回 error 保留启动期 fail fast：无效 TTL 不会拖到登录请求时才暴露。
- 与 Go 项目当前阶段的匹配点
  - 保持 `handler -> service -> repository -> sqlc/database`。
  - `internal/app/providers` 继续负责拆解配置并连接对象。
  - `internal/security/jwt` 不依赖 `internal/config`，避免全局配置对象下沉到技术组件。
  - 没有引入新的 interface、Dependencies/Deps/Options 或第三方依赖。
- 与 Java 版业务语义的对齐方式
  - Java `JwtCodec` 只负责 JWT 技术能力，不对外提供 TTL 查询门面。
  - Java `TokenServiceImpl` 从 `AuthTokenProperties` 获取 access token TTL 和 refresh token TTL 来支撑登录响应。
  - Go 版没有迁移 Spring 配置注入模型，而是由 provider 显式传入配置值；业务语义、JWT claim 和登录响应字段保持一致。

## 4. 替代方案
- 方案 A：只删除 `jwt.Config`，保留 `Codec.AccessTokenTTL()`。
  - 没有采用。这样仍会让 JWT 技术组件承担配置查询门面职责。
- 方案 B：让 `jwt.Codec` 继续保存默认 TTL，但删除 getter，auth service 再保存一份 TTL。
  - 没有采用。同一 TTL 在两个对象中重复保存，增加后续配置不一致风险。
- 方案 C：让 `jwt.Codec` 直接依赖 `internal/config.AuthTokenConfig`。
  - 没有采用。这样会让底层安全技术包依赖全局应用配置包，削弱 package 边界。
- 方案 D：本次直接拆出独立 TokenService。
  - 没有采用。方向可以在 refresh/logout 迁移时评估，但当前只为删除字段搬运结构和 TTL getter，拆新 service 会扩大范围。

## 5. 测试与验证
- 跑了哪些测试
  - `go test ./...`：通过。
  - `make test`：通过，当前 target 执行 `go test ./...`。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w internal/security/jwt internal/service/auth internal/app/providers/auth.go internal/http/auth_integration_test.go internal/http/middleware/auth_test.go`：已运行。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `make test`：通过。
  - `make vet`：通过。
  - `git diff --check`：通过。
  - `golangci-lint run`：未运行，当前环境未安装 `golangci-lint`。
  - `sqlc generate`：未运行，本次未修改 SQL、migration 或 `sqlc.yaml`。
  - migration 测试：未单独运行，本次未修改 migration；`go test ./...` 中 repository/mysql 集成测试按环境策略执行。
  - OpenAPI validate：未运行，本次未修改 API 契约，Go 版当前没有 OpenAPI 文件。
- 手工验证了哪些场景
  - `rg` 确认生产 Go 代码中不再存在 `jwt.Config`、`AccessTokenTTL()` 或 `IssueAccessTokenWithTTL()`。
  - 检查 `ProviderAuth` 只拆解配置并连接对象，不承载业务规则。
  - 检查 `Login` 使用同一 `s.accessTTL` 签发 token 并填充 `ExpiresIn`。
- Java-Go parity 如何验证
  - 对照 Java `AuthTokenProperties.java`、`JwtCodec.java`、`TokenServiceImpl.java` 和 Java auth token 配置边界 ADR。
  - 已更新 parity matrix 的 Go 内部依赖组织记录和 JWT 安全边界记录。
- 结果如何
  - 可运行验证均通过；lint 因本机工具缺失未运行。

## 6. 已知限制
- 当前版本还缺什么
  - refresh/logout、access token denylist、refresh token rotation 和管理员 RBAC 仍待迁移。
- 哪些地方后面需要继续演进
  - refresh/logout 落地时可评估拆出 token/session use case，降低 auth service 构造参数数量。
  - 如果后续支持多客户端、风险分级或动态 TTL，可以把固定 `accessTTL` 演进为 auth service 内部策略。
- 与 Java 版仍有哪些差距
  - Java 通过 Spring `AuthTokenProperties` 注入共享配置对象；Go 刻意不迁移 Spring 注入模型，而是由 provider 显式传参。
  - Java 已有更多 auth 管理端和 token 生命周期能力；Go 当前仍处于 register/login/me/access token 阶段。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - JWT 技术边界和 auth 登录业务边界更清楚，能解释为什么 token TTL 响应不应通过 codec getter 暴露。
- 对微服务 / 云原生演进的影响
  - `jwt.Codec` 更接近可复用的资源服务 token 校验组件，不携带登录响应配置查询职责。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续构造安全组件时继续优先使用显式参数，避免字段搬运型 config/deps 结构。
  - SQL、migration、sqlc、OpenAPI 不受本次影响。
