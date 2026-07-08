# Enterprise Field Validation

## 1. 背景
- Java 版 EventHub 主要通过 Jakarta Bean Validation 在 Controller 边界完成请求字段校验。典型来源包括：
  - `RegisterRequest.java`：`@NotBlank`、`@Size`、`@Pattern`、`@Email` 及字段级中文消息。
  - `LoginRequest.java` / `RefreshTokenRequest.java`：登录标识、密码、refresh token 的空值与长度消息。
  - `AdminUserQueryRequest.java`：分页、筛选长度、状态枚举、`createdAtFrom <= createdAtTo`、`updatedAtFrom <= updatedAtTo` 等 query 与跨字段规则。
  - `EchoRequest.java`：message/tag 的空值和长度规则。
- Go 版当前已经采用 spec-first OpenAPI，并通过 `internal/http/contract.RequestValidator` 在 generated strict handler 前执行 OpenAPI request contract gate。该能力已经覆盖 path/query/header/cookie/body/content-type/security 的基础契约校验。
- 当前不足是字段校验消息与部分规则仍分散在 handler 私有 parser 中，例如 `internal/http/handler/auth/validation.go`、`internal/http/handler/user/admin_validation.go`、`internal/http/handler/system/validation.go`。这会导致 OpenAPI schema、Java Bean Validation 语义和 Go handler 手写规则之间长期双写、漂移和审查成本上升。
- 本设计的目标不是兼容当前手写校验风格，而是为 Go 版建立企业级字段校验体系：OpenAPI spec 是字段规则与字段消息的唯一契约源，`internal/http/contract` 是唯一运行时执行入口，handler 只做 generated request 到 service Command / Query 的映射。

## 2. 目标
- 建立基于 OpenAPI vendor extension 的字段校验规范，推荐使用 `x-validation` 承载 OpenAPI 原生 schema 无法完整表达的项目语义与中文错误消息。
- 让 `api/openapi/eventhub.yaml` 同时表达：
  - 标准 schema 规则：`required`、`minLength`、`maxLength`、`pattern`、`format`、`enum`、`minimum`、`maximum`、`additionalProperties`。
  - 项目规则：`notBlank`、`containsLetterAndDigit`、`notAfter` 等。
  - 字段级 / operation 级中文错误消息。
- 在 `internal/http/contract` 中新增 rule catalog 与 custom rule engine：
  - 启动期解析 OpenAPI spec 中的 `x-validation`。
  - 运行时优先使用 catalog 中的字段消息映射 OpenAPI schema violation。
  - 对 OpenAPI 原生 schema 无法表达或表达不稳定的规则执行自定义校验。
- 将请求校验错误结构升级为稳定的 violations 列表，便于前端表单、日志审计和后续国际化。
- 删除 handler 中所有字段合法性校验，让 handler 只保留：
  - nil body 防御。
  - trim、lowercase 等输入规范化。
  - generated model 到 service Command / Query / Result 的映射。
- 将 production / Docker 默认策略从 request validation 默认关闭调整为默认开启；保留显式关闭开关作为应急 break-glass，而不是常态部署策略。
- 建立 OpenAPI policy test，要求新增请求字段和新增项目规则必须声明稳定消息。
- 保持 Java-Go parity：对齐 Java Bean Validation 的字段语义与用户可读消息，但用 Go spec-first 架构实现，不迁移 Java 注解机制。

## 3. 非目标
- 不引入 `go-playground/validator` 作为主校验路径；它会把规则重新写到 Go struct tag 上，与 spec-first 冲突。
- 不把 OpenAPI spec 重新 Go embed 到二进制中；继续使用显式文件系统部署资产和 `OPENAPI_SPEC_PATH`。
- 不改变 service/domain/repository 分层，不让 service/domain/repository 依赖 `api/openapi/gen`、`internal/http/contract` 或 `internal/http/requesterror`。
- 不把业务规则迁入字段校验层。账号唯一、用户是否禁用、refresh token 是否过期或重放、session 状态、数据库唯一约束仍属于 service/repository。
- 不在本设计阶段实现代码。本设计先冻结企业级目标态和阶段边界，后续由执行阶段逐步落地。
- 不一次性迁移未来 event/order/payment/inventory 模块；本次先覆盖当前已实现的 system/auth/user API。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `api/openapi/eventhub.yaml`：新增 `x-validation` 规则和消息。
  - `api/openapi/openapi_policy_test.go`：新增字段校验扩展规范测试。
  - `api/openapi/gen/{models.gen.go,server.gen.go}`：OpenAPI schema 或错误响应 schema 调整后需要重新生成。
  - `internal/http/contract`：新增 rule catalog、violation model、custom rules、schema error message mapper。
  - `internal/http/requesterror`：从 `FieldErrors map[string]any` 逐步演进到稳定 `violations` details。
  - `internal/http/handler/{auth,system,user}`：删除字段合法性校验，保留 mapper / normalize。
  - `internal/config`、`configs/*.env.example`、`Dockerfile`、`README.md`：调整 production request validation 默认策略和说明。
  - `docs/ai/parity/java-go-parity-matrix.md`：新增企业级字段校验体系索引。
