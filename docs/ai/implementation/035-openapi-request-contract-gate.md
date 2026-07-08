# OpenAPI Request Contract Gate 实现说明

## 1. 本次改动解决了什么问题

本文记录 design 035 / ADR-0027 的阶段化落地。阶段一已完成 `internal/http/validation` 到 `internal/http/requesterror` 的命名与职责收口；阶段二已完成 `OPENAPI_REQUEST_VALIDATION_ENABLED`、`OPENAPI_SPEC_PATH`、文件系统 spec loader 和 provider 启动期加载校验；阶段三已完成 path/query/body/content-type request contract gate；阶段四已完成 OpenAPI security requirement runtime 化。

本次阶段五完成三件事：

- 补齐 header parameter 与 cookie parameter violation 的错误映射扩展点，当前分别映射为 `请求头参数校验失败` 与 `Cookie 参数校验失败`。
- 明确未知 query/header/cookie 阶段性策略：当前只校验 OpenAPI 已声明字段，未知字段暂不拒绝，后续若收紧为 reject 必须补兼容性测试。
- 在 contract gate 已稳定后，删除少量已由 OpenAPI schema 完整覆盖、且有 router contract 测试证明错误 envelope 稳定的 handler 重复 transport 校验。

本次不改变对外 API path/method/字段、成功响应 envelope、JWT claim、service/domain/repository 依赖方向或数据库行为；不恢复 Go embed，不恢复 `api/openapi/spec.go:SpecYAML()`。

## 2. 改动内容
- 新增了什么
  - `requesterror.InvalidHeaders` 与 `requesterror.InvalidCookies`，继续只负责构造 HTTP request 相关 `AppError`，不执行校验。
  - `internal/http/contract/validator_test.go`，使用测试内 OpenAPI spec 覆盖 header/cookie parameter violation 和未知 query/header/cookie 当前允许通过的策略。
  - router contract 测试覆盖 echo message/tag 长度、管理员用户 status query、更新用户状态 body status enum 等由 OpenAPI schema 接管的错误场景。
- 修改了什么
  - `contract.appErrorFromValidationError` 按 `parameter.in` 将 path/query/header/cookie violation 分流到对应 request error constructor。
  - `system.parseEchoCommand` 删除 `message` 最大长度和 `tag` 最大长度校验；这两项由 OpenAPI `EchoRequest` schema 的 `maxLength` 接管。
  - `user.parseAdminUserListQuery` 删除 `page` 最小值、`size` 最小/最大值、`status` enum 校验；这些由 OpenAPI query parameter schema 接管，service 仍保留分页和状态兜底。
  - `user.parseUpdateUserStatusCommand` 删除 body `status` required/enum 校验；OpenAPI body schema 接管，service `UpdateStatus` 仍保留状态兜底。
  - design 035 补充未知 query/header/cookie 当前不拒绝的阶段性边界。
- 删除了什么
  - 删除 handler 中上述 schema-only transport 校验。
  - 未删除 auth handler 校验；register/login/refresh 仍包含 trim/normalize、邮箱解析、密码字母数字组合、空值语义等不完全等同于当前 OpenAPI schema 的规则。
- 是否更新 Java-Go parity 记录
  - 已更新。parity matrix 将 OpenAPI request contract gate 记录为阶段五实际状态：path/query/header/cookie/body/content-type/security 均已有运行时 gate、错误映射和测试索引；未知 query/header/cookie 当前是兼容允许策略。

## 3. 为什么这样设计
- 关键设计原因
  - header/cookie parameter 虽然当前业务 spec 暂无实际 cookie 参数，但扩展点必须先有测试保护；后续新增业务 header/cookie 时不需要重新判断错误 envelope 语义。
  - 未知 query/header/cookie 暂不拒绝，是为了避免代理、浏览器、灰度平台、追踪系统或历史调用方携带的非业务字段破坏现有客户端契约。
  - handler 只删除 OpenAPI schema 可以完整表达、且 contract gate 已有 router 测试证明的 transport 校验；业务组合规则不迁入 contract gate。
- 与 Go 项目当前阶段的匹配点
  - `internal/http/contract` 继续是 OpenAPI request contract gate；`internal/http/requesterror` 继续只构造 `AppError`。
  - handler 仍负责 generated request 到 service Command/Query 的映射、少量 normalize 和业务组合前置判断。
  - service/domain/repository 不依赖 `api/openapi/gen`、`internal/http/contract` 或 `internal/http/requesterror`。
