# ADR-0028 Enterprise Field Validation Contract

## 标题
以 OpenAPI `x-validation` 和 HTTP contract gate 作为企业级字段校验事实来源

## 状态
- accepted

## 背景
Java 版 EventHub 通过 Jakarta Bean Validation 在 Controller 边界执行字段校验，并把用户可读中文消息直接声明在 DTO 注解上。例如 `RegisterRequest` 使用 `@NotBlank`、`@Size`、`@Pattern`、`@Email`，`AdminUserQueryRequest` 使用 `@Min`、`@Max`、`@Pattern` 和 `@AssertTrue` 表达 query 与跨字段规则。

Go 版当前已经采用 spec-first OpenAPI，并通过 ADR-0027 引入 `internal/http/contract`，在 generated strict handler 前执行 OpenAPI request contract gate。该 gate 已覆盖 path/query/header/cookie/body/content-type/security 的基础契约校验，也已经把 `BearerAuth` 和 `x-required-roles` 接入运行时。

但字段校验体系仍不完整：OpenAPI 原生 schema 可以表达长度、枚举、格式等规则，却不能稳定表达每条规则的中文业务消息，也不能自然表达 `notBlank`、密码同时包含字母和数字、时间范围不能倒置等项目语义。当前这些规则仍散落在 handler 私有 parser 中，导致 spec、Java parity 和 Go runtime 三方存在漂移风险。

## 决策
采用以下决策：

- `api/openapi/eventhub.yaml` 是字段规则与字段消息的唯一契约源。
- 使用项目级 OpenAPI vendor extension `x-validation` 表达企业字段校验补充语义：
  - 字段级 `notBlank`。
  - 字段级 `messages`，为 `required`、`minLength`、`maxLength`、`pattern`、`format`、`enum`、`minimum`、`maximum` 等规则提供稳定中文消息。
  - 字段级 `rules`，承载 `containsLetterAndDigit` 等项目自定义规则。
  - operation 级 `crossFields`，承载 `notAfter` 等跨字段规则。
- `internal/http/contract` 是唯一运行时字段校验入口。
  - 启动期从 OpenAPI doc 编译 `ValidationCatalog`。
  - 运行时执行 OpenAPI schema validation 和 `x-validation` custom rules。
  - schema violation 优先映射到 `x-validation.messages`。
  - custom rule violation 使用 spec 中声明的 message。
- 字段校验错误返回稳定 `data.violations` 结构，而不是松散的 `data[field] = message` map。
- handler 不再做字段合法性校验。
  - handler 只做 nil body 防御、normalize 和 generated model 到 service Command / Query 的映射。
  - service/domain/repository 不依赖 OpenAPI、contract 或 HTTP requesterror。
- production / Docker 默认开启 `OPENAPI_REQUEST_VALIDATION_ENABLED`。
  - 保留显式关闭作为 break-glass 开关。
  - `OPENAPI_ENABLED` 继续只控制 `/openapi.yaml` 与 `/swagger/*` 文档入口。
- OpenAPI policy test 必须约束 `x-validation` 的写法，避免扩展变成自由格式。

## 备选方案
- 方案 1：只使用 OpenAPI 原生 schema。
- 方案 2：使用 `go-playground/validator`，在 Go struct tag 中声明校验规则和消息。
- 方案 3：从 OpenAPI 生成 Go 校验代码。
- 方案 4：继续保留 handler 手写字段校验，把错误消息抽成常量。

## 决策理由
- 选择 `x-validation` 是因为 Go 仓库已经采用 SPEC-first，字段规则、字段消息、文档、生成代码、运行时校验和 policy test 应共享同一个契约源。
- OpenAPI 原生 schema 是基础，但不足以表达 Java Bean Validation 的全部用户体验，尤其是 `@NotBlank`、`@AssertTrue` 和字段级中文消息。
- `internal/http/contract` 已经是 request contract gate 的项目边界，继续扩展 rule catalog 和 custom rules 能保持架构集中，不把 transport 校验散落到 handler。
- 不选择 `go-playground/validator`，因为它会引入第二套 Go tag 规则源，与 OpenAPI schema 双写。
- 暂不选择代码生成校验器，是为了避免当前阶段引入额外复杂工具链；等 `x-validation` 规范稳定后，可以再生成优化。
- 不保留 handler 手写校验，是因为这会继续复制 Java 注解语义，无法解决长期漂移问题。

## 影响
- 好处
  - 字段规则、字段消息和运行时行为以 OpenAPI spec 为事实来源。
  - Java Bean Validation 语义能在 Go spec-first 架构中被机器校验、运行时执行和文档索引。
  - handler 收敛为映射层，service 继续承载业务规则，分层更清楚。
  - `data.violations` 结构更适合前端表单、审计日志和未来国际化。
  - prod 默认开启 request validation，更符合企业入口契约治理。
- 代价
  - 需要维护项目级 `x-validation` 规范和 policy test。
  - 初始实现需要改 OpenAPI schema、contract gate、错误映射、handler parser 和测试。
  - 错误 details 结构变化可能影响依赖旧 `data[field]` map 的调用方。
  - custom rule engine 需要严格边界，避免承载 service/domain 业务规则。
- 后续可能需要调整的地方
  - 当 API 数量扩大时，可从 `x-validation` 生成 Go 校验器以降低运行时解析成本。
  - 可增加 `messageKey` 和多语言 catalog，支持 i18n。
  - 可将 Redocly 自定义规则接入 `make openapi-lint`，把 `x-validation` 规范前移到 lint 阶段。
  - 可对未知 query/header/cookie 做 allow -> warn -> reject 的企业治理升级。
