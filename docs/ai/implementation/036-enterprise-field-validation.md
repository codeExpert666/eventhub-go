# Enterprise Field Validation

## 1. 本次改动解决了什么问题

- 阶段 1 已把当前 system/auth/user 请求字段的标准规则消息、`notBlank`、`containsLetterAndDigit` 和管理员时间范围 `notAfter` 声明收敛到 `api/openapi/eventhub.yaml`，并由 OpenAPI policy test 固化声明格式和 Java parity 消息。
- 阶段 2 将 HTTP 字段校验错误从松散的 `data[field] = message` map 演进为稳定的 `data.violations` 数组；每一项统一包含 `location`、`field`、`path`、`rule`、`message`。
- contract gate、generated chi 参数绑定、strict body 解码和阶段性保留的 handler 校验现在通过同一 `internal/http/requesterror` 结构输出错误，不再因入口不同而产生不同 details 形状。
- OpenAPI `ErrorResponse.data` 已能机器表达 violations，同时继续兼容 `data=null` 和非字段类 `AppError.Details`，没有把全局错误模型误收窄为“所有错误都必须有 violations”。

## 2. 改动内容

- 新增了什么
  - 在 `internal/http/requesterror` 新增 `Violation`、`Violations` 和 body/query/path/header/cookie location 常量。
  - 在 `api/openapi/eventhub.yaml` 新增 `Violation` schema；五个字段全部 required，`additionalProperties: false`。
  - 在 `ErrorResponse.data` 增加可选 `violations` 数组，并保留 `nullable: true`、`additionalProperties: true`。
  - 在 OpenAPI policy test 增加 ErrorResponse violations contract 检查，约束数组 item、五个必填 string 字段、nullable 和通用 details 兼容边界。
  - 增加 response JSON、contract header/cookie、generated query/path binder、handler 保留路径及 router contract gate 的 violations 测试。
- 修改了什么
  - `InvalidBody`、`InvalidParameters`、`InvalidHeaders`、`InvalidCookies` 改为接收有序 `Violations`；`MissingBody`、`MalformedBody`、`UnsupportedContentType` 也输出相同结构，并用 `required` / `malformed` 区分缺失 body 与非法 JSON。
  - `internal/http/contract` 从 OpenAPI parameter location、`SchemaError.SchemaField` 和 JSON pointer 构造 violation；body path 使用点分完整路径，字段名保持当前顶层字段语义；显式空参数使用 `rule=allowEmptyValue`。
  - generated chi 参数错误映射现在接收 `*http.Request`，通过 chi route context 区分 query/path，并把反序列化失败稳定标记为 `rule=type`；required query/header 使用与 rule 一致的“不能为空”消息。
  - auth/system/user handler 手写校验按阶段要求继续保留，但错误聚合从无序 map 改为有序 slice，并填充与阶段 1 spec 对应的 rule；没有删除或弱化原有合法性判断。
  - `response.detailsData` 适配 generated `ErrorResponse_Data`，通过其 additional properties 能力透传通用 `apperror.Details`，不依赖 `requesterror`。
  - 重新生成 `api/openapi/gen/models.gen.go`；新增 generated `Violation`、`ViolationLocation`、`ErrorResponse_Data` 及 additional properties JSON 支持。`server.gen.go` 重新生成后无内容漂移。
- 删除了什么
  - 删除 `requesterror.FieldErrors` 主路径和所有 production 调用；请求字段错误不再输出 legacy flat details。
- 文件移动和 package 边界变化
  - 无文件移动。
  - 改动限定在 OpenAPI、`internal/http/{requesterror,contract,response}`、generated route 适配和当前 handler 校验文件；service/domain/repository、DTO、Command/Query/Result、数据库、migration、sqlc、缓存和认证边界均未改变。
- 具体类型、接口和测试替身
  - 新增两个 transport 类型 `requesterror.Violation` / `Violations`，未新增接口、生产依赖或测试替身。
  - requesterror 类型不依赖 `api/openapi/gen`；generated `Violation` 只表达 OpenAPI wire model，二者由 JSON 契约对齐，避免把生成类型向错误构造边界反向扩散。
- 是否更新 Java-Go parity 记录
  - 是。parity matrix 记录阶段 1 已初始化规则和消息、阶段 2 已落地统一 violations 错误结构；整体状态仍为“规则已初始化”，因为 runtime catalog/custom rule engine 和 handler 收敛尚未完成。

## 3. 为什么这样设计

