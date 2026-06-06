# ADR：OpenAPI spec-first 契约源

## 标题
Go 版 OpenAPI 采用 `api/openapi/eventhub.yaml` 作为 spec-first 契约源

## 状态
- accepted

## 背景
Java 版 EventHub 使用 Springdoc：Controller 与 DTO/VO 上的 `@Tag`、`@Operation`、`@Schema` 注解由 Springdoc 扫描生成 `/v3/api-docs`，`OpenApiConfig` 集中维护标题、描述、版本、联系人和许可证。这个模式适合 Spring MVC，因为请求映射、Bean Validation 和 schema 注解都位于同一套框架元数据中。

Go 版当前使用标准库 HTTP server + chi router，handler、DTO、service result 和 repository model 边界是显式拆开的。Go 生态没有与 Springdoc 等价、低成本且天然对齐当前分层的注解扫描机制。如果为了“像 Java 一样自动生成”引入反射、注释扫描或运行时中间件，契约来源会变得模糊，也可能把 HTTP DTO、service result 和生成模型混在一起。

本次需要为当前全部 API 建立可验证、可展示、可生成的 OpenAPI 契约，并为后续 API 演进提供不漂移检查。

## 决策
Go 版选择：

- `api/openapi/eventhub.yaml` 是当前 OpenAPI 契约唯一源。
- `eventhub.yaml` 覆盖当前全部业务、system 和 actuator 接口。
- schemas 统一使用 `ApiResponse` 和 `PageResponse`，并通过具体响应 schema 表达 data 类型。
- 使用 `oapi-codegen` 从 `eventhub.yaml` 生成 `api/openapi/gen/eventhub.gen.go`：
  - 生成 types。
  - 生成 chi server interface。
- 当前不强制现有 handler 实现 generated server interface。
- Makefile 提供：
  - `make openapi-validate`
  - `make openapi-generate`
  - `make openapi-check`
- Java-Go parity matrix 记录 Go spec-first 与 Java Springdoc 注解生成的刻意差异。

## 备选方案
- 方案 1：继续等待，不维护 OpenAPI，等业务更多后再补。
- 方案 2：使用 Go 注释、反射或 handler 扫描工具自动生成 OpenAPI。
- 方案 3：spec-first YAML + oapi-codegen 生成 types/server interface。
- 方案 4：一次性让所有现有 handler 改造为 oapi-codegen generated server interface 的实现。
- 方案 5：只写 YAML，不生成代码。

## 决策理由
选择方案 3，原因是：

- `eventhub.yaml` 让 API 契约成为显式文件，便于 Java-Go parity 审查、联调、文档展示和 CI 校验。
- spec-first 不依赖 Springdoc 式注解生态，符合 Go 项目当前清晰分层。
- oapi-codegen 生成 types/server interface 能证明 YAML 足以被工具消费，并为后续 typed router 对接预留路径。
- 当前 handler 已经稳定，强制改造成 generated interface 会扩大改动面，也容易把本次文档入口加固任务变成路由重构。
- 只写 YAML 不生成代码，难以及时发现 schema 对生成工具不友好或 generated file 漂移。

## 影响
- 好处
  - API 契约有单一源，后续新增接口时可以先改 spec 再改 handler。
  - `make openapi-validate` 可以在 CI 中校验契约合法性。
  - `make openapi-generate` 和 `make openapi-check` 可以发现生成产物漂移。
  - 生成的 types/server interface 给后续 OpenAPI 驱动 handler 适配留下低风险入口。
- 代价
  - 开发者需要手动维护 YAML；如果流程执行不到位，仍可能出现 handler 与 spec 漂移。
  - 当前 generated server interface 尚未绑定 router，不能自动保证运行时 handler 覆盖每个 operation。
  - 新增 `github.com/oapi-codegen/runtime` 作为生成 chi server interface 的轻量依赖。
- 后续可能需要调整的地方
  - 将 generated server interface 纳入 adapter 层，减少 path/method 手工漂移。
  - 增加 Spectral 或自定义规则，检查所有业务响应都使用 `ApiResponse`，所有分页响应都使用 `PageResponse`。
  - CI 中固定执行 `make openapi-check`。
