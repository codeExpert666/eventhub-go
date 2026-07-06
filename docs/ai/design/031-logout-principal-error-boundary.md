# Logout 认证主体错误边界

## 1. 背景
- 当前 `LogoutStrict` 和 `GetCurrentUserStrict` 都通过 `security.RequiredPrincipal(ctx)` 获取认证主体。
- `GetCurrentUserStrict` 只把 `security.ErrMissingPrincipal` 映射为 `AUTH-401`，其它异常交给 `apperror.FromErrorOrInternal` 保留内部错误语义。
- `LogoutStrict` 当前把 `RequiredPrincipal` 返回的任何错误都映射为 `AUTH-401`，与同类 strict handler 的错误分类不一致。
- Java 版对应语义是受保护接口缺失认证时返回未登录；Go 版无需逐行迁移 Java 结构，但应保留认证失败和内部异常的边界。

## 2. 目标
- 让 `LogoutStrict` 的认证主体错误判断与 `GetCurrentUserStrict` 保持一致。
- 缺失认证主体继续返回 `AUTH-401` 和“请先登录或重新登录”。
- 非缺失主体错误交给 `apperror.FromErrorOrInternal`，避免未来扩展 `RequiredPrincipal` 时把内部错误误报为认证失败。
- 成功标准是代码路径、测试和文档都能说明该错误边界。

## 3. 非目标
- 不调整 OpenAPI 路径、请求字段、响应字段或状态码声明。
- 不调整 JWT claim、认证 middleware、RBAC、service、repository、sqlc、migration 或数据库模型。
- 不改变 logout 当前“无状态 access token，服务端不修改 DB”的业务行为。
- 不逐行迁移 Java filter / controller 实现细节。

## 4. 影响范围
- 涉及 Go package：
  - `internal/http/handler/auth`
  - `internal/security`
  - `internal/apperror`
- 涉及 API：
  - `POST /api/v1/auth/logout`
- 不涉及表、缓存、外部接口、OpenAPI generated model 或 service contract。
- Java-Go parity matrix 需要记录本次 Go handler 边界收敛；对外契约不变。

## 5. 领域建模
- 核心对象仍是 `security.Principal`，表示认证 middleware 写入 context 的最小认证主体。
- `security.ErrMissingPrincipal` 表示 context 中没有有效主体，属于认证缺失。
- 其它错误不属于当前运行时可达路径，但在边界设计上应保留为非认证缺失错误。
- 与 Java 版领域对象的对应关系不变化；Go 版继续用显式错误返回表达认证主体读取失败。

## 6. API 设计
- 接口：`POST /api/v1/auth/logout`
- 请求参数：不变。
- 成功响应：不变，仍返回 `ApiResponseVoid`。
- 错误码 / 异常场景：
  - 缺失认证主体：`AUTH-401`，消息“请先登录或重新登录”。
  - 非缺失主体错误：通过 `apperror.FromErrorOrInternal` 映射，通常为 `COMMON-500`。
  - service 层 logout 业务错误：继续通过 `apperror.FromErrorOrInternal` 映射。
- 与 Java 版契约差异：无新的对外契约差异；这是 Go strict handler 内部错误边界一致性调整。

## 7. 数据设计
- 不调整表结构、索引、唯一约束或 migration。
- 不新增 sqlc query，也不影响 generated model。
- 不改变数据一致性边界。

## 8. 关键流程
- 正常流程：
  1. strict handler 调用 `security.RequiredPrincipal(ctx)`。
  2. 读取成功后把 `security.Principal` 映射为 `authsvc.LogoutCommand`。
  3. service 执行当前无状态 logout 语义。
  4. handler 返回 generated `Logout200JSONResponse`。
- 异常流程：
  1. `ErrMissingPrincipal` 映射为 `AUTH-401`。
  2. 其它主体读取错误映射为内部错误或已有 `AppError`。
- 分层分工：
  - handler 负责 HTTP 错误映射。
  - service 保持 logout 业务语义。
  - repository / sqlc/database 不参与本次改动。

## 9. 并发 / 幂等 / 缓存
- logout 当前不修改服务端状态，不引入并发或幂等新问题。
- 不涉及库存、订单、支付或缓存。
- 不调整事务边界。

## 10. 权限与安全
- `POST /api/v1/auth/logout` 仍由 OpenAPI security 与认证 middleware 保护。
- 本次只补齐 handler 内部的防御性错误分类。
- JWT claim 边界不变，仍不把角色、邮箱、用户名、用户状态写入 JWT。
- 不新增敏感信息输出。

## 11. 测试策略
- 单元 / 策略测试：
  - 新增 `LogoutStrict` 认证主体错误分类测试，固化其必须区分 `ErrMissingPrincipal` 与其它错误。
- service / repository 测试：
  - 不涉及。
- migration / sqlc 验证：
  - 不涉及。
- 接口验证：
  - 运行相关 Go 测试覆盖 handler 边界。
- OpenAPI validate：
  - 不涉及 API 契约变化。
- 异常场景验证：
  - 缺失主体继续按未登录处理。
  - 非缺失错误保留内部错误映射策略。
- Java-Go parity 验证：
  - parity matrix 记录本次为 Go handler 边界一致性收敛，对外契约不变。
- 需要运行：
  - `gofmt`
  - `go test ./internal/http/handler/auth`
  - `go test ./...`
  - `go vet ./...`
  - 如工具可用，运行 `golangci-lint run` 或仓库对应 lint 命令。

## 12. 风险与替代方案
- 风险：
  - 当前非缺失主体错误运行时不可达，常规黑盒测试难以触发。
  - 工作区已有其它 strict-server 迁移改动，需要控制本次 diff 范围。
- 备选方案：
  - 保持现状：当前运行时无明显行为问题，但会让两个同类 handler 风格分裂。
  - 抽共享 helper：可减少重复，但本次只有两处，过早抽象会增加局部复杂度。
  - 修改 `RequiredPrincipal` 以制造更多错误类型：超出本次范围，会扩大安全基础设施边界。
- 选择当前方案的原因：
  - 最小化改动，仅对齐 `LogoutStrict` 的错误分类。
  - 保留当前 handler 显式映射风格，符合 strict handler 可读性约定。
  - 不引入新的抽象、依赖或对外契约变化。
- 后续可演进点：
  - 如果后续更多 handler 需要读取主体，可再评估是否抽出认证主体错误映射 helper。
