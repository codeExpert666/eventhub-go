# Enterprise Field Validation

## 1. 本次改动解决了什么问题

- 阶段 1 已把当前 system/auth/user 请求字段的标准规则消息、`notBlank`、`containsLetterAndDigit` 和管理员时间范围 `notAfter` 收敛到 `api/openapi/eventhub.yaml`，并由 OpenAPI policy test 固化声明格式与 Java parity 消息。
- 阶段 2 已把 HTTP 字段校验错误统一为 `data.violations[{location,field,path,rule,message}]`；contract gate、generated binder 和阶段性保留的 handler 校验不再输出不同 details 形状。
- 阶段 3 已在启动期编译只读 `ValidationCatalog`，严格校验 `x-validation` 格式，并让 body/query/path 的 OpenAPI 原生 schema violation 优先使用 spec 中声明的稳定消息。
- 阶段 4 解决了 catalog 只编译 custom rule 元数据、contract gate 尚不执行这些规则的问题：`notBlank`、`containsLetterAndDigit`、`notAfter` 现已在 OpenAPI schema validation 成功后执行，失败统一生成有序 violations，并在进入 handler 前返回。
- 阶段 4 保持受保护 operation 的安全顺序：kin-openapi 仍先执行 security requirement，再执行 schema 与 custom rules；未认证的管理员请求不会先暴露字段错误。

## 2. 改动内容

- 新增了什么
  - 阶段 2 新增 `requesterror.Violation` / `Violations`、OpenAPI `Violation` schema 和 generated wire model；五字段均必填，`ErrorResponse.data.violations` 保持可选并兼容非字段类 details。
  - 阶段 3 新增 `internal/http/contract/{validation_catalog,validation_extension,validation_violation,validation_error_mapper}.go`，分别承载启动期索引、严格 extension parser、violation 定位和错误映射。
  - 阶段 4 新增 `internal/http/contract/validation_custom_rules.go`，直接消费阶段 3 编译的 field/cross-field metadata，不在请求期重读或遍历 OpenAPI document。
  - custom engine 按 `operationId` 取规则；字段先按 location 与 field 稳定排序，字段内保持 catalog 声明顺序，cross-field rules 保持 spec 数组顺序，可在一个响应中有序聚合多条 violations。
  - 新增 custom rule 单元测试和真实 spec/路由集成测试，覆盖 register、login、refresh、admin users、echo 的规则失败消息、schema 优先级、break-glass、body 回放和安全优先级。
- 修改了什么
  - `RequestValidator.Middleware` 在 `openapi3filter.ValidateRequest` 成功后执行 custom engine；schema/security 失败立即返回，不与 custom violation 混合。
  - `WithRequestValidation(false)` 仍只执行 security bridge，不执行 schema 或 custom rules；非法 `x-validation` 仍会在 `NewRequestValidator` 编译 catalog 时启动失败。
  - body custom rule 从 `GetBody` 副本读取；无副本时执行 read-and-restore。query/path/header/cookie 读取原始首值，custom 校验不修改请求值，strict handler 仍能完整读取 body。
  - `notBlank` 对 string 使用 `strings.TrimSpace`；`containsLetterAndDigit` 要求原始 password 同时包含 ASCII 字母和数字；`notAfter` 只在左右 query 值都存在且可按 `2006-01-02T15:04:05` 解析时比较，left 晚于 right 才失败。
  - custom violations 按 location 映射到 `InvalidBody`、`InvalidParameters`、`InvalidHeaders` 或 `InvalidCookies`；读取 custom 输入发生与 schema 成功状态不一致的内部异常时返回 `COMMON-500`，不泄露原始值或底层错误。
  - auth/system/user handler 手写字段校验按阶段要求继续保留；本阶段没有删除或弱化 handler 防御路径。
- 删除了什么
  - 阶段 2 已删除 `requesterror.FieldErrors` production 主路径；请求字段错误不再输出 legacy flat details。
  - 阶段 4 没有删除 production handler 校验、配置或兼容路径。
- 文件移动和 package 边界变化
  - 无文件移动。
  - 阶段 4 改动限定在 `internal/http/contract`、`internal/http` 集成测试和本 implementation/parity 文档；未修改 spec/generated、handler、service/domain/repository、DTO、Command/Query/Result、数据库、migration、sqlc、缓存、配置、Docker 或认证实现。
