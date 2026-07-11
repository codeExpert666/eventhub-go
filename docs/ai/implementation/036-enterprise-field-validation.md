# Enterprise Field Validation

## 1. 本次改动解决了什么问题

- 完成企业级字段校验体系的阶段 1：在不改变运行时 contract gate 行为的前提下，把当前已实现请求字段的规则消息和项目规则声明收敛到 `api/openapi/eventhub.yaml`。
- 解决 OpenAPI 原生 schema 虽已声明长度、格式、枚举和数值范围，但缺少逐规则稳定中文消息的问题。
- 建立 `x-validation` policy test，防止后续出现缺少 message、未知 custom rule、残缺 crossFields 或 Java parity 消息漂移。

## 2. 改动内容

- 新增了什么
  - 为 `RegisterRequest`、`LoginRequest`、`RefreshTokenRequest`、`UpdateUserStatusRequest`、`EchoRequest` 的当前请求字段声明字段级 `x-validation`。
  - 为 `GET /api/v1/admin/users` 的 `page`、`size`、`username`、`email`、`status` 和四个时间 query 参数声明逐规则 messages。
  - 为管理员用户查询声明 `createdAtRange`、`updatedAtRange` 两条 operation 级 `crossFields/notAfter` 规则。
  - 在 `api/openapi/openapi_policy_test.go` 新增请求字段规则消息、字段 custom rule 白名单、crossFields 格式和 Java parity 精确消息测试。
- 修改了什么
  - `docs/ai/parity/java-go-parity-matrix.md` 中“企业级字段校验体系”由“已决策”更新为“规则已初始化”。
- 删除了什么
  - 无。handler 手写字段校验按阶段边界继续保留。
- 文件移动和 package 边界变化
  - 无文件移动，无 production package、DTO、service Command / Query / Result、repository 或依赖边界变化。
- 具体类型、接口和测试替身
  - production code 未新增类型、接口或测试替身；仅在 OpenAPI policy test 内新增 spec 采集、扩展解析和断言辅助类型。
- 是否更新 Java-Go parity 记录
  - 是。当前 Java DTO 的 Bean Validation 核心消息已进入 OpenAPI 契约并由测试精确锁定；时间格式消息沿用当前 Go handler 的字段级提示，状态枚举消息复用 Java 管理员查询允许值提示。

## 3. 为什么这样设计

- 关键设计原因
  - 严格复用 design 036 和 ADR-0028：OpenAPI spec 是字段规则与字段消息的唯一契约源，policy test 只负责静态治理，运行时执行仍留给后续 `internal/http/contract` 阶段。
  - policy test 从 operation request body 顶层字段和 query 参数收集规则，不扫描响应模型，也不把本阶段未要求的 path/header/cookie 纳入改动面。
  - `UpdateUserStatusRequest.status` 和 query `status` 保持现有 `$ref`，在 `SchemaRef` 同级声明扩展；没有为挂载扩展改成 `allOf` 或内联 enum，因此 oapi-codegen 生成类型保持不变。
- 与 Go 项目当前阶段的匹配点
  - 不新增 `go-playground/validator`，不恢复 Go embed / `SpecYAML()`，不修改 `internal/http/contract`、handler、service、domain 或 repository。
  - `notBlank` 使用专用布尔声明，字段 `rules` 当前只允许 `containsLetterAndDigit`，operation `crossFields` 当前只允许 `notAfter`，避免扩展成为自由格式。
- 与 Java 版业务语义的对齐方式
  - 注册、登录、刷新、分页筛选、状态更新和 echo 消息逐条对照 Java DTO 注解。
  - `required`、长度、格式、枚举和数值范围使用 OpenAPI 原生规则；`notBlank`、密码字母数字组合和时间范围使用 `x-validation` 补充语义。

## 4. 替代方案

- 方案 A：本阶段同时实现 runtime rule catalog 和 custom rule engine。
  - 未采用。本阶段明确只建立 spec 规则和 policy test，提前修改 contract gate 会跨越阶段边界。
- 方案 B：立即删除 handler 手写校验。
  - 未采用。运行时 custom rule 尚未执行，提前删除会改变或削弱现有请求校验行为。
- 方案 C：使用 `go-playground/validator` 或 generated model struct tag 承载消息。
  - 未采用。会产生 OpenAPI 与 Go tag 两套规则源，违背既定 spec-first 决策。
- 方案 D：把两个 status `$ref` 改为 `allOf` 或内联 enum 后再挂扩展。
  - 未采用。`SchemaRef.Extensions` 已能承载 vendor extension，改 schema 结构会无谓增加生成物漂移风险。

## 5. 测试与验证

- `gofmt -w api/openapi/openapi_policy_test.go`：通过。
- `go test ./api/openapi -count=1`：通过。
- `make openapi-check`：通过；kin-openapi validate 和 oapi-codegen 重新生成均成功，`models.gen.go`、`server.gen.go` 无漂移。
- `make openapi-lint`：通过；Redocly 报告 API description valid。
- `go test ./...`：通过，无异常耗时或卡点。
- `go vet ./...`：通过。
- `git diff --check`：通过。
- Java-Go parity 验证：policy test 精确断言 Java DTO 的稳定中文消息、`containsLetterAndDigit` 以及两条 `notAfter`；同时覆盖 Go 现有时间 pattern 和 status enum 的稳定消息。
- 不适用项：未修改 SQL、schema、migration 或 sqlc 配置，因此不运行 `sqlc generate` 和 migration 测试；未改变运行时行为，因此本阶段不新增 contract/handler 集成场景。

## 6. 已知限制

- `internal/http/contract` 尚未解析或执行 `x-validation`，本阶段声明不会改变运行时 schema/custom rule 校验结果。
- `containsLetterAndDigit`、`notBlank`、`notAfter` 的 runtime custom rule engine 尚未实现。
- 字段错误仍未演进为稳定 `data.violations`。
- handler 私有校验仍保留，尚未收敛为纯 request -> Command / Query 映射和 normalize。
- production / Docker 的 request validation 默认策略尚未调整。
- policy 当前按阶段 1 范围扫描 request body 顶层字段和 query 参数；嵌套请求对象以及 path/header/cookie 的字段消息治理留待有明确业务需求时扩展。
- 两个 status 字段把 `x-validation` 放在 `$ref` sibling；当前 kin-openapi、oapi-codegen 和 Redocly 工具链均已验证通过，但严格按 OpenAPI 3.0 处理 Reference Object 的外部工具可能忽略该扩展，后续对外交换 spec 时需要做互操作验证。
- 结构债务：OpenAPI policy helper 当前与既有 policy 集中在同一测试文件；若规则种类显著增长，可在保持同 package 的前提下按验证主题拆分测试文件。

## 7. 对后续版本的影响

- 对简历可用版的价值
  - OpenAPI 已能同时表达标准字段约束、项目规则和稳定中文消息，并由 CI 可执行测试治理，字段契约具备可审查、可追溯基础。
- 对微服务 / 云原生演进的影响
  - 规则与消息位于传输契约，不依赖单体 service/domain 实现，后续拆分服务时可继续复用同一 spec-first 约束方式。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续阶段可在 `internal/http/contract` 编译只读 rule catalog 并执行 custom rules，无需重新定义字段消息。
  - handler 清理必须等 runtime contract tests 证明规则已被接管后再进行。
  - 新增请求字段规则时，必须同步声明对应 message；新增 custom rule 前必须先更新 design/ADR 允许范围和 policy allowlist。
  - 本阶段不影响 migration、sqlc、缓存、事务、认证/授权或 service/domain/repository 依赖方向。
