# OpenAPI 与生产文档入口加固设计

## 1. 背景
- Go 版 EventHub 已实现认证、当前用户、管理员用户、system 与 actuator 基础接口，但 `api/openapi` 目前只有占位文件，缺少可验证、可展示、可生成的接口契约源。
- Java 版使用 Springdoc：
  - `backend/pom.xml` 引入 `springdoc-openapi-starter-webmvc-ui`。
  - `backend/src/main/java/com/eventhub/infra/openapi/OpenApiConfig.java` 提供全局标题、描述、版本、联系人和许可证。
  - Controller、request DTO、response VO 通过 `@Tag`、`@Operation`、`@Schema` 注解生成 OpenAPI。
  - `backend/src/main/resources/application.yml` 默认开启 `springdoc.api-docs.enabled=true` 和 `springdoc.swagger-ui.enabled=true`。
  - `backend/src/main/resources/application-prod.yml` 默认关闭二者。
  - `OpenApiProductionSecurityTest` 验证 prod 下 `/v3/api-docs` 与 `/swagger-ui.html` 不应继续公开，携带管理员 token 也不应取得文档资源。
- Go 版没有 Springdoc 注解扫描能力，继续依赖代码注解生成契约会引入额外运行时或构建复杂度。当前更适合用 spec-first OpenAPI，把契约文件作为 Go 端 API 的显式源。

## 2. 目标
- 新增 `api/openapi/eventhub.yaml` 作为当前 Go 版全部 HTTP 接口的 OpenAPI 3.0 契约源。
- 覆盖当前全部接口：
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout`
  - `GET /api/v1/me`
  - `GET /api/v1/admin/users`
  - `PATCH /api/v1/admin/users/{userId}/status`
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET /actuator/health`
  - `GET /actuator/info`
- OpenAPI schemas 使用统一 `ApiResponse` 和 `PageResponse`，并通过具体包装 schema 表达泛型 data，例如 `ApiResponseUserInfo`、`ApiResponseLogin`、`ApiResponseAdminUserPage`。
- 新增文档 handler 暴露：
  - `GET /openapi.yaml`
  - `GET /swagger/*`
- 新增 `OPENAPI_ENABLED` 配置：
  - dev/test 默认 `true`。
  - prod 默认 `false`。
  - 显式环境变量可覆盖默认值。
- router 根据配置只在启用时注册 `/openapi.yaml` 与 `/swagger/*`；prod 默认不注册，因此统一落入现有 `COMMON-404` 未匹配路由响应。
- 使用 `oapi-codegen` 生成 `api/openapi/gen/eventhub.gen.go` 的 types 和 chi server interface，作为契约漂移检查和后续 typed router 对接准备。
- Makefile 增加：
  - `make openapi-validate`
  - `make openapi-generate`
- 补测试覆盖：
  - dev/test 配置默认可访问文档。
  - prod 配置默认不可访问 `/openapi.yaml` 与 `/swagger/*`。
  - OpenAPI 生成产物与契约不漂移。

## 3. 非目标
- 本次不改业务 API 语义、错误码、鉴权逻辑、JWT claim、数据库表、migration、sqlc query 或 repository。
- 本次不把业务 handler 改造成 generated server interface 的实现；生成接口只用于契约漂移检查和未来迁移准备。
- 本次不迁移 Java 的注解式 `@Operation` / `@Schema` 生成模式。
- 本次不引入运行时 OpenAPI 请求校验 middleware；handler 的入参校验仍由现有 HTTP validation 代码负责。
- 本次不在 prod 中提供管理员可访问的 Swagger UI；prod 默认连路由都不注册。
- 本次不新增 Swagger UI 本地静态资源打包依赖；页面可以加载外部 Swagger UI 资源并读取本地 `/openapi.yaml`。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `api/openapi`
    - 新增 `eventhub.yaml`。
    - 新增 Go embed 辅助，让 handler 从同一契约源读取 YAML。
  - `api/openapi/gen`
    - 新增 `oapi-codegen` 生成 types 与 chi server interface。
  - `internal/config`
    - 新增 `OpenAPIConfig` 和 `OPENAPI_ENABLED` 解析。
  - `internal/http/handler/openapi/handler.go`
    - 新增 OpenAPI YAML 与 Swagger UI handler，放在 openapi 能力子包，避免在 handler 根目录长期放具体 handler。
  - `internal/http/router.go`
    - 按 OpenAPI handler 是否装配注册文档路由。
  - `internal/app/providers/http.go`
    - 根据 `platform.Config.OpenAPI.Enabled` 装配或跳过文档 handler。
  - `Makefile`
    - 新增 OpenAPI validate/generate 命令。
  - 测试：
    - config 默认值测试。
    - router dev/test 可访问、prod 不可访问测试。
    - generated OpenAPI 文件不漂移测试。