- 涉及 API / 表 / 缓存 / 外部接口：
  - 不新增业务 endpoint。
  - 不涉及数据库表、migration、sqlc query、Redis 或外部服务。
  - 错误响应 `data` 结构会更稳定地表达 `violations`，属于错误响应 contract 细化。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 影响。本设计把 Java Bean Validation 对齐方式从 handler 手写校验升级为 OpenAPI spec + contract gate 的已决策架构。

## 5. 领域建模
- `ValidationRule`
  - 表示字段或 operation 上的一条可执行规则。
  - 标准规则来自 OpenAPI schema，例如 `required`、`minLength`、`format`、`enum`。
  - 项目规则来自 `x-validation.rules`，例如 `notBlank`、`containsLetterAndDigit`、`notAfter`。
- `ValidationMessage`
  - 表示某个 operation/location/field/rule 对应的用户可读中文消息。
  - 优先级：字段级 `x-validation.messages` > operation 级 `x-validation.messages` > 项目默认消息。
- `ValidationCatalog`
  - 启动期从 OpenAPI doc 编译出的只读索引。
  - 推荐索引维度：
    - `operationId`
    - `location`：`body`、`query`、`path`、`header`、`cookie`
    - `field` / JSON pointer
    - `rule`
- `Violation`
  - 运行时返回给 HTTP 错误 envelope 的结构化校验失败项。
  - 推荐字段：
    - `location`
    - `field`
    - `path`
    - `rule`
    - `message`
- `CustomRule`
  - Go 代码中实现的项目级 rule 函数。
  - 初始只支持当前 Java parity 必需规则：
    - `notBlank`
    - `containsLetterAndDigit`
    - `notAfter`
- 与 Java 版领域对象的对应关系：
  - Java `@NotBlank` -> Go `x-validation.notBlank` / `x-validation.rules: notBlank`。
  - Java `@Size` -> OpenAPI `minLength` / `maxLength` + `x-validation.messages`。
  - Java `@Pattern` -> OpenAPI `pattern` + `x-validation.messages`，密码字母数字组合可用项目 `containsLetterAndDigit` 避免复杂正则兼容问题。
  - Java `@Email` -> OpenAPI `format: email` + `x-validation.messages`。
  - Java `@Min` / `@Max` -> OpenAPI `minimum` / `maximum` + `x-validation.messages`。
  - Java `@AssertTrue` 跨字段校验 -> operation 级 `x-validation.crossFields`。

## 6. API 设计
- 接口列表：
  - 不新增业务接口。
  - 当前已实现 API 均纳入企业级字段校验：
    - `POST /api/v1/auth/register`
    - `POST /api/v1/auth/login`
    - `POST /api/v1/auth/refresh`
    - `POST /api/v1/auth/logout`
    - `GET /api/v1/me`
    - `GET /api/v1/admin/users`
    - `PATCH /api/v1/admin/users/{userId}/status`
    - `POST /api/v1/system/echo`
    - `GET /api/v1/system/ping`
    - actuator health/info
- 请求参数：
  - 参数合法性统一由 OpenAPI schema 和 `x-validation` 承载。
  - handler 不再对字段合法性调用 `requesterror.InvalidBody` 或 `requesterror.InvalidParameters`。
  - trim / lowercase 是 normalize，不是字段合法性判断，可留在 mapper/service 防御边界。
- 推荐 `x-validation` 写法：

```yaml
RegisterRequest:
  type: object
  required:
    - username
    - email
    - password
  properties:
    username:
      type: string
      minLength: 3
      maxLength: 32
      pattern: '^[A-Za-z0-9_]+$'
      x-validation:
        notBlank: true
        messages:
          required: username 不能为空
          notBlank: username 不能为空
          minLength: username 长度必须在 3 到 32 个字符之间
          maxLength: username 长度必须在 3 到 32 个字符之间
          pattern: username 只能包含字母、数字和下划线
    password:
      type: string
      minLength: 8
      maxLength: 72
      x-validation:
        notBlank: true
        rules:
          - name: containsLetterAndDigit
            message: password 至少包含字母和数字
        messages:
          required: password 不能为空
          notBlank: password 不能为空
          minLength: password 长度必须在 8 到 72 个字符之间
          maxLength: password 长度必须在 8 到 72 个字符之间
```

