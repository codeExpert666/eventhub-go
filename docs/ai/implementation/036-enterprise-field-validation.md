# Enterprise Field Validation

## 1. 本次改动解决了什么问题

- 阶段 1 已把当前 system/auth/user 请求字段的标准规则消息、`notBlank`、`containsLetterAndDigit` 和管理员时间范围 `notAfter` 声明收敛到 `api/openapi/eventhub.yaml`，并由 OpenAPI policy test 固化声明格式和 Java parity 消息。
- 阶段 2 将 HTTP 字段校验错误从松散的 `data[field] = message` map 演进为稳定的 `data.violations` 数组；每一项统一包含 `location`、`field`、`path`、`rule`、`message`。
- contract gate、generated chi 参数绑定、strict body 解码和阶段性保留的 handler 校验现在通过同一 `internal/http/requesterror` 结构输出错误，不再因入口不同而产生不同 details 形状。
- OpenAPI `ErrorResponse.data` 已能机器表达 violations，同时继续兼容 `data=null` 和非字段类 `AppError.Details`，没有把全局错误模型误收窄为“所有错误都必须有 violations”。
- 阶段 3 解决了 contract gate 仍直接暴露 kin-openapi 默认 schema reason、尚未消费阶段 1 `x-validation.messages` 的问题：启动期现已编译 `ValidationCatalog`，body/query/path 的 OpenAPI 原生 schema violation 会优先使用 spec 中声明的稳定消息。
- 阶段 3 同时把 `x-validation` 内部格式校验前移到 `NewRequestValidator`；即使通过 break-glass 关闭 request validation，非法 field/operation extension、缺失 native rule message 或未知 custom rule 也会让应用启动失败。

## 2. 改动内容

- 新增了什么
  - 在 `internal/http/requesterror` 新增 `Violation`、`Violations` 和 body/query/path/header/cookie location 常量。
  - 在 `api/openapi/eventhub.yaml` 新增 `Violation` schema；五个字段全部 required，`additionalProperties: false`。
  - 在 `ErrorResponse.data` 增加可选 `violations` 数组，并保留 `nullable: true`、`additionalProperties: true`。
  - 在 OpenAPI policy test 增加 ErrorResponse violations contract 检查，约束数组 item、五个必填 string 字段、nullable 和通用 details 兼容边界。
  - 增加 response JSON、contract header/cookie、generated query/path binder、handler 保留路径及 router contract gate 的 violations 测试。
  - 阶段 3 新增 `internal/http/contract/{validation_catalog,validation_extension,validation_violation,validation_error_mapper}.go`：分别承载启动期索引、严格 extension parser、violation 定位和 `AppError` 映射。
  - 新增只读 `ValidationCatalog`，以 `operationId/location/field-or-path/rule` 为精确消息键，并保留 operation rule 级 fallback；同时编译但不执行 `notBlank`、`containsLetterAndDigit`、`notAfter` 元数据。
  - 新增 body/query/path catalog 消息测试，覆盖 `required`、`minLength`、`maxLength`、`pattern`、`format`、`enum`、`minimum`、`maximum`，以及非法 extension 启动失败、operation 消息 fallback 和 custom rules 本阶段不执行。