- 涉及 API：
  - 新增文档入口 `GET /openapi.yaml` 与 `GET /swagger/*`，仅在 `OPENAPI_ENABLED=true` 时存在。
- 涉及表 / 缓存 / 外部接口：
  - 不涉及数据库表、索引、migration、sqlc、Redis 或外部业务接口。
  - Swagger UI HTML 会引用外部 Swagger UI CDN 资源；服务端只负责返回 HTML 与本地 YAML。
- 影响 `docs/ai/parity/java-go-parity-matrix.md`：是。本次从“待迁移”推进到“已对齐/规则已初始化”，记录 Java Springdoc 与 Go spec-first 的刻意差异、prod hardening 和验证命令。

## 5. 领域建模
- `OpenAPIContract`
  - 物理文件：`api/openapi/eventhub.yaml`。
  - 是 Go 版当前 API 契约源，包含 paths、schemas、securitySchemes、tags 和错误响应形态。
  - 与 Java 版 Springdoc 生成出来的 `/v3/api-docs` 对应，但 Go 端不依赖注解扫描。
- `OpenAPIConfig`
  - 字段：`Enabled bool`。
  - 来源：`OPENAPI_ENABLED`，缺省值由 `EVENTHUB_ENV` 推导。
  - dev/test 缺省开启，prod 缺省关闭。
- `OpenAPIHandler`
  - 属于 HTTP 文档入口，不属于具体业务模块。
  - 只返回契约 YAML 和 Swagger UI HTML，不访问 service、repository、database、security principal 或 JWT。
- `GeneratedOpenAPI`
  - 物理文件：`api/openapi/gen/eventhub.gen.go`。
  - 由 `oapi-codegen` 从 `eventhub.yaml` 生成 types 与 chi server interface。
  - 当前不作为运行时路由实现，避免一次性重构现有 handler；用于编译期发现 schema 不合法，并通过测试发现生成产物漂移。
- 与 Java 版领域对象的对应关系：
  - Java `OpenApiConfig` 对应 Go `eventhub.yaml` 的 `info` 元数据。
  - Java Controller `@Tag` / `@Operation` 对应 Go OpenAPI `tags` 与 operation `summary/description`。
  - Java request DTO / VO `@Schema` 对应 Go OpenAPI `components.schemas`。
  - Java `ApiResponse<T>` / `PageResponse<T>` 对应 Go `ApiResponse` 基础 schema 与具体 data 包装 schema。

## 6. API 设计
- `GET /openapi.yaml`
  - 仅 `OPENAPI_ENABLED=true` 时注册。
  - 成功返回 `200`，`Content-Type: application/yaml; charset=utf-8`。
  - 响应体为 `api/openapi/eventhub.yaml` 的内容。
  - prod 默认不注册，返回现有统一 `COMMON-404`。
- `GET /swagger/*`
  - 仅 `OPENAPI_ENABLED=true` 时注册。
  - `/swagger` 重定向到 `/swagger/`。
  - `/swagger/` 与 `/swagger/index.html` 返回 Swagger UI HTML，页面读取 `/openapi.yaml`。
  - prod 默认不注册，返回现有统一 `COMMON-404`。
- `api/openapi/eventhub.yaml` 覆盖业务 API：
  - 公开接口：register、login、refresh、system ping、system echo、actuator health、actuator info。
  - 认证接口：logout、me、admin users list、admin update status，声明 BearerAuth。
  - 管理员接口在描述中标注需要 ADMIN 角色；OpenAPI 只表达 BearerAuth，角色授权仍由服务端 middleware 执行。
- OpenAPI schema 约定：
  - `ApiResponse` 保留 `code/message/data/requestId/timestamp`，`data` 为 nullable object。
  - 具体响应通过 `allOf` 组合 `ApiResponse` 并覆盖 `data` 类型。
  - `PageResponseUserInfo` 表达用户列表分页字段。
  - `EchoRequest` 对齐现有 system handler 校验：`message` 必填且最长 64 个字符，`tag` 可选且最长 32 个字符。
  - `ErrorResponse` 复用 `ApiResponse` 并允许 `data` 为字段错误 map 或 null。
  - actuator 端点保持 Java Actuator 风格，不包 `ApiResponse`。