- operation 级跨字段规则示例：

```yaml
x-validation:
  crossFields:
    - name: createdAtRange
      rule: notAfter
      left: createdAtFrom
      right: createdAtTo
      message: createdAtFrom 不能晚于 createdAtTo
    - name: updatedAtRange
      rule: notAfter
      left: updatedAtFrom
      right: updatedAtTo
      message: updatedAtFrom 不能晚于 updatedAtTo
```

- 响应结构：
  - 错误 envelope 仍为 `code/message/data/requestId/timestamp`。
  - `data` 对字段校验错误使用稳定 `violations`：

```json
{
  "violations": [
    {
      "location": "body",
      "field": "username",
      "path": "username",
      "rule": "notBlank",
      "message": "username 不能为空"
    }
  ]
}
```

- 错误码 / 异常场景：
  - 字段校验失败：`COMMON-400`。
  - 请求体缺失或 JSON 格式错误：`COMMON-400`，`message=请求体格式不合法`。
  - body 字段规则失败：`COMMON-400`，`message=请求体参数校验失败`。
  - path/query 规则失败：`COMMON-400`，`message=请求参数校验失败`。
  - header 规则失败：`COMMON-400`，`message=请求头参数校验失败`。
  - cookie 规则失败：`COMMON-400`，`message=Cookie 参数校验失败`。
  - content-type 不支持：`COMMON-400`，`message=请求内容类型不支持`。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Java 使用注解和 Bean Validation provider 执行字段校验。
  - Go 使用 OpenAPI `eventhub.yaml` + `x-validation` + `internal/http/contract` 执行字段校验。
  - Go 的设计让 API 文档、生成代码、运行时校验、错误消息和 policy test 共享同一个 spec-first 来源。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 字段校验更早拒绝无效输入，不改变数据库一致性策略。
  - 账号唯一、角色关系、session 状态等仍由 service/repository 和数据库约束保证。

## 8. 关键流程
- 启动流程：
  1. `config.Load()` 读取 `OPENAPI_SPEC_PATH` 与 `OPENAPI_REQUEST_VALIDATION_ENABLED`。
  2. provider 通过 `contract.LoadSpec` 加载、ResolveRefs、Validate OpenAPI doc。
  3. `contract` 从 doc 编译 `ValidationCatalog`，校验 `x-validation` 写法是否合法。
  4. `contract.NewRequestValidator` 持有只读 spec、router、security bridge、validation catalog。
  5. router 将 contract middleware 包在 generated strict server 前。
- 请求流程：
  1. contract gate 匹配 OpenAPI operation。
  2. 先执行 security requirement，保持未认证请求优先返回 `AUTH-401/403`。
  3. 执行 OpenAPI schema validation。
  4. 将 schema violation 通过 catalog 映射为稳定中文消息。
  5. 执行 custom rules，例如 `notBlank`、`containsLetterAndDigit`、`notAfter`。
  6. 如有 violation，写出统一 error envelope，不进入 handler。
  7. 如无 violation，回放 body，进入 generated strict handler。
  8. module handler 只做 request -> Command/Query 映射、normalize 和 response mapping。
- handler / service / repository / sqlc/database 分工：
  - handler：HTTP/generated model 映射，不做字段合法性判断。
  - service：业务规则、事务边界、状态流转。
  - repository：持久化语义。
  - sqlc/database：查询执行与 schema 映射。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无。
- 如何防重复提交：不涉及订单或写入幂等。
- 事务边界在哪里：不改变现有 service/repository 事务边界。
- 缓存放在哪里，为什么：
  - `ValidationCatalog` 是启动期编译的只读内存索引，属于 contract metadata，不是业务缓存。
  - 不引入 Redis 或外部缓存。
- 并发考虑：
  - `ValidationCatalog` 和 OpenAPI doc 在启动完成后只读共享。
  - 每个请求的 body buffer、violation slice 和 custom rule input 均为请求内局部变量。

## 10. 权限与安全
- 哪些角色能访问：
  - 沿用现有 OpenAPI `security: BearerAuth` 和 `x-required-roles`。
