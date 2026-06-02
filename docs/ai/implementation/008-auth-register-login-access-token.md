# Auth Register Login Access Token 实现说明

## 1. 本次改动解决了什么问题

本次为 Go 版 EventHub 实现了认证最小闭环：

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/me`
- JWT access token 签发与解析
- Bearer 认证 middleware
- Go context 中传递 `security.Principal`
- 登录成功创建 `ACTIVE` auth session
- refresh token 明文只返回一次，DB 只保存 `sha256:<hex>`

实现前已先读取 Java 版 auth DTO、Controller、Service、JWT/security、测试和 OpenAPI 配置，新增 `docs/ai/parity/java-auth-api-contract.md` 作为本阶段 auth 契约索引。

## 2. 改动内容
- 新增了什么
  - `internal/security/principal.go`：定义 `Principal` 与 context 读写。
  - `internal/security/password`：BCrypt hash / matches。
  - `internal/security/jwt`：HS256 access token 签发、解析、issuer/exp/signature/claim 校验。
  - `internal/security/refresh`：opaque refresh token 生成、格式校验和 `sha256:<hex>` hash。
  - `internal/service/auth`：注册、登录、Command/Result、错误辅助和事务边界。
  - `internal/service/user`：当前用户摘要查询和认证主体加载。
  - `internal/http/dto/auth`、`internal/http/dto/user`：认证请求和用户响应 DTO。
  - `internal/http/handler/auth`、`internal/http/handler/user`：注册、登录、`/me` handler。
  - `internal/http/middleware/auth.go`：Bearer access token 认证 middleware。
  - `internal/http/auth_integration_test.go`：覆盖注册、重复注册、登录、错误密码、`/me`、禁用用户旧 token。
  - `internal/service/auth/service_test.go`、`internal/http/middleware/auth_test.go`、`internal/security/*_test.go`：覆盖 service、安全和 middleware 边界。
  - `docs/ai/design/008-auth-register-login-access-token.md`。
  - `docs/ai/adr/0011-jwt-claim-boundary.md`。
  - `docs/ai/adr/0012-refresh-token-hash-format.md`。
  - `docs/ai/adr/0013-current-user-loading-strategy.md`。
- 修改了什么
  - `internal/config` 增加 MySQL 和 auth token 配置。
  - `internal/app` 在配置 MySQL DSN 时装配真实 auth repository/service/middleware。
  - `internal/http/router.go` 和 `internal/http/server.go` 支持通过 `RouterOption` 注入 auth 路由依赖。
  - `internal/http/validation/error.go` 增加 error 到 `AppError` 的统一转换辅助。
  - `configs/*.env.example` 增加 MySQL、access token 和 refresh token 配置示例。
  - `go.mod` 将本次直接使用的 `github.com/google/uuid` 和 `golang.org/x/crypto` 标记为直接依赖。
- 删除了什么
  - 无。
- 是否更新 Java-Go parity 记录
  - 已新增 `docs/ai/parity/java-auth-api-contract.md`。
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
- file moves / package boundary changes
  - 无文件移动。
  - 新增 `handler/auth`、`handler/user`、`dto/auth`、`dto/user` 子包，未把业务 handler/DTO 放在 root 包。
  - 新增 `service/auth` 与 `service/user`，按 `service.go`、`command.go`、`query.go`、`result.go` 和 use case 文件拆分。

## 3. 为什么这样设计
- 关键设计原因
  - 对齐 Java 业务语义，而不是迁移 Spring Security 结构。
  - JWT 只保存 `iss/sub/iat/exp/jti/sid/typ=access`，动态用户状态和角色每次回 DB 加载，确保禁用用户旧 token 被拒绝。
  - 注册和登录的多表写入由 service 通过 `Transactor` 控制，handler 不接触数据库。
  - refresh token 使用 opaque token；明文只返回一次，DB 保存 `sha256:<hex>`。
  - `Principal` 通过 Go context 传递，handler 不解析 JWT。
- 与 Go 项目当前阶段的匹配点
  - 复用已有 `users/roles/user_roles/auth_sessions` migration、repository 和 sqlc 查询，不改 SQL。
  - router 使用 option 注入 auth 依赖，既能保持现有 system 测试轻量，也能让 app 在有 DSN 时接入真实数据库。
  - DTO 和 service contract 继续遵守模块化包边界。
- 与 Java 版业务语义的对齐方式
  - API 路径、请求字段、响应字段、错误码和核心错误消息与 Java 版对齐。
  - 注册默认绑定 `USER`，重复账号映射 `AUTH-409`。
  - 登录成功返回 `accessToken/refreshToken/authorizationScheme/expiresIn/refreshExpiresIn/sessionId/user`。
  - 受保护请求解析 token 后回库加载用户和角色，禁用用户旧 token 返回 `AUTH-401`。

## 4. 替代方案
- 方案 A：JWT 写入角色、邮箱、用户名和状态。
  - 没有采用。这样会把动态权限变成 token 快照，禁用用户和角色变更无法及时生效，也违反 Java ADR 和项目约束。
- 方案 B：只实现无 refresh token 的单 access token 登录。
  - 没有采用。Java 当前登录响应已经是 token pair，并要求登录成功创建 `ACTIVE` auth session。
- 方案 C：handler 直接使用 repository 或 database。
  - 没有采用。会破坏 `handler -> service -> repository -> sqlc/database` 分层。
- 方案 D：把 auth handler、user handler、DTO 全部放在 root 包。
  - 没有采用。当前仓库已经决策业务 handler/DTO 按模块子包组织。
- 方案 E：本次同步实现 refresh/logout/admin users。
  - 没有采用。任务明确 refresh endpoint 下一阶段实现，本次保留最小闭环并聚焦 access token 安全边界。

## 5. 测试与验证
- 跑了哪些测试
  - `go test ./...`
  - `go test -race ./internal/service/auth ./internal/http/middleware`
  - `go vet ./...`
  - `make test`
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - 已执行 `gofmt`。
  - 已执行 `go mod tidy`。
  - 已执行 `go test ./...`，通过。
  - 已执行 `go test -race ./internal/service/auth ./internal/http/middleware`，通过。
  - 已执行 `go vet ./...`，通过。
  - 已执行 `make test`，通过。
  - 未执行 `sqlc generate`：本次没有修改 SQL、migration 或 `sqlc.yaml`。
  - 未执行 OpenAPI validate：Go 版当前仍没有 OpenAPI 契约文件。
  - 未执行 `golangci-lint run`：当前机器未安装 `golangci-lint`。
- 手工验证了哪些场景
  - 通过测试验证注册、重复注册、登录、错误密码、`/me`、禁用用户旧 token。
  - 验证 JWT `typ`、`jti`、`sid` 缺失或错误会被拒绝。
  - 验证 refresh token hash 前缀和长度为 `sha256:` + 64 hex。
  - 验证登录失败不创建 session。
- Java-Go parity 如何验证
  - 对照 `docs/ai/parity/java-auth-api-contract.md`。
  - 对照 Java `AuthIntegrationTest` 和 `JwtCodecTest` 的关键断言。
- 结果如何
  - 可行验证均通过；lint 因工具缺失未运行。

## 6. 已知限制
- 当前版本还缺什么
  - 尚未实现 `POST /api/v1/auth/refresh`。
  - 尚未实现 `POST /api/v1/auth/logout`。
  - 尚未实现管理员用户列表、管理员更新用户状态和 RBAC 管理端接口。
  - 尚未生成 Go OpenAPI YAML。
- 哪些地方后面需要继续演进
  - refresh token 轮换需要使用 old hash、version、status 和过期时间条件更新。
  - logout 可以结合 `sid`、`jti`、Redis denylist 或 session status 校验做即时吊销。
  - 管理员禁用用户时可批量吊销该用户 ACTIVE sessions。
  - 如果引入用户状态/角色缓存，必须先设计失效策略。
- 与 Java 版仍有哪些差距
  - Java 已有 refresh、logout、admin users、状态更新和生产 OpenAPI 安全测试；Go 本次只迁移 register/login/me/access token。
  - Java 的 OpenAPI 由 Springdoc 自动生成；Go 尚未建立契约文件。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - Go 版已有真实 auth 登录闭环、JWT claim 边界、refresh token 安全存储和禁用用户旧 token 拒绝能力。
- 对微服务 / 云原生演进的影响
  - `Principal`、JWT claim 边界和 auth session `sid` 为后续网关、资源服务、denylist、审计和设备会话管理提供稳定基础。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续 refresh/logout/admin 继续复用 `service/auth`、`service/user`、`security` 和现有 repository。
  - 如果新增 SQL 或 migration，必须重新运行 `sqlc generate` 和 migration 测试。
  - OpenAPI 文件落地后需要补 validate 命令和契约测试。
