# JWT Codec 构造与 TTL 边界收敛设计

## 1. 背景
- 当前 `internal/security/jwt.Codec` 通过 `jwt.Config` 构造，`Config` 与 `Codec` 字段高度重合，并混合了 issuer、signing secret、access token TTL 与 `clock.Clock`。
- 先前 `docs/ai/design/009-dependency-organization-simplification.md`、`010-app-provider-constructor-rules.md` 和 `011-app-provider-runtime-structure.md` 已经收敛了 handler、service、middleware 和 app provider 的依赖组织规则：业务组件优先使用显式构造参数，避免仅用于字段转移的结构体。
- Java 版对应来源：
  - `infra/security/config/AuthTokenProperties.java`
  - `infra/security/jwt/JwtCodec.java`
  - `modules/auth/service/impl/TokenServiceImpl.java`
  - Java ADR `docs/ai/adr/2026-05-24-auth-token-configuration-boundary.md`
- Java 版已经明确：`JwtCodec` 只负责 JWT access token 的生成、解析和验签，不对外提供 TTL 查询门面；登录响应中的 access token TTL 由 token 业务服务从认证令牌配置读取。

## 2. 目标
- 移除 `internal/security/jwt.Config`。
- 将 `jwt.NewCodec` 改为显式参数构造，避免字段搬运型配置结构体。
- 移除 `Codec.AccessTokenTTL()`。
- 将 access token TTL 作为 auth service 的业务参数保存，由 `Login` 同时用于签发 access token 和填充登录响应 `expiresIn`。
- 保持 JWT claim、登录 API 响应字段、错误码、数据库模型和认证 middleware 行为不变。
- 成功标准：
  - Go 代码中不再存在 `jwt.Config` 和 `AccessTokenTTL()`。
  - `internal/security/jwt` 不依赖 `internal/config`，仍保持 JWT 技术组件边界。
  - `go test ./...` 和 `go vet ./...` 通过。

## 3. 非目标
- 不修改 HTTP API 路径、请求字段、响应字段、状态码或错误码。
- 不修改 JWT claim 集合；仍只包含 `iss/sub/iat/exp/jti/sid/typ=access`。
- 不实现 refresh/logout、token rotation、denylist 或 RBAC 管理端能力。
- 不引入 DI 容器、配置绑定库或新的第三方依赖。
- 不把 `internal/config.Config` 直接传入 `internal/security/jwt` 或 `internal/service/auth`。
- 不逐行迁移 Java 的 Spring `AuthTokenProperties` 注入方式。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `internal/security/jwt`
  - `internal/service/auth`
  - `internal/app/providers`
  - `internal/http` 相关测试
- 不涉及 API / 表 / 缓存 / 外部接口。
- 影响 `docs/ai/parity/java-go-parity-matrix.md`：是。本次属于 Go 内部构造 API 与 JWT 配置边界收敛，不改变 Java 对外契约，但需要记录 Go 与 Java 在配置注入方式上的刻意差异。

## 5. 领域建模
- 本次不新增业务领域实体。
- 工程边界对象：
  - `jwt.Codec`：JWT 技术组件，只保存 issuer、签名密钥和时间来源，负责签发、解析、验签、issuer 校验、过期校验和 claim 提取。
  - `auth.Service`：认证业务用例，持有 access token TTL，用于登录签发 token pair 和返回 `expiresIn`。
  - `config.AuthTokenConfig`：启动配置来源，继续由 app provider 读取并拆成显式构造参数。
- 与 Java 版对应关系：
  - Java `JwtCodec` 通过 `AuthTokenProperties` 读取签发默认 TTL，但不暴露 TTL 查询方法。
  - Go 不把全局配置对象注入 JWT 包；由 provider 把配置值显式传入 `jwt.Codec` 和 `auth.Service`，保持 package 边界直接。

## 6. API 设计
- 对外 HTTP API 不变。
- 内部构造 API 调整：
  - `jwt.NewCodec(issuer string, signingSecret string, tokenClock clock.Clock) (*Codec, error)`
  - `(*jwt.Codec).IssueAccessToken(subjectID int64, sessionID string, ttl time.Duration) (string, error)`
  - 删除 `(*jwt.Codec).IssueAccessTokenWithTTL`
  - 删除 `(*jwt.Codec).AccessTokenTTL`
  - `auth.NewService(..., tokens *jwt.Codec, accessTokenTTL time.Duration, refreshToken *refresh.Manager, ...) (*Service, error)`
- 错误码 / 异常场景：
  - HTTP 业务错误码不变。
  - `jwt.NewCodec` 仍在启动期校验 issuer 和签名密钥。
  - `auth.NewService` 在启动期校验 access token TTL 必须为正数；该错误属于 provider 装配错误，不进入 HTTP 统一响应 envelope。
- 与 Java 版 OpenAPI / controller 契约的差异：无。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 登录成功仍在同一事务中校验用户、创建 ACTIVE auth session、保存 refresh token hash 并签发 access token。
  - TTL 来源从 `jwt.Codec` getter 改为 `auth.Service` 字段，不改变 auth session 的 `refresh_expires_at` 或 access token `exp` 计算语义。