- 鉴权与鉴别约束：
  - 认证/授权仍由 contract security bridge 驱动，不由字段校验扩展驱动。
  - 字段校验不能绕过认证；受保护 operation 应先完成认证/授权，再暴露字段级错误。
- JWT claim 边界：
  - 不把角色、邮箱、用户名、用户状态写入 JWT。
- 是否涉及敏感信息、审计或操作日志：
  - violation message 必须是用户可读业务提示，不泄露 Go error、内部类型、文件路径、SQL 或 token 内容。
  - password 等敏感字段可以返回字段名和规则名，但不记录或回显原始值。

## 11. 测试策略
- 单元测试：
  - `internal/http/contract`：
    - 解析 body schema `x-validation.messages`。
    - 解析 parameter schema `x-validation.messages`。
    - 解析 operation `x-validation.crossFields`。
    - spec 中未知 rule、缺少 message、类型错误时启动期失败。
    - schema violation 优先命中 catalog message。
    - `notBlank`、`containsLetterAndDigit`、`notAfter` custom rules。
  - `internal/http/requesterror`：
    - `violations` details 的 code/message/data 结构。
- handler 测试：
  - AST 或行为测试确认 auth/system/user handler 不再用 `requesterror.InvalidBody` / `InvalidParameters` 做字段合法性校验。
  - mapper 测试只覆盖 trim/lowercase 和 Command/Query 字段映射。
- API 集成测试：
  - register：username 空白、username 长度、username pattern、email 空白、email format、password 空白、password 长度、password 字母数字组合。
  - login：usernameOrEmail 空白/超长、password 空白/超长。
  - refresh：refreshToken 空白/超长。
  - admin users：page/size、username/email 长度、status enum、createdAt/updatedAt 格式与跨字段范围。
  - update status：userId、status required/enum。
  - echo：message 空白/长度、tag 长度。
  - content-type、malformed JSON、unknown query/header/cookie 当前兼容策略。
- OpenAPI validate：
  - `make openapi-lint`
  - `make openapi-check`
  - `make openapi-breaking-check` 如 API schema 变化可能影响兼容性。
- Java-Go parity 验证：
  - 对照 Java DTO 注解消息，Go negative tests 应断言同等核心消息。
- 需要运行的命令：
  - `gofmt` 针对变更 Go 文件。
  - `go test ./internal/http/contract ./internal/http/requesterror ./internal/http -count=1`。
  - `go test ./api/openapi -count=1`。
  - `go test ./...`。
  - `go vet ./...`。
  - `make openapi-lint`。
  - `make openapi-check`。
  - `make lint`。
  - `git diff --check`。

## 12. 风险与替代方案
- 当前方案的风险：
  - `x-validation` 是项目扩展，必须用 policy test 约束，否则会成为不可维护的自由格式。
  - 错误结构从 map 演进到 `violations` 可能影响依赖旧 `data[field]` 的调用方；当前项目尚未有独立前端生产依赖，但仍应通过 OpenAPI contract 记录。
  - custom rules 可能膨胀成业务规则垃圾桶；必须限制在 HTTP 字段语义，不承载 service/domain 规则。
  - prod 默认开启 request validation 会暴露历史客户端不规范请求；需要明确这是企业级入口契约要求，并保留 break-glass 开关。
- 备选方案：
  - 方案 A：只使用 OpenAPI 原生 schema，不新增 `x-validation`。
  - 方案 B：使用 `go-playground/validator` 在 generated model 外再建 Go struct tag。
  - 方案 C：从 OpenAPI 生成 Go 校验代码。
  - 方案 D：继续保留 handler 手写字段校验，只把消息搬到常量。
- 为什么不选备选方案：
  - 不选方案 A：无法稳定表达 Java Bean Validation 的自定义消息、`notBlank`、跨字段和密码组合规则。
  - 不选方案 B：会形成 OpenAPI schema 与 Go tag 双规则源，违背 SPEC-first。
  - 不选方案 C：长期可考虑，但当前会显著增加工具链复杂度；先建立 spec 扩展和运行时 catalog 更稳。
  - 不选方案 D：只是整理历史写法，不能解决规则源分散问题，后续新增 API 仍会漂移。
- 后续可演进点：
  - 将 `x-validation` 扩展抽成仓库内规范文档或 Redocly 自定义规则。
  - 增加 i18n message key，让 `message` 支持多语言 catalog。
  - 当 API 规模扩大后，再评估从 OpenAPI 生成高性能校验器。
  - 对未知 query/header/cookie 从 allow -> warn -> reject 做灰度治理。
