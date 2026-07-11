# Enterprise Field Validation

## 1. 本次改动解决了什么问题

- 阶段 1 已把当前 system/auth/user 请求字段的标准规则消息、`notBlank`、`containsLetterAndDigit` 和管理员时间范围 `notAfter` 收敛到 `api/openapi/eventhub.yaml`，并由 OpenAPI policy test 固化声明格式与 Java parity 消息。
- 阶段 2 已把 HTTP 字段校验错误统一为 `data.violations[{location,field,path,rule,message}]`；contract gate、generated binder 和阶段性保留的 handler 校验不再输出不同 details 形状。
- 阶段 3 已在启动期编译只读 `ValidationCatalog`，严格校验 `x-validation` 格式，并让 body/query/path 的 OpenAPI 原生 schema violation 优先使用 spec 中声明的稳定消息。
- 阶段 4 解决了 catalog 只编译 custom rule 元数据、contract gate 尚不执行这些规则的问题：`notBlank`、`containsLetterAndDigit`、`notAfter` 现已在 OpenAPI schema validation 成功后执行，失败统一生成有序 violations，并在进入 handler 前返回。
- 阶段 4 保持受保护 operation 的安全顺序：kin-openapi 仍先执行 security requirement，再执行 schema 与 custom rules；未认证的管理员请求不会先暴露字段错误。
- 阶段 5 删除了 auth/user/system handler 中由 OpenAPI contract gate 接管的字段合法性校验；handler parser 现在只保留 nil body 防御、normalize、必要类型转换和 generated request 到 service Command/Query 的映射。
- 阶段 5 同时闭环了删除 handler 校验前发现的两个 contract 缺口：注册请求的 `format: email` 现在由 contract runtime 执行；管理员时间字段在 spec 中显式声明 `x-validation.rules: localDateTime`，由 custom engine 在 `notAfter` 之前拒绝真实日历日期与时钟无效值。
- 阶段 5 新增 AST 与行为测试，防止 auth/user/system handler 再次通过 `requesterror.InvalidBody`、`InvalidParameters` 或私有正则/mail parser 建立第二字段规则源。

## 2. 改动内容

- 新增了什么
  - 阶段 2 新增 `requesterror.Violation` / `Violations`、OpenAPI `Violation` schema 和 generated wire model；五字段均必填，`ErrorResponse.data.violations` 保持可选并兼容非字段类 details。
  - 阶段 3 新增 `internal/http/contract/{validation_catalog,validation_extension,validation_violation,validation_error_mapper}.go`，分别承载启动期索引、严格 extension parser、violation 定位和错误映射。
  - 阶段 4 新增 `internal/http/contract/validation_custom_rules.go`，直接消费阶段 3 编译的 field/cross-field metadata，不在请求期重读或遍历 OpenAPI document。
  - custom engine 按 `operationId` 取规则；字段先按 location 与 field 稳定排序，字段内保持 catalog 声明顺序，cross-field rules 保持 spec 数组顺序，可在一个响应中有序聚合多条 violations。
  - 新增 custom rule 单元测试和真实 spec/路由集成测试，覆盖 register、login、refresh、admin users、echo 的规则失败消息、schema 优先级、break-glass、body 回放和安全优先级。
  - 阶段 5 新增 `validation_native_formats.go`，在 `NewRequestValidator` 初始化时以 `sync.Once` 向 kin-openapi format registry 注册 contract-owned `email` evaluator；它使用标准库 `net/mail` 精确 address 解析并按 Java UTF-16 code unit 限制 64 长度 local part，规则名和用户消息仍来自 OpenAPI `format: email` 与 `x-validation.messages.format`。
  - 阶段 5 在既有 `x-validation.rules` 机制中新增 `localDateTime` evaluator，使用秒精度 layout 校验真实日期和时钟值；四个字段分别在 spec rule item 中声明名称与消息。
  - 阶段 5 新增 `handler_validation_boundary_test.go`，AST 扫描 auth/user/system production handler：`requesterror` 只允许用于 nil body 的 `MalformedBody`，并禁止重新引入 `net/mail`、`regexp` 字段校验依赖。
  - 阶段 5 新增 mapper 行为测试，证明即使直接构造 contract-invalid generated model，handler parser 也只 normalize/映射而不返回字段校验错误。