- 关键设计原因
  - 严格复用 design 036 和 ADR-0028：OpenAPI spec 是字段规则和字段消息的唯一契约源，`internal/http/contract` 是运行时字段校验目标入口，`internal/http/requesterror` 只负责稳定错误构造。
  - slice 能稳定保留多字段错误顺序，并显式表达 location/path/rule；flat map 无法区分 query/path，也无法承载同一字段的多条规则。
  - `Violation` 五字段在 spec 与 Go JSON tag 两侧都被测试锁定；response 仍通过 generated `ErrorResponse` 写出 `code/message/data/requestId/timestamp` envelope。
  - `ErrorResponse.data.violations` 保持可选，是因为 ErrorResponse 是全局错误模型：认证、业务和 service 防御性错误仍可能 `data=null` 或使用其它结构化 details。
- 与 Go 项目当前阶段的匹配点
  - 不引入 `go-playground/validator`，不恢复 Go embed / `SpecYAML()`，不实现 `x-validation` catalog/custom rules，不开启 kin-openapi MultiError。
  - handler 校验暂不删除，只把其错误输出迁移到目标 details 结构，保证本阶段不因运行时规则尚未接管而削弱校验。
  - service/domain/repository 不依赖 generated model、contract 或 requesterror；`response` 通过 generated additional properties 适配通用 details，不识别具体 requesterror 类型。
- 与 Java 版业务语义的对齐方式
  - 外层 `COMMON-400` code 和请求体、参数、header、cookie、content-type 中文 message 保持不变。
  - handler 阶段性规则名与 Java Bean Validation / 阶段 1 OpenAPI 规则对齐，例如 `notBlank`、`minLength`、`maxLength`、`pattern`、`format`、`minimum`、`containsLetterAndDigit`、`notAfter`。
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

## 5. 测试与验证

- `gofmt`：所有变更 Go 文件已格式化。
- `go test ./internal/http/requesterror ./internal/http/contract ./internal/http -count=1`：通过。
- `go test ./api/openapi -count=1`：通过。
- `go test ./internal/http/response ./internal/http/handler/auth ./internal/http/handler/system ./internal/http/handler/user -count=1`：通过。
- `make openapi-check`：通过。由于该目标会把生成物与 Git index 比较，而阶段 2 合法修改了 generated model，本次使用临时 `GIT_INDEX_FILE` 预装当前 generated 文件运行同一 target；kin-openapi validate、两次 oapi-codegen 生成及 generated diff 检查均通过，真实 index 未改变。
- `go test ./...`：通过。
- `go vet ./...`：通过。
- `git diff --check`：通过。
- Java-Go parity 验证：阶段 1 policy test 继续锁定 Java DTO 稳定消息；阶段 2 requesterror/response/router/contract 测试锁定同一 `COMMON-400` 语义和新的 violations wire contract。
- 不适用项：未修改 SQL、schema、migration 或 sqlc 配置，因此不运行 `sqlc generate` 和 migration 测试；本阶段未要求 `make openapi-breaking-check`、`make openapi-lint` 或 lint 全量门禁。

## 6. 已知限制

- `internal/http/contract` 尚未编译或执行 `x-validation`；`notBlank`、`containsLetterAndDigit`、`notAfter` 仍由阶段性 handler 校验执行。
- handler 私有校验仍保留，尚未收敛为纯 generated request -> Command/Query 映射和 normalize。
- contract gate 当前未开启 MultiError，一次 kin-openapi schema failure 通常只返回一条 violation；`Violations` 已支持有序多项，handler 多字段失败可返回多项，后续 catalog/custom rule 阶段再统一聚合策略。
- 阶段 3 catalog 落地前，kin-openapi schema reason 仍可能是第三方英文消息；阶段 2 只稳定结构、定位和 rule，不提前实现消息映射。
- 缺失 body 与非法 JSON 继续复用外层 `请求体格式不合法` 和既有 details message，但 violation rule 已分别固定为 `required` 与 `malformed`；后续若拆分用户文案，必须同步调整 OpenAPI 和测试。
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
  - 后续 catalog/custom rule 阶段直接生成 `requesterror.Violations`，无需再次改变 response wire shape。
  - 删除 handler 校验前，必须由 contract/router negative tests 证明 spec 标准规则、catalog 消息和 custom rules 已完整接管。
  - 新增 HTTP 输入 location 或 violation 字段时必须先修改 OpenAPI schema/policy，再同步 requesterror 与 generated model。
  - 本阶段不影响 migration、sqlc、缓存、事务、认证/授权或 service/domain/repository 依赖方向。