- 与 Java 版业务语义的对齐方式
  - Java/Spring MVC + Bean Validation 会在进入 controller/service 前拦截多数 transport contract violation；Go 端以 OpenAPI contract gate 显式执行同类语义。
  - Java 版业务组合规则与 service 层兜底不会因为 OpenAPI schema 存在而消失；Go 端也保留 service 对分页、用户状态、userId 等关键输入的防御。

## 4. 替代方案
- 方案 A：直接拒绝所有未知 query/header/cookie。
- 方案 B：继续让 header/cookie violation 使用通用 `InvalidParameters` message。
- 方案 C：一次性删除 auth/system/user handler 中所有看起来像字段校验的逻辑。
- 为什么没有采用
  - 不采用方案 A：当前没有兼容性证据证明所有客户端都不会携带未知字段，贸然 reject 可能破坏历史调用方或基础设施注入字段。
  - 不采用方案 B：header/cookie 是企业扩展常见入口，提前稳定更具体的错误 message 有利于未来客户端定位问题。
  - 不采用方案 C：auth handler 中仍有 schema 不完整覆盖的规则；user handler 中仍有时间范围、日期实际解析、文本筛选长度兜底等 handler/service 边界规则，不能误迁到 contract gate。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./internal/http/contract -run 'TestRequestValidatorMaps(Header|Cookie)ParameterViolation|TestRequestValidatorAllowsUnknown' -count=1` 先失败，失败原因为 header/cookie violation 仍返回通用 `请求参数校验失败`。
  - GREEN：`go test ./internal/http/requesterror ./internal/http/contract -run 'TestInvalid(Header|Cookie)s|TestRequestValidatorMaps(Header|Cookie)ParameterViolation|TestRequestValidatorAllowsUnknown' -count=1` 通过。
  - GREEN：`go test ./internal/http -run 'TestOpenAPIRequestContractGateRejects(EchoSchemaLengthViolation|EchoTagSchemaLengthViolation|InvalidStatusQueryBeforeHandler|InvalidUpdateStatusBodyBeforeHandler)' -count=1` 通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt`：已对本次修改的 Go 文件执行。
  - `go test ./internal/http/handler/... ./internal/http/contract ./internal/http -count=1`：通过。
  - `go test ./...`：通过。
  - `go vet ./...`：通过，无输出。
  - `make openapi-check`：通过，OpenAPI validate/generate 后 generated 文件无漂移。
  - `make openapi-lint`：通过，Redocly lint 认为 API description valid。
  - `make lint`：通过，输出 `0 issues.`。
  - `git diff --check`：通过，无输出。
  - `sqlc generate`：未运行，本次不涉及 SQL、schema、repository 或 sqlc 配置。
- 手工验证了哪些场景
  - 未做额外手工 HTTP 请求；本阶段通过 contract/router 自动化测试覆盖。
- Java-Go parity 如何验证
  - 主 parity matrix 已更新 OpenAPI request contract gate 实际状态，并记录未知 query/header/cookie 当前允许通过。
- 结果如何
  - 阶段五 RED/GREEN 定向测试、定向包测试、全量 Go 测试、vet、OpenAPI check/lint、Go lint 和补丁空白检查均通过。

## 6. 已知限制
- 未知 query/header/cookie 当前不拒绝；这是一项兼容性策略，不代表这些字段具备业务语义。
- schema violation 的字段级 message 仍主要来自 `kin-openapi` 的底层 reason；本阶段只稳定 envelope message、字段名和 location 分类。
- auth handler 的 register/login/refresh 校验暂不收敛，因为当前 schema 尚未完整表达 trim 后空值、密码必须同时包含字母和数字、登录空密码等语义。
- user handler 继续保留时间字符串实际解析、时间区间先后关系、username/email 筛选长度和 nil body/path guard 等规则；这些不是本阶段要迁入 contract gate 的内容。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - OpenAPI 已从文档和代码生成源推进为运行时请求契约、安全策略和 transport 校验收敛的事实来源，展示 spec-first 后端治理能力。
- 对微服务 / 云原生演进的影响
  - 显式文件系统 `OPENAPI_SPEC_PATH` 便于容器、Kubernetes ConfigMap/镜像资产、网关或 sidecar 共享同一份 spec。
  - contract gate 可作为服务内 defense-in-depth，也可与未来 API gateway request validation 协同。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续如果要拒绝未知 query/header/cookie，应增加 policy 开关、兼容性测试和必要的灰度策略。
  - 不影响 migration、sqlc 或 OpenAPI generated code。