- 修改了什么
  - `RequestValidator.Middleware` 在 `openapi3filter.ValidateRequest` 成功后执行 custom engine；schema/security 失败立即返回，不与 custom violation 混合。
  - `WithRequestValidation(false)` 仍只执行 security bridge，不执行 schema 或 custom rules；非法 `x-validation` 仍会在 `NewRequestValidator` 编译 catalog 时启动失败。
  - body custom rule 从 `GetBody` 副本读取；无副本时执行 read-and-restore。query/path/header/cookie 读取原始首值，custom 校验不修改请求值，strict handler 仍能完整读取 body。
  - `notBlank` 对 string 使用 `strings.TrimSpace`；`containsLetterAndDigit` 要求原始 password 同时包含 ASCII 字母和数字；`notAfter` 只在左右 query 值都存在且可按 `2006-01-02T15:04:05` 解析时比较，left 晚于 right 才失败。
  - custom violations 按 location 映射到 `InvalidBody`、`InvalidParameters`、`InvalidHeaders` 或 `InvalidCookies`；读取 custom 输入发生与 schema 成功状态不一致的内部异常时返回 `COMMON-500`，不泄露原始值或底层错误。
  - 阶段 5 在四个管理员时间 query schema 上新增 `x-validation.rules: localDateTime`，外形合法但真实日期/时钟无效时由 custom engine 返回 `rule=localDateTime`；`notAfter` 继续只负责已通过字段规则的范围顺序比较。
  - `userId` path minimum 的 required/format/minimum 消息也补入 spec，删除 handler userId 校验后仍由 catalog 返回 spec 声明的稳定提示。
  - auth parser 只 trim username/email，password 和 refresh token 保持原样；system echo message/tag 保持原样；user parser 保留分页默认值、筛选 trim、local date-time 到 `*time.Time` 的必要类型转换和 status/userId 映射。
  - 五个 nil body 防御统一改为 `requesterror.MalformedBody()`；handler 不再调用 `MissingBody`、`InvalidBody` 或 `InvalidParameters`。
  - service 层业务兜底未删除：分页边界、状态转换、userId、账号唯一、用户状态、refresh token/session 轮换等规则仍在原有 service/repository 边界执行。
- 删除了什么
  - 阶段 2 已删除 `requesterror.FieldErrors` production 主路径；请求字段错误不再输出 legacy flat details。
  - 阶段 5 删除 auth handler 的 username/email/password/login/refresh 私有字段规则、`net/mail`、username regexp、password composition helper 和 violation builder。
  - 阶段 5 删除 system handler 的 echo whitespace guard，以及 user handler 的筛选长度、时间范围、userId minimum 和 query violation builder。
- 文件移动和 package 边界变化
  - 无文件移动。
  - 阶段 5 改动限定在 `api/openapi/eventhub.yaml` 与 policy test、`internal/http/contract`、`internal/http/handler/{auth,user,system}`、HTTP/handler 测试和本 implementation/parity 文档；OpenAPI generated 文件经重生成确认无漂移，未修改 service/domain/repository、DTO、Command/Query/Result、数据库、migration、sqlc、缓存、配置、Docker 或认证实现。
  - `validation.go` / `admin_validation.go` 文件名为控制阶段 diff 暂时保留，但内容已收敛为 parser/mapper；没有新增跨 package 依赖或结构性分层债务。
- 具体类型、接口和测试替身
  - 阶段 4 只新增 package-private `customRuleRequestValues` 和 rule helper；未新增导出类型、接口、第三方依赖或 production test double。
  - `ValidationCatalog` 继续在启动后只读共享；每个请求的 body map、query 值和 violations slice 都是请求内局部状态，不需要锁或外部缓存。
  - 阶段 5 只新增 package-private email format 注册/helper 与 `localDateTime` custom evaluator；未新增导出 API、接口、第三方依赖或 test double。email 通过现有 kin-openapi registry 接入且只依赖 Go 标准库，`sync.Once` 只保护启动期全局 registry 写入。
- 是否更新 Java-Go parity 记录
  - 是。parity matrix 已记录阶段 5 的 contract email/local date-time 接管、handler 纯映射边界和 AST/行为测试证据，并同步修正 system/auth 行中的历史描述。
  - 整体状态继续为“部分对齐”：字段 runtime 与 handler 收敛目标已完成，production / Docker 默认开启 request validation 仍是后续阶段。

## 3. 为什么这样设计