## 8. 关键流程
- 正常流程：
  1. `ProviderAuth` 从 `platform.Config.AuthToken` 读取 issuer、access signing secret、access token TTL 和 refresh token TTL。
  2. `ProviderAuth` 用 issuer、secret 和 clock 创建 `jwt.Codec`。
  3. `ProviderAuth` 将 access token TTL 显式传给 `auth.NewService`。
  4. `auth.Service.Login` 创建服务端 auth session。
  5. `auth.Service.Login` 调用 `jwt.Codec.IssueAccessToken(userID, sessionID, accessTokenTTL)`。
  6. `auth.Service.Login` 用同一份 `accessTokenTTL` 填充 `LoginResult.ExpiresIn`。
- 异常流程：
  - issuer 为空或签名密钥不足 32 字节时，`jwt.NewCodec` 返回错误，`Bootstrap` 包装为 auth provider 启动失败。
  - access token TTL 非正数时，`auth.NewService` 返回错误，`Bootstrap` 同样以启动装配错误处理。
  - token 格式错误、签名篡改、issuer 不匹配、过期、缺失必要 claim 时，认证 middleware 仍映射为 `AUTH-401`。
- 状态流转：
  - 不改变用户状态、auth session 状态或 token 生命周期状态机。
- handler / service / repository / sqlc/database 分工：
  - handler：仍只做 HTTP DTO 映射与响应写出。
  - service：持有登录业务需要的 access token TTL 和 refresh token TTL 相关能力。
  - repository/sqlc：不受影响。
  - provider：只拆解配置并连接对象，不承载业务规则。

## 9. 并发 / 幂等 / 缓存
- 不涉及库存、订单、支付、幂等键或缓存。
- 不新增并发控制。
- 事务边界不变：登录流程仍由 auth service 通过 `platform/db.TxRunner` 控制事务。
- access token TTL 是启动期不可变配置，保存在 service 字段中，不引入共享可变状态。

## 10. 权限与安全
- 认证和授权边界不变。
- JWT claim 边界不变：
  - 继续写入 `iss/sub/iat/exp/jti/sid/typ=access`。
  - 不写入角色、邮箱、用户名或用户状态。
- signing secret 仍只在 `jwt.Codec` 内部转为 `[]byte` 保存，不写日志、不进入响应。
- 移除 `jwt.Config` 后不会把 secret 放入可打印的聚合结构体，降低未来误日志输出的机会。
- access token TTL 校验前移到 auth service 构造，避免运行时用无效 TTL 签发登录 token。

## 11. 测试策略
- 单元测试：
  - 更新 `internal/security/jwt` 测试，使用显式 `NewCodec` 构造并调用显式 TTL 签发方法。
  - 增加或调整 auth service fixture，验证 `expiresIn` 仍来自配置 TTL。
- service / repository 测试：
  - auth service 既有注册、登录和失败场景测试继续覆盖。
- migration / sqlc 验证：
  - 不适用，本次不改 SQL、migration 或 sqlc 配置。
- 接口验证：
  - HTTP auth 集成测试继续覆盖登录响应 token pair 和 `/me`。
- OpenAPI validate：
  - 不适用，本次不改 OpenAPI，Go 版当前没有 OpenAPI 契约文件。
- 异常场景验证：
  - `jwt.NewCodec` issuer/secret 校验保持。
  - `auth.NewService` access token TTL 非正数校验通过测试或代码检查覆盖。
- Java-Go parity 验证：
  - 对照 Java `AuthTokenProperties`、`JwtCodec`、`TokenServiceImpl` 和 auth token 配置边界 ADR。
  - 更新 parity matrix 的“Go 内部依赖组织与装配边界”和“JWT、认证上下文与 RBAC 安全边界”说明。
- 需要运行的命令：
  - `gofmt`
  - `go test ./...`
  - `go vet ./...`
  - `make test`
  - `make vet`
  - `golangci-lint run` 如工具可用

## 12. 风险与替代方案
- 当前方案风险：
  - `auth.NewService` 改为返回 `(*Service, error)`，需要同步所有测试和 provider 构造点。
  - access token TTL 同时影响签发和响应，一旦未来支持按客户端或风险等级动态 TTL，需要把当前固定字段演进为策略对象或用例参数。
- 备选方案：
  - 方案 A：只删除 `jwt.Config`，保留 `Codec.AccessTokenTTL()`。
  - 方案 B：让 `jwt.Codec` 继续保存默认 TTL，但删除 getter，auth service 也保存一份 TTL。
  - 方案 C：让 `jwt.Codec` 直接依赖 `internal/config.AuthTokenConfig`。
  - 方案 D：新增独立 `TokenService`，专门持有 JWT codec、access TTL 和 refresh manager。
- 为什么不选备选方案：
  - 不选 A：`AccessTokenTTL()` 仍会让 JWT 技术组件承担配置查询门面职责。
  - 不选 B：同一 TTL 在 codec 和 service 中重复保存，容易产生配置不一致风险。
  - 不选 C：会让安全技术组件依赖全局应用配置包，削弱 package 边界。
  - 不选 D：方向合理，但当前 Go auth service 还只有 register/login/me 阶段；单独拆 TokenService 会扩大本次重构范围，适合 refresh/logout 迁移时再评估。
- 后续可演进点：
  - refresh/logout 落地时评估是否拆分 token/session use case。
  - 如果支持多端、多租户或风险分级 TTL，可将固定 `accessTokenTTL` 演进为 auth service 内部策略。
