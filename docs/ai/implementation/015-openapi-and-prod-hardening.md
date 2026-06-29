# OpenAPI 与生产文档入口加固实现说明

## 1. 本次改动解决了什么问题

本次补齐 Go 版 EventHub 的 OpenAPI / Swagger UI 基础能力，并对齐 Java 版 Springdoc 的生产环境安全语义：

- 新增 `api/openapi/eventhub.yaml`，覆盖当前业务、system 与 actuator API。
- 新增 `oapi-codegen` 生成的 types 和 chi server interface。
- 新增 `/openapi.yaml` 与 `/swagger/*` 文档入口。
- 新增 `OPENAPI_ENABLED` 开关，dev/test 默认开启，prod 默认关闭。
- prod 默认不注册文档路由，管理员 token 也不能访问文档资源。
- Makefile 新增 OpenAPI validate/generate/check 命令。
- 测试覆盖 OpenAPI 默认配置和文档入口启用/禁用行为。

## 2. 改动内容
- 新增了什么
  - 设计文档：`docs/ai/design/015-openapi-and-prod-hardening.md`。
  - ADR：
    - `docs/ai/adr/0018-openapi-spec-first.md`
    - `docs/ai/adr/0019-prod-disable-api-docs.md`
  - OpenAPI 契约：
    - `api/openapi/eventhub.yaml`
    - `api/openapi/spec.go`（当时文件名；后续本地静态资源重构已更名为 `api/openapi/assets.go`）
  - OpenAPI 生成代码：
    - `api/openapi/gen/eventhub.gen.go`
  - 文档 handler：
    - `internal/http/handler/openapi/handler.go`
  - 测试：
    - `internal/config/config_test.go`
    - 扩展 `internal/http/router_test.go`
    - 扩展 `internal/app/providers/http_test.go`
- 修改了什么
  - `internal/config`
    - 新增 `OpenAPIConfig`。
    - 新增 `OPENAPI_ENABLED` bool 解析。
    - dev/test 默认开启，prod 默认关闭。
  - `internal/app/providers/http.go`
    - 根据 `platform.Config.OpenAPI.Enabled` 决定是否创建 `OpenAPIHandler`。
  - `internal/http/router.go`
    - 仅当 `RouterDependencies.OpenAPI` 存在时注册 `/openapi.yaml`、`/swagger`、`/swagger/`、`/swagger/*`。
  - `configs/*.env.example`
    - dev/test 示例设置 `OPENAPI_ENABLED=true`。
    - prod 示例设置 `OPENAPI_ENABLED=false`。
  - `Makefile`
    - 新增 `openapi-validate`、`openapi-generate`、`openapi-check`。
  - `go.mod` / `go.sum`
    - 新增 `github.com/oapi-codegen/runtime`，用于 generated chi server interface 的参数绑定。
- 删除了什么
  - 未删除生产代码。
  - `api/openapi/.gitkeep` 保留不影响功能；目录已有真实文件后后续可清理。
- 是否更新 Java-Go parity 记录
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
  - 本次触发 API 契约、文档入口、生产安全边界、测试策略和 Go/Java 刻意差异 parity 更新。

## 3. 为什么这样设计
- 关键设计原因
  - Java Springdoc 依赖 Spring MVC 注解和 Bean Validation 元数据；Go 端没有等价的低成本注解扫描机制，因此选择 spec-first。
  - `api/openapi/eventhub.yaml` 作为唯一契约源，便于联调、审查、validate、generate 和 parity matrix 索引。
  - 当时的 `api/openapi/spec.go` 通过 Go embed 暴露同一个 YAML，避免 handler 运行时依赖工作目录读取文件；后续本地静态资源重构已更名并收敛为 `api/openapi/assets.go` 路径常量。
  - `oapi-codegen` 生成 types/server interface，但当前不强制改造现有 handler，避免把文档入口任务扩大成 router 重构。
  - 文档 handler 位于 `internal/http/handler/openapi/handler.go`，属于跨业务 HTTP 文档入口；它进入 openapi 能力子包而不是 handler 根目录或 auth/user/system 业务子包，也不访问 service 或 repository。
  - `EchoRequest` schema 显式声明 `message maxLength=64`、`tag maxLength=32`，对齐现有 system handler 的运行时校验，避免生成客户端误认为超长字段合法。
  - prod 默认不注册路由，而不是注册后要求认证；这更接近 Java prod 关闭 springdoc 资源本身的安全目标。
- 与 Go 项目当前阶段的匹配点
  - router 仍只负责 URL、method、中间件和 handler 绑定。
  - provider 负责从 config 装配 OpenAPI handler，符合 composition root 规则。
  - 不触碰 service/repository/sqlc/database，不破坏 `handler -> service -> repository -> sqlc/database`。
  - 不修改 JWT claim，不把角色、邮箱、用户名、用户状态写入 JWT。
  - OpenAPI schemas 复用 `ApiResponse` 与 `PageResponse` 契约，避免把 sqlc generated model 暴露为 HTTP schema。
- 与 Java 版业务语义的对齐方式
  - OpenAPI `info` 对齐 Java `OpenApiConfig` 的标题、描述、版本、联系人和许可证。
  - paths 对齐 Java Controller 与当前 Go handler 的路径、方法、请求字段、响应字段和错误码。
  - dev/test 默认开放，对齐 Java `application.yml`。
  - prod 默认关闭，对齐 Java `application-prod.yml` 与 `OpenApiProductionSecurityTest` 的核心安全目标。

