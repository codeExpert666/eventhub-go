# Refresh Token 轮换、重放检测与 Logout No-op 实现说明

## 1. 本次改动解决了什么问题

本次补齐 Go 版 auth 双 token 生命周期中的 refresh/logout 闭环：

- 新增 `POST /api/v1/auth/refresh`，客户端可使用 opaque refresh token 轮换新的 access token 与 refresh token。
- refresh 成功后旧 refresh token 立即失效，`auth_sessions.version` 执行 `version + 1`。
- 同一旧 refresh token 并发提交时，依赖 DB 条件更新保证最多一个请求成功。
- 旧 refresh token 重放、过期 session、revoked session、禁用用户和非法格式统一返回 `AUTH-401`。
- 新增 `POST /api/v1/auth/logout`，当前只要求已认证并返回成功，不修改 DB。

本次保留 JWT claim 边界，不把角色、邮箱、用户名、用户状态写入 JWT；refresh token 仍是 opaque token，不是 JWT。

## 2. 改动内容
- 新增了什么
  - 设计文档：`docs/ai/design/013-refresh-token-rotation.md`。
  - service 输入输出：`RefreshCommand`、`LogoutCommand`、`RefreshResult`。
  - service 用例：`AuthService.Refresh`、`AuthService.Logout`。
  - HTTP DTO：`RefreshTokenRequest`、`TokenPairResponse`。
  - HTTP handler：`Handler.Refresh`、`Handler.Logout`。
  - 路由：
    - `POST /api/v1/auth/refresh` 匿名放行。
    - `POST /api/v1/auth/logout` 放入 auth middleware 保护组。
  - service 测试：
    - refresh 成功轮换并拒绝旧 token 重放。
    - 新 refresh token 可再次轮换。
    - expired、revoked、disabled user、非法格式均返回 `AUTH-401`。
    - 同一旧 token 并发 16 次最多 1 次成功。
    - logout authenticated no-op，缺失 principal 返回 `AUTH-401`。
  - HTTP 集成测试：
    - refresh 成功返回 token pair。
    - refresh 旧 token 重放返回 `AUTH-401`。
    - refreshToken 为空返回 `COMMON-400`。
    - logout 未认证返回 `AUTH-401`。
    - logout 已认证成功且不修改 session。
  - 新增 ADR：
    - `docs/ai/adr/0014-refresh-token-rotation-and-replay.md`
    - `docs/ai/adr/0015-logout-noop-vs-session-revoke.md`
- 修改了什么
  - `AuthSessionRepository` 将原 `RotateRefreshToken` 语义收敛为 `ConditionalRotate`。
  - sqlc query 名称改为 `ConditionalRotateAuthSessionRefreshToken`，SQL 条件保持 Java 对齐。
  - MySQL repository 和 integration test 同步使用 `ConditionalRotate`。
  - fake repository 实现原子条件更新语义：sessionId、old hash、old version、ACTIVE、未过期全部命中才替换 hash 并 `version + 1`。
  - router 的受保护组支持 auth logout 和 `/api/v1/me` 分别按依赖存在注册。
- 删除了什么
  - 未删除生产能力。
  - 旧 repository 方法名 `RotateRefreshToken` 被替换为更明确的 `ConditionalRotate`。
- 是否更新 Java-Go parity 记录
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
  - 本次触发 API、错误码、repository 行为、并发语义、auth/security 和测试策略 parity 更新。

## 3. 为什么这样设计
- 关键设计原因
  - refresh token 是长期凭证，服务端仍只保存 `sha256:<hex>`，避免数据库泄漏后直接复用明文 token。
  - opaque refresh token 不携带用户、session、角色或权限信息；refresh 身份只来自 DB hash 匹配到的 auth session。
  - 并发正确性依赖单条 DB 条件更新，不依赖 Go mutex；这样能覆盖多实例部署。
  - 条件更新包含旧 sessionId、旧 hash、旧 version、ACTIVE 和未过期时间，确保同一旧 token 最多轮换成功一次。
  - logout 当前 no-op 对齐 Java 版无状态 access token 语义，避免在未设计 denylist/token family 前引入半套吊销机制。
- 与 Go 项目当前阶段的匹配点
  - handler 只做 DTO decode、validate、Command 映射和响应写出。
  - service 承载 refresh 业务校验、事务边界、token 生成、条件更新成功判定和错误语义收敛。
  - repository 表达持久化业务语义，不暴露 sqlc 参数结构给 service。
  - sqlc/database 只执行参数化 SQL，不承载业务判断。
  - 未新增空 package、重量级依赖、Redis 锁、Go mutex 或领域外抽象。
- 与 Java 版业务语义的对齐方式
  - API 路径和字段对齐 Java `AuthController.refresh/logout`、`RefreshTokenRequest`、`TokenPairResponse`。
  - refresh 失败原因统一收敛为 `AUTH-401` 和 `refresh token 无效或已过期`。
  - 条件更新 SQL 对齐 Java `AuthSessionMapper.xml`。
  - logout 对齐 Java `AuthServiceImpl.logout`：要求已认证，服务端 no-op。

