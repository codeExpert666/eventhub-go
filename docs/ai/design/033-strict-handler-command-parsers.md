# Strict Handler Command Parser 边界收敛

## 1. 背景
- 当前 strict handler 中有一类重复模式：先调用 `validateXxxRequest` 校验 generated request body，再在 strict 方法体内手写 service `Command` / `Query`。
- 上一轮管理员用户列表已经将 query 参数处理收敛为 `parseAdminUserListQuery`，让校验值、规范化值和 service query 构造来自同一个流程。
- 现有 `auth`、`system`、`user` handler 仍保留部分“校验后立即转 command”的 `validateXxxRequest` helper，命名和职责不够一致。
- Java 版对应语义仍以 controller/DTO 校验 + service 入参转换为参考；Go 版不逐行迁移 Java 结构，但 strict handler 应清楚表达 generated HTTP model 到 service Command/Query 的边界。

## 2. 目标
- 将校验后马上转 service 对象的 helper 收敛为 `parseXxxCommand` / `parseXxxQuery` 命名与返回值。
- 覆盖范围：
  - `auth`: register、login、refresh。
  - `system`: echo。
  - `user`: update admin user status。
- 保持 API 路径、请求字段、响应字段、状态码、错误文案和既有错误优先级不变。
- 保持凭证和回显字段的现有行为：
  - `password` 原样传递。
  - `refreshToken` 原样传递。
  - `echo.message` 和 `echo.tag` 原样传递。
- 抽出 login / refresh 共享的 token pair data mapping，以及管理员用户列表分页结果 data mapping，降低 generated response data 构造重复度，但不抽 strict 主流程模板。
- 成功标准是 strict 方法不再先 validate 再重复组装对应 command，测试能证明 parse helper 返回正确 command，且 token pair / admin user page data mapping 字段完整。

## 3. 非目标
- 不调整 `openapi.validateStaticAssets`，它是启动期资源校验，不是 HTTP request 到 service command 的转换。
- 不新增共享 validator 框架或跨模块 parser 抽象。
- 不修改 service Command / Query / Result 类型结构。
- 不修改 `page.Response`、OpenAPI generated page response model 或分页语义。
- 不调整 repository、sqlc、migration、OpenAPI spec 或 generated code。
- 不改变密码、token 或 echo 字段的 trim 策略；refresh token 只用 trim 后值判断空白，service command 必须保留原始 opaque token。

## 4. 影响范围
- 涉及 Go package：
  - `internal/http/handler/auth`
  - `internal/http/handler/system`
  - `internal/http/handler/user`