## 4. 替代方案
- 方案 A：Go 端通过注释或反射自动生成 OpenAPI。
  - 没有采用。Go 当前分层没有 Springdoc 式统一框架元数据，自动生成会让契约来源不清晰，也可能引入重量级依赖。
- 方案 B：只手写 YAML，不生成代码。
  - 没有采用。缺少生成检查时，schema 对工具是否友好、生成代码是否漂移都难以及时发现。
- 方案 C：一次性把现有 handler 改造成 oapi-codegen generated server interface。
  - 没有采用。当前 handler 已稳定，本次主要目标是契约源和 prod hardening；强行改造会扩大行为风险。
- 方案 D：prod 下文档入口要求登录或 ADMIN。
  - 没有采用。认证保护不能消除 OpenAPI schema 泄露风险；Java prod 的核心语义是关闭 springdoc 资源本身。
- 方案 E：把 Swagger UI 静态资源 vendored 到仓库。
  - 没有采用。本次后端只需要提供文档入口，vendored UI assets 会增加维护成本；后续如果要离线 Swagger UI 可单独设计。

## 5. 测试与验证
- 跑了哪些测试
  - `make openapi-validate`：通过。
  - `make openapi-generate`：通过。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w`：已对新增和修改的 Go 文件运行。
  - `make openapi-validate`：使用 `kin-openapi` 校验 `api/openapi/eventhub.yaml`。
  - `make openapi-generate`：使用 `oapi-codegen` 生成 `api/openapi/gen/eventhub.gen.go`。
  - `go test ./...`：通过，生成包也被编译。
  - `go vet ./...`：通过。
  - `sqlc generate`：未运行；本次没有修改 SQL、schema 或 sqlc 配置。
  - migration 测试：未单独运行；本次没有 migration 变化，`go test ./...` 已覆盖现有 repository 测试。
- 手工验证了哪些场景
  - 检查 `eventhub.yaml` 覆盖任务要求的 11 个接口，并额外记录当前 router 已有的 actuator HEAD 探针。
  - 检查 generated server interface 包含当前 OpenAPI operations。
  - 检查 router 中文档路由只在 OpenAPI handler 非空时注册。
  - 检查 OpenAPI schema 与 system echo handler 的字段长度约束一致。
- Java-Go parity 如何验证
  - 对照 Java `OpenApiConfig`、`application.yml`、`application-prod.yml`、`SecurityConfig` 的文档路径放行逻辑和 `OpenApiProductionSecurityTest`。
  - 更新 parity matrix 的 OpenAPI / Swagger 行。
- 结果如何
  - dev/test 默认 `OPENAPI_ENABLED=true`，prod 默认 `OPENAPI_ENABLED=false`。
  - provider/router 测试覆盖启用时 `/openapi.yaml`、`/swagger/` 可访问。
  - provider/router 测试覆盖禁用时 `/openapi.yaml`、`/swagger/index.html` 返回 `COMMON-404`。
  - `make openapi-check` 可用于后续 CI 检查 generated file 是否与 YAML 漂移。

## 6. 已知限制
- 当前版本还缺什么
  - generated server interface 尚未接入当前 router，只作为生成、编译和未来适配基础。
  - Swagger UI HTML 使用外部 CDN，离线环境下 UI 静态资源可能加载失败；`/openapi.yaml` 不受影响。
  - 暂未引入 Spectral 或自定义 OpenAPI 风格规则。
  - 暂未在 CI 配置中固定执行 `make openapi-check`。
- 哪些地方后面需要继续演进
  - 新增业务接口时先更新 `api/openapi/eventhub.yaml`，再更新 handler/service。
  - 可逐步引入 generated interface adapter，减少 path/method 漂移。
  - 可增加规则检查，强制业务接口响应使用 `ApiResponse`，分页接口使用 `PageResponse`。
  - 如需生产临时文档，应单独设计内网访问、认证、审计和短期开关。
- 与 Java 版仍有哪些差距
  - Java 版由 Springdoc 注解扫描生成 `/v3/api-docs`；Go 版使用 spec-first YAML。
  - Java Swagger UI 路径为 `/swagger-ui.html` / `/swagger-ui/**`；Go 按任务要求使用 `/swagger/*`。
  - Java prod 未认证访问文档路径可能先返回 `AUTH-401`；Go prod 默认不注册文档路径，返回 `COMMON-404`。安全目标一致：文档资源默认不可访问。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 展示了后端接口契约、Swagger UI、生产暴露面控制、生成代码和质量门禁的完整工程闭环。
- 对微服务 / 云原生演进的影响
  - spec-first OpenAPI 可作为后续网关、前端 SDK、契约测试和服务拆分的输入。
  - prod 默认关闭文档入口，减少对外暴露面，符合云原生部署的默认安全要求。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续 API 变化需要同步 `eventhub.yaml`、生成代码、handler 测试和 parity matrix。
  - 数据库或 sqlc 变化不受本次影响。
  - CI 可新增 `make openapi-validate`、`make openapi-generate` 或 `make openapi-check`，防止 OpenAPI 漂移。