- 错误码 / 异常场景：
  - 业务接口保留现有 `COMMON-400`、`COMMON-404`、`COMMON-500`、`AUTH-401`、`AUTH-403`、`AUTH-409`。
  - 文档路由禁用时不新增错误码，沿用 router 未匹配 `COMMON-404`。
  - Swagger UI 资源加载失败属于浏览器外部资源问题，不影响服务端 OpenAPI YAML 可访问性。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Java 使用 Springdoc 由注解生成 `/v3/api-docs`，Go 使用 spec-first 手写 `eventhub.yaml`。
  - Java Swagger UI 路径是 `/swagger-ui.html` 和 `/swagger-ui/**`，Go 本次使用 `/swagger/*`，符合任务要求并避免与 Springdoc 路径机械绑定。
  - Java prod 未认证访问文档路径先落到 `AUTH-401`，携带管理员 token 后为 `COMMON-404`；Go prod 默认不注册文档路由，因此无论是否携带 token 都是 `COMMON-404`。这是因为 Go 当前没有 Spring Security 的“未放行但路径存在”中间状态，prod 安全目标是文档资源不可访问。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - OpenAPI 契约文件与生成代码属于构建产物一致性，不涉及业务数据一致性。
  - 使用 `make openapi-generate` 和漂移测试保证 `api/openapi/gen/eventhub.gen.go` 与 `eventhub.yaml` 同步。

## 8. 关键流程
- 正常流程：
  1. `config.Load()` 根据 `EVENTHUB_ENV` 得出环境。
  2. `OPENAPI_ENABLED` 未配置时，dev/test 默认 true，prod 默认 false；配置存在时显式覆盖。
  3. `ProviderHTTP` 读取 `platform.Config.OpenAPI.Enabled`。
  4. 开启时创建 `OpenAPIHandler` 并放入 `RouterDependencies`。
  5. `NewRouter` 注册 `/openapi.yaml`、`/swagger`、`/swagger/`、`/swagger/*`。
  6. 请求 `/openapi.yaml` 时 handler 返回 embed 的 `eventhub.yaml`。
  7. 请求 `/swagger/*` 时 handler 返回 Swagger UI HTML。
- 异常流程：
  - prod 默认关闭时不创建 OpenAPI handler，router 不注册文档路由，请求落入 `NotFound`，返回 `COMMON-404`。
  - `OPENAPI_ENABLED=false` 显式配置时，即使 dev/test 也不注册文档路由。
  - `OPENAPI_ENABLED=true` 显式配置时，即使 prod 也会注册文档路由；这是受控临时排障或内网演示入口，需要部署侧谨慎使用。
- 状态流转：
  - 不涉及业务状态机。
  - 只涉及配置状态 `enabled/disabled`。
- handler / service / repository / sqlc/database 分工：
  - OpenAPI handler 是跨业务 HTTP 文档 handler，只处理静态契约输出；代码放在 `internal/http/handler/openapi` 能力子包，不下沉到 auth/user/system 业务子包。
  - 不新增 service。
  - 不新增 repository。
  - 不触碰 sqlc/database。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无，本次不涉及库存或订单。
- 如何防重复提交：无，本次不涉及写业务操作。
- 事务边界在哪里：无数据库事务。
- 缓存放在哪里，为什么：
  - 不引入 Redis 或业务缓存。
  - OpenAPI YAML 通过 Go embed 编入二进制，等价于只读静态资源；每个请求返回同一份内存数据。
  - Swagger UI HTML 使用常量字符串返回，避免运行时读取模板文件。

## 10. 权限与安全
- 哪些角色能访问：
  - dev/test 默认任何调用方可访问文档入口，对齐 Java 非生产联调语义。
  - prod 默认任何调用方都不可访问，因为路由不注册。
  - 管理员角色也不能绕过 prod 默认关闭；携带 token 不影响未注册路由结果。
- 鉴权与鉴别约束：
  - 文档入口本身不走 auth middleware；是否存在由配置决定。
  - 业务 API 的 BearerAuth 仍由现有 auth middleware 处理。
- JWT claim 边界：
  - 不修改 JWT。
  - 不把角色、邮箱、用户名、用户状态写入 JWT。