- 修改了什么
  - `InvalidBody`、`InvalidParameters`、`InvalidHeaders`、`InvalidCookies` 改为接收有序 `Violations`；`MissingBody`、`MalformedBody`、`UnsupportedContentType` 也输出相同结构，并用 `required` / `malformed` 区分缺失 body 与非法 JSON。
  - `internal/http/contract` 从 OpenAPI parameter location、`SchemaError.SchemaField` 和 JSON pointer 构造 violation；body path 使用点分完整路径，字段名保持当前顶层字段语义；显式空参数使用 `rule=allowEmptyValue`。
  - generated chi 参数错误映射现在接收 `*http.Request`，通过 chi route context 区分 query/path，并把反序列化失败稳定标记为 `rule=type`；required query/header 使用与 rule 一致的“不能为空”消息。
  - auth/system/user handler 手写校验按阶段要求继续保留，但错误聚合从无序 map 改为有序 slice，并填充与阶段 1 spec 对应的 rule；没有删除或弱化原有合法性判断。
  - `response.detailsData` 适配 generated `ErrorResponse_Data`，通过其 additional properties 能力透传通用 `apperror.Details`，不依赖 `requesterror`。
  - 重新生成 `api/openapi/gen/models.gen.go`；新增 generated `Violation`、`ViolationLocation`、`ErrorResponse_Data` 及 additional properties JSON 支持。`server.gen.go` 重新生成后无内容漂移。
  - 阶段 3 的 `NewRequestValidator` 总是在已加载、已 ResolveRefs/Validate 的 OpenAPI document 上编译 catalog；构造错误继续由现有 provider 自然传播为启动失败，不重新读取文件，也不恢复 embed。
  - parameter mapper 通过 `RequestError.Input.Route.Operation.OperationID`、`Parameter.In/Name` 和 rule 查 catalog；body mapper 通过 `SchemaError.JSONPointer()` 与 `SchemaField` 查 catalog。字段消息优先于 operation 消息，均未命中时保留原 fallback。
  - catalog 同时遍历 PathItem 与 Operation effective parameters，后者按 `(in,name)` 覆盖前者；schema extension 先读取 `SchemaRef.Extensions`，再读取 `SchemaRef.Value.Extensions`，确保 `$ref` sibling 上的两个 status 消息可命中。
  - 真实路由测试现在精确断言 page、echo message/tag、query/body status 来自 `eventhub.yaml` 的中文消息；未声明 extension 的 production `userId` path 仍使用既有通用文案。
- 删除了什么
  - 删除 `requesterror.FieldErrors` 主路径和所有 production 调用；请求字段错误不再输出 legacy flat details。
  - 阶段 3 将原本集中在 `validator.go` 的 schema error 映射 helper 按职责移入新文件；只移动职责，没有删除 security、content-type、malformed body 或 fallback 行为。
- 文件移动和 package 边界变化
  - 无文件移动。
  - 阶段 3 改动限定在 `internal/http/contract`、`internal/http/router_test.go` 和本 implementation/parity 文档；没有修改 spec、generated 文件、handler、service/domain/repository、DTO、Command/Query/Result、数据库、migration、sqlc、缓存、配置或认证边界。
- 具体类型、接口和测试替身
  - 新增两个 transport 类型 `requesterror.Violation` / `Violations`，未新增接口、生产依赖或测试替身。
  - requesterror 类型不依赖 `api/openapi/gen`；generated `Violation` 只表达 OpenAPI wire model，二者由 JSON 契约对齐，避免把生成类型向错误构造边界反向扩散。
  - 阶段 3 新增具体类型 `ValidationCatalog` 及 package-private extension/rule 结构，没有新增接口、依赖或测试替身；catalog 构建后只读共享，无需请求期锁或外部缓存。
- 是否更新 Java-Go parity 记录
  - 是。parity matrix 增加阶段 3 的启动期 catalog、非法格式 fail-fast 与 native schema 消息映射记录；整体状态仍为“规则已初始化”，因为 custom rule engine、handler 收敛和 production 默认策略尚未完成。

## 3. 为什么这样设计

- 关键设计原因
  - 严格复用 design 036 和 ADR-0028：OpenAPI spec 是字段规则和字段消息的唯一契约源，`internal/http/contract` 是运行时字段校验目标入口，`internal/http/requesterror` 只负责稳定错误构造。
  - slice 能稳定保留多字段错误顺序，并显式表达 location/path/rule；flat map 无法区分 query/path，也无法承载同一字段的多条规则。
  - `Violation` 五字段在 spec 与 Go JSON tag 两侧都被测试锁定；response 仍通过 generated `ErrorResponse` 写出 `code/message/data/requestId/timestamp` envelope。
  - `ErrorResponse.data.violations` 保持可选，是因为 ErrorResponse 是全局错误模型：认证、业务和 service 防御性错误仍可能 `data=null` 或使用其它结构化 details。
  - catalog 在启动期一次性编译、请求期只读查询，既让非法扩展 fail-fast，也避免每个请求重复遍历 OpenAPI document；精确字段消息 > operation rule 消息 > 既有默认消息的优先级直接对应 design 036。
  - mapper 在完整的 `RequestError` 边界查询 catalog，而不是使用 kin-openapi `WithCustomSchemaErrorFunc`：后者只有 `SchemaError`，缺少 operation/location 上下文，也覆盖不了 parameter required sentinel 和 `$ref` sibling 消息。
