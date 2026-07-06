# Strict Handler Command Parser 边界收敛实现说明

## 1. 本次改动解决了什么问题

本次改动解决了 strict handler 中“先 validate generated request，再在 strict 方法体内重复组装 service command”的职责分裂问题。

改动前，auth、system、user 的部分 strict 方法在调用 `validateXxxRequest` 后，仍需要再次读取 generated request 字段并手写 service command。这样会让校验值、规范化值和 service 入参值存在分叉风险；管理员用户列表 query 已经在上一轮收敛为 `parseAdminUserListQuery`，本次把其它同类 body/path 场景也统一到 parse-to-command 风格。

改动后，相关 helper 直接返回 service command，strict 方法主流程收敛为调用 parser、调用 service、构造 generated response；body nil 可由 strict 方法或 parser 根据当前端点错误优先级处理。

本次追加收敛了 `ListAdminUsersStrict` 中的分页结果转换：分页元数据与 `items` 映射从 strict 方法体抽到局部 data mapping helper，strict 方法继续只表达 parse query、调用 service、填充 success envelope 的主流程。

## 2. 改动内容
- 新增了什么
  - `internal/http/handler/auth/validation_test.go`
    - 覆盖 register/login/refresh parser 返回 command 的字段值。
    - 固化 username/email/login identifier 会 trim 后进入 command，password/refreshToken 保持原样。
  - `internal/http/handler/system/validation_test.go`
    - 覆盖 echo parser 保持 message/tag 原样进入 command。
  - `docs/ai/design/033-strict-handler-command-parsers.md`。
  - `docs/ai/implementation/033-strict-handler-command-parsers.md`。
- 修改了什么
  - `internal/http/handler/auth/validation.go`
    - `validateRegisterRequest` -> `parseRegisterCommand`。
    - `validateLoginRequest` -> `parseLoginCommand`。
    - `validateRefreshTokenRequest` -> `parseRefreshCommand`。
    - 修复 `parseRefreshCommand`：继续用 trim 后值判断空白，但 service command 保留原始 refresh token。
  - `internal/http/handler/auth/strict.go`
    - register/login/refresh strict 方法改为调用 parser 返回的 service command。
    - 抽出 `toOpenAPILoginResponse`、`toOpenAPIRefreshTokenResponse` 和共享 `toOpenAPITokenPairResponse`，集中处理 login / refresh token pair data mapping。
  - `internal/http/handler/auth/strict_test.go`
    - 增加 login / refresh token pair data mapping 测试。
  - `internal/http/handler/user/strict_test.go`
    - 增加 admin user page data mapping 测试，覆盖分页字段、用户列表项字段和 nil roles 归一化。
  - `internal/http/handler/system/validation.go`
    - `validateEchoRequest` -> `parseEchoCommand`。
  - `internal/http/handler/system/strict.go`
    - echo strict 方法改为调用 parser 返回的 `systemsvc.EchoCommand`。
  - `internal/http/handler/user/admin_validation.go`
    - `validateUpdateUserStatusRequest` -> `parseUpdateUserStatusCommand`，同时处理 path `userId`、body 缺失和 body `status`，并保持 path 校验优先级。
  - `internal/http/handler/user/admin_validation_test.go`
    - 增加 update-status parser 测试。
  - `internal/http/handler/user/strict.go`
    - update-status strict 方法改为调用 parser 返回的 `usersvc.UpdateUserStatusCommand`。
    - `ListAdminUsersStrict` 改为调用 `toOpenAPIAdminUserPage` 构造 generated page response data。
  - `docs/ai/parity/java-go-parity-matrix.md`
    - 索引 strict handler command parser 边界收敛。
- 删除了什么
  - 删除对应旧 `validateXxxRequest` 私有 helper。
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次不改变 Java-facing API 契约，但调整了 Go strict handler 内部的 generated model 到 service Command / Query 映射约定，属于 intentional Go-only structure 需要记录。

## 3. 为什么这样设计
- 关键设计原因
  - handler 私有 parser 是 HTTP generated model 到 service command 的自然边界。
  - parser 让校验、必要 normalize 和 command 构造共享同一份字段值，降低后续维护时的分叉风险。
  - strict 方法保持清晰的主流程：body 检查、parse command、service call、response mapping。
  - 分页 data mapping 独立后，`ListAdminUsersStrict` 不再同时承担分页字段逐项搬运，和 auth token pair mapping 保持同一类局部 helper 风格。
- 与 Go 项目当前阶段的匹配点
  - 不新增 HTTP DTO，不修改 service command 类型。
  - 不抽象成跨模块 validator，保留各模块字段规则的局部可读性。
  - service、domain、repository 仍不依赖 generated OpenAPI model。
- 与 Java 版业务语义的对齐方式
  - Java 版 controller/DTO 负责 HTTP 入参校验，service 接收业务输入。
  - Go 版用 strict handler parser 显式完成 generated request 到 service command 的转换，保持业务契约对齐但采用 Go idiom。

## 4. 替代方案
- 方案 A：只改函数名，不返回 command。
  - 没有采用。这样仍会让 strict 方法二次读取 request 字段构造 command，职责没有真正收敛。
- 方案 B：把 body nil 检查也放入 parser。
  - 部分采用。register/login/refresh/echo 仍由 strict 方法直接处理 body 缺失；update-status 因为需要保持 path `userId` 校验优先于 body 缺失，所以由 path + body parser 一并处理。
- 方案 C：抽通用 body parser 或 validator。
  - 没有采用。register/login/refresh/echo/update-status 字段规则差异明显，通用抽象会增加复杂度。
- 方案 D：顺手 trim password、refresh token 或 echo message。
  - 没有采用。这会改变凭证或回显行为，超出本次边界收敛范围。