- 具体类型、接口和测试替身
  - 阶段 4 只新增 package-private `customRuleRequestValues` 和 rule helper；未新增导出类型、接口、第三方依赖或 production test double。
  - `ValidationCatalog` 继续在启动后只读共享；每个请求的 body map、query 值和 violations slice 都是请求内局部状态，不需要锁或外部缓存。
- 是否更新 Java-Go parity 记录
  - 是。parity matrix 已记录阶段 4 的 runtime custom engine、五类真实 API 消息和认证优先级证据。
  - 状态从“规则已初始化”调整为“部分对齐”：核心运行时语义已经落地，但 handler 收敛、email format runtime、production 默认开启等目标态仍未完成。

## 3. 为什么这样设计

- 关键设计原因
  - 严格复用 design 036 和 ADR-0028：OpenAPI spec 是字段规则和消息的唯一契约源，`internal/http/contract` 是运行时字段校验目标入口，`internal/http/requesterror` 只负责稳定错误构造。
  - custom engine 复用启动期 catalog，避免形成 Go struct tag、handler 常量或请求期 spec 解析等第二规则源。
  - 顺序固定为 security -> OpenAPI schema -> custom rules -> handler。这样既不重复执行认证 middleware，也避免 custom rule 覆盖更基础的类型、required、长度或格式错误。
  - custom rule 读取原始 HTTP 输入而不 normalize；trim/lowercase 仍在校验成功后的 handler mapper/service 防御边界，password、refresh token 和 echo message 不被 contract 改写。
  - body 通过副本解析而不是消费 `request.Body`，保持 generated strict handler 的既有 body replay 契约。
- 与 Go 项目当前阶段的匹配点
  - 不引入 `go-playground/validator`，不恢复 Go embed / `SpecYAML()`，不改变 spec/generated，也不开启 kin-openapi MultiError。
  - 初始 engine 只执行已冻结的三类 transport rule，不扩展为通用业务规则引擎；账号唯一、用户状态、token/session 语义仍属于 service/repository。
  - handler 校验本阶段继续保留；contract gate 开启时 invalid request 已在 handler 前拦截，handler 清理属于后续阶段。
  - service/domain/repository 不依赖 generated model、contract 或 requesterror；依赖方向未变化。
- 与 Java 版业务语义的对齐方式
  - `notBlank` 对齐 Java `@NotBlank` 的空白拒绝意图；Go 使用 `strings.TrimSpace`，对 Unicode 空白的覆盖略宽，是 Go 自然写法下的刻意小差异。
  - `containsLetterAndDigit` 对齐 Java password 正则中的 ASCII `[A-Za-z]` 与数字组合语义，不 trim、不回显 password。
  - `notAfter` 对齐 Java `@AssertTrue`：任一边界缺失时通过、相等通过、from 晚于 to 时失败，violation 指向 left 字段并使用 spec 消息。
  - 外层 `COMMON-400`、请求体/请求参数 envelope message 和 violations 五字段保持不变；Go 只用 spec-first runtime 实现替代 Jakarta provider。

## 4. 替代方案

- 方案 A：引入 `go-playground/validator`，在 generated model 外再声明 Go tag。
  - 未采用。会形成 OpenAPI 与 Go tag 双规则源，违背 ADR-0028。
- 方案 B：在 OpenAPI schema validation 前执行 custom rules。
  - 未采用。类型、required、长度、pattern 等基础错误应先由 schema 拦截；custom 只补 OpenAPI 原生规则无法稳定表达的语义。
- 方案 C：直接复用 auth/system/user handler 的私有校验 helper。
  - 未采用。会让唯一运行时入口反向依赖业务 handler，并继续保留规则与消息双写。
- 方案 D：每个请求遍历 OpenAPI document 并重新解析 `x-validation`。
  - 未采用。增加热路径成本，且非法 extension 只能在流量到达时暴露；阶段 3 的启动期 catalog 已提供只读元数据。
- 方案 E：把无法解析的管理员日期直接报告为 `notAfter`。
  - 未采用。`notAfter` 只表达范围顺序，日期有效性属于 format/pattern 边界；误用会返回错误规则和消息。