## 4. 替代方案
- 方案 A：refresh token 改为 JWT，携带 sessionId/tokenId。
  - 没有采用。Java 当前 refresh token 是 opaque random token；JWT 会让长期凭证携带可解析元数据。
- 方案 B：新增 refresh token 历史表或 token family 表，旧 token 重放时吊销整条 token family。
  - 没有采用。安全性更强，但需要新增 migration、清理策略、审计语义和 reuse 状态机；本次明确当前不自动吊销已轮换的新会话。
- 方案 C：使用 Go mutex 或 Redis 锁串行化同一 session refresh。
  - 没有采用。Go mutex 无法覆盖多实例，Redis 锁增加外部一致性边界；DB 条件更新仍是必须兜底。
- 方案 D：logout 立即 `REVOKED` 当前 session，并加入 access token denylist。
  - 没有采用。当前 Java 版 logout 是 no-op；session revoke 和 denylist 需要单独设计多端退出、access token 即时失效和缓存一致性。

## 5. 测试与验证
- 跑了哪些测试
  - `go test ./internal/service/auth`：通过。
  - `go test ./internal/http`：通过。
  - `go test -race ./internal/service/auth`：通过。
  - `go test ./...`：通过。
  - `make test`：通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w`：已对改动 Go 文件运行。
  - `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate`：通过。
  - `make sqlc`：通过。
  - `go test ./...`：通过。
  - `go test -race ./internal/service/auth`：通过。
  - `go vet ./...`：通过。
  - `make test`：通过。
  - `make vet`：通过。
  - `golangci-lint run`：未运行，当前环境没有安装 `golangci-lint`，`command -v golangci-lint` 无输出。
  - migration 测试：本次未修改 migration；`go test ./...` 已运行 repository/mysql 集成测试，按 Testcontainers 环境策略执行。
  - OpenAPI validate：未运行，Go 版当前没有 OpenAPI 契约文件，本次也未修改 `api/openapi`。
- 手工验证了哪些场景
  - `rg` 确认旧 repository 方法名 `RotateRefreshToken` 只剩历史设计文档引用，生产代码和测试均使用 `ConditionalRotate`。
  - 检查 `POST /api/v1/auth/refresh` 未挂 auth middleware，避免 access token 过期影响 refresh。
  - 检查 `POST /api/v1/auth/logout` 在 auth middleware 保护组内。
- Java-Go parity 如何验证
  - 对照 Java `AuthController.java`、`RefreshTokenRequest.java`、`TokenPairResponse.java`、`AuthServiceImpl.refresh/logout`、`RefreshTokenParser.java`、`RefreshTokenHasher.java`、`AuthSessionMapper.xml` 和 `V3__create_auth_sessions.sql`。
  - 更新 parity matrix 中 auth refresh/logout、auth session 持久化底座、refresh token 与 auth session 服务流程、认证错误与安全响应等行。
- 结果如何
  - 可运行验证均通过；lint 因本机工具缺失未运行。

## 6. 已知限制
- 当前版本还缺什么
  - 不保存 refresh token 历史 hash，因此旧 token 重放只能识别为无效凭证，不能定位并吊销已轮换的新会话。
  - logout 不吊销 auth session，也不让已签发 access token 即时失效。
  - 没有 access token denylist、token family、reuse audit、过期 session 清理任务。
- 哪些地方后面需要继续演进
  - 引入 token family 或 refresh token reuse audit 表后，可在重放时吊销当前会话链。
  - 实现用户禁用时批量吊销 ACTIVE sessions。
  - 实现真正 logout session revoke 前，需要设计 access token denylist 或 `sid` session status 校验。
  - 增加 OpenAPI 契约文件和 validate 命令后，应同步 refresh/logout schema。
- 与 Java 版仍有哪些差距
  - Go 当前未迁移 Java 的管理员用户 API、完整 OpenAPI 注解输出和更多 auth integration 测试。
  - Java 文档已讨论后续 token family/reuse revoke，但当前 Java 实现同样不自动吊销已轮换的新会话；Go 本次保持对齐。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 补齐双 token 登录续期闭环，能展示 refresh token 轮换、重放检测、乐观条件更新和安全错误收敛。
- 对微服务 / 云原生演进的影响
  - 并发正确性落在 DB 条件更新上，比进程内锁更适合未来多实例部署。
  - 后续若拆分认证服务，`ConditionalRotate` 可作为会话服务的核心持久化语义。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - repository 语义命名从 `RotateRefreshToken` 收敛为 `ConditionalRotate`，后续 session revoke、device session 和 token family 能继续按业务动作命名。
  - 如果新增 token family/reuse audit，需要 migration、sqlc query、repository interface、service 测试和 parity matrix 同步扩展。
  - OpenAPI 文件落地后，需要补充 refresh/logout request/response schema 和安全要求。