- 方案 E：抽 strict 执行模板来消除更多重复。
  - 没有采用。模板会把 generated response 类型、service call 和错误映射塞进回调，降低 strict 方法主流程可读性；本轮只抽 token pair 与 admin user page 这类局部 data mapping。
- 方案 F：保留 `ListAdminUsersStrict` 内联分页转换。
  - 没有采用。分页字段和 item 映射属于 generated response data mapping，抽到局部 helper 后更容易单独测试，也避免 strict 方法继续扩张。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./internal/http/handler/auth ./internal/http/handler/system ./internal/http/handler/user`
    - 失败原因符合预期：`undefined: parseRegisterCommand`、`undefined: parseLoginCommand`、`undefined: parseRefreshCommand`、`undefined: parseEchoCommand`、`undefined: parseUpdateUserStatusCommand`。
  - GREEN：`go test ./internal/http/handler/auth ./internal/http/handler/system ./internal/http/handler/user`
    - 通过。
  - 追加 RED：`go test ./internal/http/handler/user`
    - 失败原因符合预期：`parseUpdateUserStatusCommand` 尚未接受 nil body，且测试暴露 `AppError.Details` 需要通过方法读取。
  - 追加 GREEN：`go test ./internal/http/handler/user`
    - 通过。
  - 追加 RED：`go test ./internal/http/handler/auth`
    - 失败原因符合预期：`toOpenAPILoginResponse` / `toOpenAPIRefreshTokenResponse` 尚未定义；修正测试签名后也能覆盖 refresh token 原样进入 command 的语义。
  - 追加 GREEN：`go test ./internal/http/handler/auth`
    - 通过。
  - 追加 GREEN：`go test ./internal/http/handler/auth ./internal/http/handler/user`
    - 通过，覆盖 auth token pair data mapping 和 admin user page data mapping。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w internal/http/handler/auth/validation.go internal/http/handler/auth/validation_test.go internal/http/handler/auth/strict.go internal/http/handler/system/validation.go internal/http/handler/system/validation_test.go internal/http/handler/system/strict.go internal/http/handler/user/admin_validation.go internal/http/handler/user/admin_validation_test.go internal/http/handler/user/strict.go internal/http/handler/user/strict_test.go`：已执行。
  - `gofmt -l internal/http/handler/auth/validation.go internal/http/handler/auth/validation_test.go internal/http/handler/auth/strict.go internal/http/handler/system/validation.go internal/http/handler/system/validation_test.go internal/http/handler/system/strict.go internal/http/handler/user/admin_validation.go internal/http/handler/user/admin_validation_test.go internal/http/handler/user/strict.go internal/http/handler/user/strict_test.go`：无输出。
  - `go test ./internal/http/handler/auth ./internal/http/handler/system ./internal/http/handler/user`：通过。
  - `go test ./...`：通过。
  - `go test -count=1 ./...`：通过，用于确认非缓存全量测试结果。
  - `go vet ./...`：通过。
  - `golangci-lint run`：通过，输出 `0 issues.`。
  - `git diff --check -- internal/http/handler/auth/validation.go internal/http/handler/auth/validation_test.go internal/http/handler/auth/strict.go internal/http/handler/system/validation.go internal/http/handler/system/validation_test.go internal/http/handler/system/strict.go internal/http/handler/user/admin_validation.go internal/http/handler/user/admin_validation_test.go internal/http/handler/user/strict.go docs/ai/design/033-strict-handler-command-parsers.md docs/ai/implementation/033-strict-handler-command-parsers.md docs/ai/parity/java-go-parity-matrix.md`：通过。
  - `sqlc generate`：未运行，本次不涉及 SQL、schema 或 sqlc 配置。
  - OpenAPI validate / generate：未运行，本次不涉及 OpenAPI 契约或 generated code。
- 手工验证了哪些场景
  - 检查旧 `validateRegisterRequest`、`validateLoginRequest`、`validateRefreshTokenRequest`、`validateEchoRequest`、`validateUpdateUserStatusRequest` 引用已替换为 parse helper。
  - 检查 password、refreshToken、echo message/tag 仍按原始 request 值进入 command。
  - 检查 login / refresh generated response data 由局部 mapping helper 统一填充。
  - 检查 admin user page generated response data 由局部 mapping helper 统一填充。
  - 检查 update-status 保持 path `userId` 校验优先于 body 缺失。
- Java-Go parity 如何验证
  - 更新 `docs/ai/parity/java-go-parity-matrix.md` 的 system 与 Auth、当前用户与管理员用户 API 行。
  - 确认相关 API 的路径、请求字段、响应字段、错误码和状态码不变。
- 结果如何
  - 定向 RED/GREEN、全量 Go 测试、vet、lint 和格式化检查均通过。

## 6. 已知限制
- helper 文件名仍为 `validation.go`，但内部主要函数已是 parse-to-command。后续如果文件内容继续偏向 parser，可单独评估文件重命名。
- body nil 检查多数仍留在 strict 方法中；update-status parser 为了合并 path/body 并保持旧错误优先级，会接收 body pointer。
- service 层仍保留部分 normalize，例如 email lower-case、login identifier lower-case，这些仍是业务/持久化前的防御边界。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - strict handler 的输入边界更清楚，读者可以直接看到 generated request 如何成为 service command。
- 对微服务 / 云原生演进的影响
  - 无直接影响，但保留了 service 层 normalize 和凭证原样传递语义，便于未来接入其它调用源。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续新增 strict handler 时，优先采用 `parseXxxCommand` / `parseXxxQuery` 表达 HTTP 到 service 的转换。
  - 不影响 migration、sqlc 或 OpenAPI 生成策略。