- 关键设计原因
  - 严格复用 design 036 和 ADR-0028：OpenAPI spec 是字段规则和消息的唯一契约源，`internal/http/contract` 是运行时字段校验目标入口，`internal/http/requesterror` 只负责稳定错误构造。
  - custom engine 复用启动期 catalog，避免形成 Go struct tag、handler 常量或请求期 spec 解析等第二规则源。
  - 顺序固定为 security -> OpenAPI schema -> custom rules -> handler。这样既不重复执行认证 middleware，也避免 custom rule 覆盖更基础的类型、required、长度或格式错误。
  - custom rule 读取原始 HTTP 输入而不 normalize；trim/lowercase 仍在校验成功后的 handler mapper/service 防御边界，password、refresh token 和 echo message 不被 contract 改写。
  - body 通过副本解析而不是消费 `request.Body`，保持 generated strict handler 的既有 body replay 契约。
  - 删除 handler 校验前先在 spec 声明并补齐 email 与真实 local date-time 的 contract 执行证据，避免规则从 handler 删除后变成未执行或隐式 Go 规则；email 复用 OpenAPI `format`，日期复用已冻结的 `x-validation.rules` 扩展形态，只把 evaluator 白名单增加到当前 parity 必需范围。
  - handler 的 invalid generated model 行为测试刻意断言“仍映射”，AST policy 则限制 `requesterror` 只用于 `MalformedBody`，分别从行为和依赖两侧防止规则回流。
  - user query 必须把 generated string 转为 service 的 `*time.Time`；正常 HTTP 路径已由 contract 保证可解析，直接调用 parser 时的解析失败作为 contract invariant 收敛为 `COMMON-500`，不伪装成 handler 字段校验。
- 与 Go 项目当前阶段的匹配点
  - 不引入 `go-playground/validator`，不恢复 Go embed / `SpecYAML()`，不改变 generated model/strict server 形状，也不开启 kin-openapi MultiError。
  - engine 只执行当前 parity 必需的 `notBlank`、`containsLetterAndDigit`、`localDateTime` 与 `notAfter`，不扩展为通用业务规则引擎；账号唯一、用户状态、token/session 语义仍属于 service/repository。
  - handler 字段校验已在阶段 5 删除；parser 继续依赖 generated model、service contract、normalize 工具和 nil body 错误构造，不依赖 contract catalog。
  - service/domain/repository 不依赖 generated model、contract 或 requesterror；依赖方向未变化。
- 与 Java 版业务语义的对齐方式
  - `notBlank` 对齐 Java `@NotBlank` 的空白拒绝意图；Go 使用 `strings.TrimSpace`，对 Unicode 空白的覆盖略宽，是 Go 自然写法下的刻意小差异。
  - `containsLetterAndDigit` 对齐 Java password 正则中的 ASCII `[A-Za-z]` 与数字组合语义，不 trim、不回显 password。
  - `format: email` 对齐 Java `@Email` 的核心拒绝语义；negative tests 覆盖无 `@`、未引用空格、开头/连续点、超过 64 字符 local part 和 display-name address，避免退化为 kin-openapi 的宽松示例正则。
  - `localDateTime` 对齐 Java request binding 对 `LocalDateTime` 真实日历值的拒绝语义；OpenAPI pattern 继续负责秒精度外形，custom rule 负责 2 月 30 日、25 点等语义值。
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
- 方案 G：阶段 5 机械删除 handler 校验，不补 email format 与真实日历日期执行。
  - 未采用。会让 spec 中已声明的 email format 失去 runtime evaluator，并让外形合法但日历无效的时间穿过 contract，违背“contract 唯一运行时入口”。
- 方案 H：继续在 handler 保留 email/date 特例，或把这些字段规则移到 service。
  - 未采用。前者继续形成第二 transport 规则源；后者让 service 承担 OpenAPI 输入格式职责并破坏分层。
- 方案 I：使用标准 OpenAPI `format: local-date-time`。
  - 未采用。oasdiff 会把四个既有 query 的标准 format 新增判为 `request-parameter-type-changed` breaking change；`x-validation.rules: localDateTime` 能准确记录既有 handler 行为迁移，同时复用冻结 extension 形态并通过 breaking gate。

## 5. 测试与验证