- 涉及 API：
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/system/echo`
  - `PATCH /api/v1/admin/users/{userId}/status`
- 不涉及数据库表、缓存、外部接口、OpenAPI generated model 或 service package。
- Java-Go parity matrix 需要索引本次 Go strict handler 边界收敛；对外契约状态仍为已对齐。

## 5. 领域建模
- generated request model 仍只属于 HTTP 层，例如 `openapigen.RegisterRequest`、`openapigen.EchoRequest`。
- service command 仍属于 service 层输入，例如 `authsvc.RegisterCommand`、`systemsvc.EchoCommand`、`usersvc.UpdateUserStatusCommand`。
- `parseXxxCommand` 是 HTTP handler 私有 helper，负责在 HTTP 层完成校验、必要 normalize 和 command 构造。
- auth token pair data mapping 是 HTTP handler 私有 helper，负责把 `authsvc.LoginResult` / `authsvc.RefreshResult` 映射为 OpenAPI generated response data。
- admin user page data mapping 是 HTTP handler 私有 helper，负责把 `page.Response[usersvc.UserResult]` 映射为 OpenAPI generated page response data。
- 与 Java 版关系：
  - Java DTO/Bean Validation 负责入参校验。
  - Go strict handler helper 负责 generated model 到 service command 的显式转换，不让 service/domain/repository 依赖 generated model。

## 6. API 设计
- 接口列表与请求/响应结构不变。
- 错误码 / 异常场景不变：
  - malformed body 继续返回现有 body 解析错误。
  - body 字段校验继续使用 `validation.BodyValidationError`。
  - path/query 参数错误继续使用 `validation.ParameterValidationError`。
- 字段处理规则：
  - register username/email：trim 后进入 command；email 小写化仍由 service 层负责。
  - register password：原样进入 command。
  - login usernameOrEmail：trim 后进入 command；是否 lower-case 仍由 service 层负责。
  - login password：原样进入 command。
  - refresh token：用 trim 后值判断空白，但原始 opaque token 进入 command。
  - echo message/tag：按现有规则校验，原始值进入 command。
  - update user status：path `userId` 与 body `status` 一起构造 service command。
- 响应 data mapping 规则：
  - login / refresh：共享 token pair 字段映射，保留各接口 generated data 类型。
  - admin user list：分页元数据和 `items` 由局部 helper 从 service result 映射到 generated page data。
- 与 Java 版 OpenAPI / controller 契约差异：无新的对外契约差异。

## 7. 数据设计
- 不调整表结构、索引、唯一约束或 migration。
- 不新增或修改 sqlc query、generated model 或 repository model。
- 数据一致性边界不变。

## 8. 关键流程
- 正常流程：
  1. strict server 解码 generated request object。
  2. strict 方法或 parser 检查 body 是否存在；update-status 这类 path + body 组合 parser 先校验 path，再处理 body 缺失，以保持既有错误优先级。
  3. `parseXxxCommand` 校验字段并构造 service command。
  4. strict 方法调用对应 service。
  5. strict 方法通过局部 data mapping helper 构造 generated response data，再填充 generated response envelope。
- 异常流程：
  - body 缺失由 strict 方法或 parser 返回 `validation.MalformedBodyError`。
  - 字段校验失败由 parser 返回 `validation.BodyValidationError` 或 `ParameterValidationError`。
- 分层分工：
  - handler 负责 HTTP 入参校验、必要 normalize、service command 构造和响应映射。
  - service 继续负责业务规则、事务边界、凭证处理和最终 normalize。
  - repository / sqlc/database 不参与本次改动。

## 9. 并发 / 幂等 / 缓存
- 本次不改变写入业务流程、事务边界、session 轮换、库存、订单或缓存。
- refresh token 幂等/重放语义仍在 service 和 repository 条件更新中处理。

## 10. 权限与安全
- 认证和 RBAC middleware 不变。
- JWT claim 边界不变，仍不把角色、邮箱、用户名、用户状态写入 JWT。
- 密码和 refresh token 不被 parser 静默 trim 后传给 service，避免凭证语义被 handler 隐式改写。
- 参数仍通过 service/repository/sqlc 参数化路径传递，不拼接 SQL。

## 11. 测试策略
- 单元测试：
  - 新增/更新 handler 包测试，覆盖 parser 返回 service command 的字段值。
  - 覆盖 username/email/login identifier trim 后进入 command。
  - 覆盖 password、refresh token、echo message/tag 原样进入 command。
  - 覆盖 login / refresh token pair data mapping 字段完整。
  - 覆盖 admin user page data mapping 的分页字段、列表项字段和 nil roles 归一化。
  - 覆盖 update-status parser 同时处理 path userId 和 body status，并保持 path 校验优先于 body 缺失。
- service / repository 测试：
  - 不涉及。
- migration / sqlc 验证：
  - 不涉及。
- 接口验证：
  - 运行相关 handler 包测试和全量 Go 测试。
- OpenAPI validate：
  - 不涉及 API 契约变化。
- 异常场景验证：
  - parser 继续返回既有 validation error helper。
- Java-Go parity 验证：
  - 更新 parity matrix 的 strict handler / auth/user/system API 索引。
- 需要运行：
  - `gofmt`
  - `go test ./internal/http/handler/auth ./internal/http/handler/system ./internal/http/handler/user`
  - `go test ./...`
  - `go vet ./...`
  - `golangci-lint run`

## 12. 风险与替代方案
- 风险：
  - 工作区已有 strict-server 迁移相关未提交改动，需要严格控制本次 diff。
  - 将 parser 做得过宽可能误改凭证或 echo 行为。
- 备选方案：
  - 只改函数名，不返回 command。
    - 未采用原因：这不能解决 strict 方法内重复组装 command 的职责分裂。
  - 把所有 body nil 检查都固定在 strict 方法。
    - 未采用原因：auth parser 接收 body pointer 可以让 strict 方法只关注 parse command / service / response 主流程；update-status 这类 path + body 组合 parser 也需要纳入 body nil 检查以保持 path 校验优先级。
  - 抽通用 parser/validator。
    - 未采用原因：当前模块字段规则差异明显，抽象会增加认知负担。
  - 抽 strict 执行模板。
    - 未采用原因：会把 generated response 类型、service call 和错误映射塞进回调，降低 strict 方法主流程可读性。
  - 不抽 data mapping，接受 IDE 重复提示。
    - 未采用原因：login / refresh 的 token pair data 字段高度一致，抽局部映射 helper 能降低重复且不影响 strict 主流程可读性。
- 后续可演进点：
  - 如果更多模块采用 strict-server，可沿用“HTTP model -> parse command/query -> service”的局部 helper 约定。