- 与 Go 项目当前阶段的匹配点
  - 不引入 `go-playground/validator`，不恢复 Go embed / `SpecYAML()`，不执行 `x-validation` custom rules，不开启 kin-openapi MultiError。
  - handler 校验暂不删除；catalog 阶段只接管 OpenAPI schema/native violation 的消息来源，保证 `notBlank`、密码组合和时间范围在 custom engine 落地前仍由现有 handler 防御。
  - service/domain/repository 不依赖 generated model、contract 或 requesterror；`response` 通过 generated additional properties 适配通用 details，不识别具体 requesterror 类型。
- 与 Java 版业务语义的对齐方式
  - 外层 `COMMON-400` code 和请求体、参数、header、cookie、content-type 中文 message 保持不变。
  - handler 阶段性规则名与 Java Bean Validation / 阶段 1 OpenAPI 规则对齐，例如 `notBlank`、`minLength`、`maxLength`、`pattern`、`format`、`minimum`、`containsLetterAndDigit`、`notAfter`。
  - 阶段 3 的 negative tests 直接断言 Java parity 消息已经从 spec 进入 runtime `data.violations[].message`，不再依赖 handler 常量或 kin-openapi 英文 reason。
  - `data.violations` 是 Go spec-first 架构的刻意结构化增强；Java 注解仍是字段语义和用户消息来源，不迁移 Jakarta provider 或 DTO 注解结构。

## 4. 替代方案

- 方案 A：保留 `FieldErrors map`，在 response 序列化时临时包装成 violations。
  - 未采用。map 已经丢失 location/path/rule，遍历顺序也不稳定，response 无法可靠恢复这些语义。
- 方案 B：把 `ErrorResponse.data` 全局收窄为必填 violations。
  - 未采用。会破坏 `data=null` 和 service 现有通用 details，迫使非字段类错误伪装成字段错误，扩大阶段范围。
- 方案 C：让 `requesterror.Violation` 直接别名 generated `openapigen.Violation`。
  - 未采用。会让错误构造包反向依赖 generated contract model，并把 enum/生成器细节扩散到运行时映射；当前通过同一 JSON wire contract 对齐即可。
- 方案 D：本阶段同时实现 validation catalog/custom rules 并删除 handler 校验。
  - 未采用。阶段 2 只演进错误结构；规则执行和 handler 收敛属于后续阶段，提前处理会跨越冻结范围。
- 方案 E：引入 `go-playground/validator`。
  - 未采用。会形成 OpenAPI 与 Go tag 双规则源，违背 ADR-0028。
- 方案 F：使用 kin-openapi `Options.WithCustomSchemaErrorFunc` 直接重写 `SchemaError.Error()`。
  - 未采用。回调缺少 operation/location/parameter 上下文，无法可靠形成 catalog key，也无法处理 parameter required；在统一 `RequestError` mapper 中查询信息更完整。
- 方案 G：每个请求直接遍历 OpenAPI schema 和 `x-validation`。
  - 未采用。重复解析增加热路径成本，而且非法扩展只能在流量到达时暴露；启动期编译的只读 catalog 更符合 fail-fast 和并发共享边界。
- 方案 H：让 generated chi binder 的 `ErrorHandlerFunc` 也注入 catalog。
  - 未采用。contract route-level middleware 在 validation 开启时先于 generated binder 拦截 query/path schema failure；binder 仅是关闭/未接 contract 时的语法绑定兜底，缺少可靠 schema rule 上下文，跨 package 注入会扩大阶段范围。

## 5. 测试与验证

- `gofmt`：阶段 3 变更 Go 文件已格式化。
- `go test ./internal/http/contract -count=1`：通过；覆盖启动期 catalog、extension fail-fast、body/query/path 原生规则消息、operation fallback、header/cookie 既有 fallback 与 custom rules 不执行。
- `go test ./internal/http ./api/openapi -count=1`：通过；真实 spec 的 page、echo message/tag、query/body status 消息已从 `x-validation.messages` 命中，OpenAPI policy 继续通过。
- `make openapi-check`：通过；kin-openapi validate、models/server 两次生成及 generated diff 检查均通过，本阶段没有 spec/generated 漂移。
- `go test ./...`：通过。
- `go vet ./...`：通过。
- `make lint`：通过，golangci-lint 报告 `0 issues`。
- `git diff --check`：通过。
- Java-Go parity 验证：阶段 1 policy test 继续精确锁定 Java DTO 消息；阶段 3 contract/router negative tests 证明这些消息已进入 runtime `data.violations`，同时 synthetic path 测试证明同一索引模型覆盖 path。
- 不适用项：本阶段未修改 SQL、schema、migration 或 sqlc 配置，因此不运行 `sqlc generate` 和 migration 测试；未改 API schema，不需要 `openapi-breaking-check`。