- `gofmt`：阶段 5 全部变更 Go 文件已格式化。
- `go test ./internal/http/handler/auth ./internal/http/handler/user ./internal/http/handler/system -count=1`：通过；覆盖 auth/user/system mapper、normalize、nil body `MalformedBody`、合法时间转换和“contract-invalid generated value 不由 handler 复验”。
- `go test ./internal/http/contract -count=1`：通过；新增 email format parity 边界、合法 email，以及外形合法但真实日历/时钟无效的 local date-time negative tests，并继续覆盖既有 schema/custom rule 顺序。
- `go test ./internal/http -count=1`：通过；AST policy、blank echo contract 路径、email format、local date-time、path minimum 和既有 router/integration tests 全部通过。
- `go test ./...`：通过。
- `go vet ./...`：通过。
- `make lint`：通过，golangci-lint 报告 `0 issues`。
- `make openapi-lint`：通过，Redocly 接受新增 `localDateTime` vendor-extension rule。
- `make openapi-check`：通过，kin-openapi validate、models/server 重生成及 generated diff 检查均通过，确认 spec 变更未改变 generated 文件。
- `make openapi-breaking-check`：通过，oasdiff 报告 spec 存在差异但 `/api/v1` 无 breaking changes。
- `git diff --check`：通过。
- Java-Go parity 验证：现有 policy test 继续锁定 Java DTO 字段消息；contract negative tests 锁定 email 与管理员时间消息；handler 行为/AST tests 固定 Go spec-first 的刻意结构差异；service 既有测试继续覆盖状态、分页、userId、账号唯一与 token/session 业务兜底。
- 不适用项：未修改 SQL、schema、migration 或 sqlc 配置，因此不运行 `sqlc generate` 和 migration 测试。

## 6. 已知限制

- production / Docker 的 request validation 默认策略尚未调整；显式关闭 validation 时 schema/custom/email/local-date-time rules 都会跳过，handler 也不再提供字段校验。这是保留 break-glass 开关的预期代价，下一阶段必须完成默认开启与部署说明。
- user parser 为了映射 service `*time.Time` 仍执行必要类型转换；正常 production route 已由 contract 保证可解析，直接绕过 contract 调用 parser 的非法日期会返回 `COMMON-500` invariant，而不是 handler 字段错误。
- kin-openapi string format registry 是进程级状态；阶段 5 只用 `sync.Once` 注册冻结契约需要的 `email` evaluator。未来新增 format 必须同时有 spec、catalog message 和 runtime negative test。
- native schema validation 未开启 MultiError，通常只返回首条 schema violation；custom engine 已支持按稳定顺序聚合多条 custom violations，但不与 schema failure 混合。
- custom engine 只承诺 string `notBlank`、ASCII password 组合、秒精度 `localDateTime` 和 query local-date-time `notAfter`；cross-field extension 当前按冻结 spec 解释为 query 字段，不承载 service/domain 业务规则。
- 当前 catalog parser 依赖 spec/policy 保证 custom rule 挂载在匹配的 string schema；未来新增 rule 类型时需同步增加 schema 类型约束和 fail-closed evaluator，不应让未知组合静默通过。
- 字段未声明 `x-validation` 或消息未命中 catalog 时仍使用既有 contract fallback；字段覆盖完整性继续由 OpenAPI policy test 约束。
- generated chi binder 的 fallback 错误处理不读取 catalog；validation 开启时由更外层 contract gate 拦截，关闭或未接 contract 时 binder 继续只负责语法绑定错误。
- `ErrorResponse.data` 继续保留 additional properties，以兼容非字段类 `AppError.Details`；只有 requesterror 字段校验构造器保证输出 `data.violations`。
- parser 所在文件仍沿用历史 `validation.go` / `admin_validation.go` 名称；这不影响运行时边界，后续如做纯命名整理应单独保持无行为 diff。

## 7. 对后续版本的影响

- 对简历可用版的价值
  - 字段规则、稳定消息、native/custom runtime、错误定位、安全顺序、纯映射 handler 和统一 envelope 已有可执行 OpenAPI、AST 与 Go 行为测试证据，前端可按 `data.violations` 稳定关联表单字段。
- 对微服务 / 云原生演进的影响
  - catalog 启动期编译、请求期只读且 violations 不依赖单体 service/domain；未来拆分服务时可复用同一 transport contract。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - production / Docker 默认开启 request validation 仍需独立阶段实施并保留 break-glass 说明。
  - 新增 custom rule 必须先扩展 spec/policy/catalog parser，再实现 engine 与 parity tests，不能直接在 handler 或 service 增加 transport 校验。
  - 新增 handler parser 时应复用阶段 5 的 AST/行为边界：只映射/normalize，nil body 可防御，字段合法性必须先在 spec + contract 中闭环。
  - 本阶段不影响 migration、sqlc、缓存、事务、JWT claim 或 service/domain/repository 依赖方向。