- 方案 F：阶段 4 同时删除 handler 校验并调整 production 默认配置。
  - 未采用。用户冻结的本阶段只实现 custom rules；email format 和真实日历日期等接管证据尚未补齐，提前删除会扩大范围并留下校验缺口。

## 5. 测试与验证

- `gofmt`：阶段 4 变更 Go 文件已格式化。
- `go test ./internal/http/contract ./internal/http -count=1`：通过；覆盖三类 custom rule、稳定聚合、合法/相等/缺边界、schema 优先、break-glass、body replay、真实 spec 五类 endpoint 消息、contract-only downstream sentinel 和未认证优先 `AUTH-401`。
- `go test ./...`：通过。
- `go vet ./...`：通过。
- `make lint`：通过，golangci-lint 报告 `0 issues`。
- `make openapi-check`：通过；kin-openapi validate、models/server 生成及 generated diff 检查均通过，确认本阶段没有 spec/generated 漂移。
- `git diff --check`：通过。
- Java-Go parity 验证：policy test 继续锁定 Java DTO 消息；custom unit tests 锁定 Java 规则语义；真实 spec 集成测试锁定 register/login/refresh/admin users/echo 的最终 violations；security 集成测试证明受保护 operation 未认证时先返回 `AUTH-401`。
- 不适用项：本阶段未修改 SQL、schema、migration 或 sqlc 配置，因此不运行 `sqlc generate` 和 migration 测试；未修改 OpenAPI schema，不需要 `openapi-breaking-check`。

## 6. 已知限制

- handler 私有字段校验仍保留，尚未收敛为纯 generated request -> Command/Query 映射与 normalize；本阶段只保证 request contract validation 开启时 invalid request 在 handler 前被拦截。
- production / Docker 的 request validation 默认策略尚未调整；显式关闭 validation 时 custom rules 也会跳过，当前仍依赖保留的 handler 防御。
- kin-openapi v0.131.0 默认未注册 `format: email` 的运行时 validator；删除 auth handler 校验前仍需在 contract 边界补齐 email format 执行和 negative test。
- 管理员时间 schema 的 pattern 只验证外形，不能识别 2 月 30 日等非法日历日期；`notAfter` 遇到解析失败会跳过，目前仍由 handler `time.Parse` 兜底。
- native schema validation 未开启 MultiError，通常只返回首条 schema violation；custom engine 已支持按稳定顺序聚合多条 custom violations，但不与 schema failure 混合。
- 初始 engine 只承诺 string `notBlank`、ASCII password 组合和 query local-date-time `notAfter`；cross-field extension 当前按冻结 spec 解释为 query 字段，不承载 service/domain 业务规则。
- 当前 catalog parser 依赖 spec/policy 保证 custom rule 挂载在匹配的 string schema；未来新增 rule 类型时需同步增加 schema 类型约束和 fail-closed evaluator，不应让未知组合静默通过。
- 字段未声明 `x-validation` 或消息未命中 catalog 时仍使用既有 contract fallback；字段覆盖完整性继续由 OpenAPI policy test 约束。
- generated chi binder 的 fallback 错误处理不读取 catalog；validation 开启时由更外层 contract gate 拦截，关闭或未接 contract 时 binder 继续只负责语法绑定错误。
- `ErrorResponse.data` 继续保留 additional properties，以兼容非字段类 `AppError.Details`；只有 requesterror 字段校验构造器保证输出 `data.violations`。

## 7. 对后续版本的影响

- 对简历可用版的价值
  - 字段规则、稳定消息、custom runtime、错误定位、安全顺序和统一 envelope 已有可执行 OpenAPI 与 Go 测试证据，前端可按 `data.violations` 稳定关联表单字段。
- 对微服务 / 云原生演进的影响
  - catalog 启动期编译、请求期只读且 violations 不依赖单体 service/domain；未来拆分服务时可复用同一 transport contract。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续阶段应先补 email format 与真实日期有效性，再删除 handler 字段校验并保留 mapper/normalize。
  - production / Docker 默认开启 request validation 仍需独立阶段实施并保留 break-glass 说明。
  - 新增 custom rule 必须先扩展 spec/policy/catalog parser，再实现 engine 与 parity tests，不能直接在 handler 或 service 增加 transport 校验。
  - 本阶段不影响 migration、sqlc、缓存、事务、JWT claim 或 service/domain/repository 依赖方向。