## 6. 已知限制

- `internal/http/contract` 已编译 `x-validation` 消息和 custom rule 元数据，但本阶段只让 OpenAPI schema/native violation 使用 catalog；`notBlank`、`containsLetterAndDigit`、`notAfter` 尚未在 contract 中执行，仍由阶段性 handler 校验。
- handler 私有校验仍保留，尚未收敛为纯 generated request -> Command/Query 映射和 normalize。
- contract gate 当前未开启 MultiError，一次 kin-openapi schema failure 通常只返回一条 violation；`Violations` 已支持有序多项，后续 custom rule 阶段再统一聚合策略。
- 字段未声明 `x-validation`、rule 未进入 catalog 或 operation fallback 未命中时，仍使用既有 parameter contract 文案或 kin-openapi schema reason；阶段 3 不把“缺少 extension”升级为启动失败，项目 spec 的字段覆盖完整性继续由 policy test 约束。
- 字段一旦声明 `x-validation`，当前 policy 和 catalog compiler 都要求其全部 native rule 在字段 `messages` 中完整声明；operation fallback 用于没有字段 extension 的通用 rule message，不替代字段声明完整性。
- kin-openapi v0.131.0 默认未注册 `format: email` 的运行时 validator；当前非法邮箱仍由保留的 auth handler 校验拒绝。后续删除 handler 校验前必须在 contract 边界补齐 email format 执行并增加 negative test。
- 缺失 body 与非法 JSON 继续复用外层 `请求体格式不合法` 和既有 details message，但 violation rule 已分别固定为 `required` 与 `malformed`；后续若拆分用户文案，必须同步调整 OpenAPI 和测试。
- generated chi binder 的 fallback 错误处理不读取 catalog；validation 开启时 query/path schema failure 已由更外层 contract gate 拦截，关闭 validation 或未接 contract 时 binder 继续只负责语法绑定错误。
- production `userId` path 当前没有 `x-validation.messages`，因此仍使用通用 path contract 文案；本阶段只用 synthetic spec 证明 path catalog 映射能力，没有扩大阶段 1 spec/policy 范围。
- `ErrorResponse.data` 仍保留 additional properties，用于兼容非字段类 `AppError.Details`；只有 requesterror 字段校验构造器保证输出 `data.violations`。
- production / Docker 的 request validation 默认策略尚未调整。
- 两个 status 字段继续把 `x-validation` 放在 `$ref` sibling；当前 kin-openapi、oapi-codegen 和 Redocly 工具链可用，但对外交换 spec 时仍需关注严格 OpenAPI 3.0 工具的互操作性。
- 结构债务：OpenAPI policy helper 仍集中在单一测试文件；规则种类继续增长时可按主题拆分，但不改变同 package policy 边界。

## 7. 对后续版本的影响

- 对简历可用版的价值
  - 字段规则、稳定消息、错误定位和统一 envelope 已具备可执行 OpenAPI 与 Go 测试证据，前端可以按 `data.violations` 稳定关联表单字段。
- 对微服务 / 云原生演进的影响
  - violations 是 transport contract，不依赖单体 service/domain；未来拆分服务时可复用同一错误模型并继续由 OpenAPI 治理。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续 custom rule 阶段直接复用已编译的 field/cross-field metadata 和 `requesterror.Violations`，无需再次改变 response wire shape 或重新设计消息来源。
  - 阶段 4 需要执行 `notBlank`、`containsLetterAndDigit`、`notAfter`；阶段 5 删除 handler 字段校验前必须补齐 email format runtime validator，并用 contract/router negative tests 证明所有 Java parity 规则已接管。
  - 删除 handler 校验前，必须由 contract/router negative tests 证明 spec 标准规则、catalog 消息和 custom rules 已完整接管。
  - 新增 HTTP 输入 location 或 violation 字段时必须先修改 OpenAPI schema/policy，再同步 requesterror 与 generated model。
  - 本阶段不影响 migration、sqlc、缓存、事务、认证/授权或 service/domain/repository 依赖方向。
