# Logout 认证主体错误边界实现说明

## 1. 本次改动解决了什么问题

本次改动解决了 `LogoutStrict` 与 `GetCurrentUserStrict` 在读取认证主体失败时错误分类不一致的问题。

改动前，`LogoutStrict` 会把 `security.RequiredPrincipal(ctx)` 返回的任何错误都映射为 `AUTH-401`。当前 `RequiredPrincipal` 只会返回 `security.ErrMissingPrincipal`，所以运行时暂时没有明显外部行为差异；但如果后续该函数扩展出其它错误类型，logout 会把内部异常误报为“请先登录或重新登录”。

改动后，`LogoutStrict` 只把 `security.ErrMissingPrincipal` 映射为 `AUTH-401`，其它错误交给 `apperror.FromErrorOrInternal` 保留内部错误语义，与 `GetCurrentUserStrict` 保持一致。

## 2. 改动内容
- 新增了什么
  - `internal/http/handler/auth/strict_test.go`
    - 新增 `TestLogoutStrictClassifiesRequiredPrincipalErrors`，用 AST 策略测试固化 `LogoutStrict` 必须在 `RequiredPrincipal` 错误分支中显式区分 `ErrMissingPrincipal` 与其它错误。
  - `docs/ai/design/031-logout-principal-error-boundary.md`。
  - `docs/ai/implementation/031-logout-principal-error-boundary.md`。
- 修改了什么
  - `internal/http/handler/auth/strict.go`
    - 新增标准库 `errors` import。
    - `LogoutStrict` 在 `RequiredPrincipal` 返回错误时先用 `errors.Is(err, security.ErrMissingPrincipal)` 判断缺失主体，再用 `apperror.FromErrorOrInternal(err)` 处理其它错误。
  - `docs/ai/parity/java-go-parity-matrix.md`
    - 更新最近核验日期。
    - 在认证错误与安全响应行记录 logout strict handler 的 Go-only 边界一致性收敛。
- 删除了什么
  - 无。
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次不改变对外契约，但调整了 Go 端认证错误边界，属于 auth/security parity 索引需要记录的 Go-only 结构收敛。

## 3. 为什么这样设计
- 关键设计原因
  - 缺失认证主体是明确的认证失败，应继续映射为 `AUTH-401`。
  - 非缺失主体错误不应被伪装成认证失败，否则会降低未来排障和监控的准确性。
  - `GetCurrentUserStrict` 已经采用该分类方式，logout 与其保持一致能减少 strict handler 风格分裂。
- 与 Go 项目当前阶段的匹配点
  - 不新增抽象，不引入依赖，不改 service contract。
  - handler 继续只承担 HTTP 入参、认证上下文读取、service 调用和错误映射。
  - service、domain、repository、sqlc/database 不依赖 HTTP 或 generated model 的边界不变。
- 与 Java 版业务语义的对齐方式
  - Java 受保护接口缺失认证时返回未登录语义。
  - Go 版继续对外保持缺失主体的 `AUTH-401` 语义，同时用显式错误返回保留内部异常边界。

## 4. 替代方案
- 方案 A：保持现状。
  - 没有采用。当前虽然运行时可接受，但同类 strict handler 风格不一致，且未来扩展错误类型时容易误分类。
- 方案 B：抽一个共享 helper，例如 `requiredPrincipalOrUnauthorized`。
  - 没有采用。当前只有两处读取主体，直接在 handler 内显式分类更清楚，避免过早抽象。
- 方案 C：修改 `security.RequiredPrincipal` 让测试能直接构造其它错误。
  - 没有采用。该函数当前语义很小，扩展它只是为了测试会扩大安全基础设施边界。
- 方案 D：只补注释不改代码。
  - 没有采用。注释无法约束后续代码行为，也无法让 logout 与 `/me` 的错误分类真正一致。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./internal/http/handler/auth`
    - 失败原因符合预期：`LogoutStrict should classify security.ErrMissingPrincipal explicitly`。
  - GREEN：`go test ./internal/http/handler/auth`
    - 通过。
  - `go test ./...`
    - 通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w internal/http/handler/auth/strict.go internal/http/handler/auth/strict_test.go`：已执行。
  - `gofmt -l internal/http/handler/auth/strict.go internal/http/handler/auth/strict_test.go`：无输出。
  - `go vet ./...`：通过。
  - `golangci-lint run`：通过，输出 `0 issues.`。
  - `git diff --check -- docs/ai/design/031-logout-principal-error-boundary.md internal/http/handler/auth/strict.go internal/http/handler/auth/strict_test.go`：通过。
  - `sqlc generate`：未运行，本次不涉及 SQL、schema 或 sqlc 配置。
  - OpenAPI validate / generate：未运行，本次不涉及 OpenAPI 契约或 generated code。
- 手工验证了哪些场景
  - 对比 `LogoutStrict` 与 `GetCurrentUserStrict` 的主体读取错误映射，确认两者都显式区分 `ErrMissingPrincipal` 和其它错误。
- Java-Go parity 如何验证
  - 更新 `docs/ai/parity/java-go-parity-matrix.md` 的认证错误与安全响应行。
  - 确认 `POST /api/v1/auth/logout` 的路径、请求、成功响应、401 错误语义和 OpenAPI 契约不变。
- 结果如何
  - 相关包测试、全量 Go 测试、vet、lint、格式化和 whitespace 检查均通过。

## 6. 已知限制
- `security.RequiredPrincipal` 当前只返回 `security.ErrMissingPrincipal`，所以非缺失主体错误分支运行时不可直接构造。
- 因此本次使用 AST 策略测试固定 handler 边界；如果未来 `RequiredPrincipal` 增加可达错误类型，应补充黑盒行为测试。
- 当前 logout 仍是无状态 access token 语义，不处理 token blacklist 或 session revoke。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 认证错误边界更清晰，strict handler 风格更统一。
- 对微服务 / 云原生演进的影响
  - 后续认证主体加载如果接入缓存、远程权限服务或更多上下文校验，内部错误不会被误报为未登录。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 新增需要读取认证主体的 strict handler 时，优先采用同样的 `ErrMissingPrincipal` 显式分类方式。
  - 不影响 migration、sqlc 或 OpenAPI 生成策略。