- 是否涉及敏感信息、审计或操作日志：
  - OpenAPI 会暴露接口路径、字段、错误码和 schema，prod 默认关闭是安全边界。
  - 本次不新增审计日志；后续如果允许 prod 受控开启，应评估来源 IP、认证、审计和内网访问策略。

## 11. 测试策略
- 单元测试：
  - `internal/config` 覆盖 dev/test/prod 默认值与 `OPENAPI_ENABLED` 覆盖。
  - `internal/http` 覆盖启用时 `/openapi.yaml` 返回 YAML、`/swagger/` 返回 HTML。
  - 覆盖禁用时 `/openapi.yaml` 与 `/swagger/index.html` 返回统一 `COMMON-404`。
- service / repository 测试：
  - 不新增；本次不触碰业务服务和持久化。
- migration / sqlc 验证：
  - 不运行 migration 生成；本次无 schema/query 变化。
  - `go test ./...` 会编译已有 sqlc generated code 和新增 oapi generated package。
- 接口验证：
  - router 层 `httptest` 验证文档入口注册与未注册行为。
  - provider 层测试可确认 config 进入 HTTP 装配。
- OpenAPI validate：
  - `make openapi-validate` 使用 OpenAPI parser 校验 `api/openapi/eventhub.yaml`。
- OpenAPI 不漂移：
  - `make openapi-generate` 由 `eventhub.yaml` 生成 `api/openapi/gen/eventhub.gen.go`。
  - 新增测试在临时目录重新生成并与仓库内 generated file 比较；如果开发者改了 YAML 但忘记重新生成，测试失败。
  - CI 可串联 `make openapi-validate`、`make openapi-generate`、`git diff --exit-code api/openapi/gen/eventhub.gen.go` 或直接运行该漂移测试。
- 异常场景验证：
  - prod 默认关闭。
  - dev/test 默认开启。
  - 显式 false 覆盖 dev/test 默认开启。
  - 显式 true 覆盖 prod 默认关闭。
- Java-Go parity 验证：
  - 对照 Java `OpenApiConfig`、`application*.yml`、`SecurityConfig` 文档白名单逻辑、`OpenApiProductionSecurityTest`。
- 需要运行的命令：
  - `gofmt`。
  - `make openapi-validate`。
  - `make openapi-generate`。
  - `go test ./...`。
  - `go vet ./...`。

## 12. 风险与替代方案
- 当前方案的风险：
  - spec-first 需要开发者维护 YAML，若新增接口忘记更新 OpenAPI，会产生契约漂移。
  - generated server interface 暂未接入现有 router，不能自动保证 handler 签名与 operation 完全绑定。
  - Swagger UI HTML 使用 CDN 资源，本地无网络时 UI 页面可能无法完整加载；但 `/openapi.yaml` 仍可用于工具导入和 validate。
  - prod 显式设置 `OPENAPI_ENABLED=true` 会暴露文档入口，部署侧需要谨慎。
- 备选方案：
  - 方案 A：运行时从 Go handler 注释或反射自动生成 OpenAPI。
  - 方案 B：直接接入 generated chi server interface，让所有 handler 实现 oapi-codegen 接口。
  - 方案 C：引入本地 Swagger UI 静态资源依赖或 vendored assets。
  - 方案 D：prod 下注册文档路由但要求认证或 ADMIN。
- 为什么不选备选方案：
  - 不选方案 A：Go 端没有 Springdoc 等价生态，注解/反射式生成容易让契约来源模糊，也不利于 Java-Go parity 审核。
  - 不选方案 B：当前 handler 已稳定，强行接入 generated interface 会扩大改动面；本次先把契约源、生成和验证链路建立起来。
  - 不选方案 C：本次目标是后端文档入口和 prod hardening，vendored UI assets 会增加依赖和维护成本；后续如需离线 Swagger UI 可单独设计。
  - 不选方案 D：prod 安全目标是默认不暴露文档资源。认证保护不能消除接口 schema 暴露风险，也不符合 Java prod 关闭 springdoc 资源本身的核心语义。
- 后续可演进点：
  - 将 generated server interface 纳入 router 适配，减少 path/method 漂移。
  - 在 CI 增加 OpenAPI validate、generate 后 diff 检查。
  - 接入 Spectral 或自定义规则检查错误响应、ApiResponse/PageResponse 包装约定。
  - 如需生产临时文档，可设计内网、认证、审计和短期开关组合，而不是直接长期打开 `OPENAPI_ENABLED=true`。
