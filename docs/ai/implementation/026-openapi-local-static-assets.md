# OpenAPI 本地静态资源实现说明

## 1. 本次改动解决了什么问题

本次将 Go 版 Swagger UI 从外部 CDN 和 Go 代码内嵌 HTML 改为本服务本地静态资源，同时将 `/openapi.yaml` 从 Go embed 内嵌字节改为读取部署目录中的本地 `api/openapi/eventhub.yaml` 文件。

这样 dev/test 显式暴露文档入口时不再依赖公网 CDN，企业内网、离线环境和 Docker 部署更可重复；prod 默认 `OPENAPI_ENABLED=false`、不注册文档入口并返回统一 `COMMON-404` 的行为保持不变。

## 2. 改动内容
- 新增了什么
  - 新增 `api/openapi/swagger/`，保存 `swagger-ui-dist@5.17.14` 的本地静态资源：
    - `index.html`
    - `swagger-ui.css`
    - `swagger-ui-bundle.js`
    - `LICENSE`
    - `NOTICE`
    - `README.md`
  - `README.md` 记录资源来源版本和更新命令。
- 修改了什么
  - `internal/http/handler/openapi/handler.go`：
    - Swagger UI HTML 改为读取本地 `swagger/index.html`，不再保存在 Go 字符串常量中。
    - `swagger/index.html` 只引用 `/swagger/swagger-ui.css` 和 `/swagger/swagger-ui-bundle.js`。
    - `/openapi.yaml` 改为从本地 asset root 读取 `eventhub.yaml`。
    - YAML、HTML、CSS、JS 统一进入 `staticAssets` 白名单表，公开 URL、资源根目录相对路径和 `Content-Type` 在同一处登记。
    - `YAML`、`SwaggerUI`、`SwaggerAsset` 复用同一个 `serveStaticAsset`，不再分别维护 YAML 和 Swagger UI 的读取 helper。
    - `NewOpenAPIHandler` 在构造阶段校验 `eventhub.yaml`、`swagger/index.html`、`swagger/swagger-ui.css`、`swagger/swagger-ui-bundle.js` 存在且不是目录。
    - 本地文件缺失或未知资源返回统一 `COMMON-404`。
    - `NewOpenAPIHandler` 改为接收配置传入的 asset root，删除构造时从工作目录 / 可执行文件目录自动查找资源根目录的逻辑。
  - `internal/config/config.go`：
    - `OpenAPIConfig` 新增 `AssetRoot` 字段。
    - 新增 `OPENAPI_ASSET_ROOT` 环境变量解析；未配置或空白时默认 `api/openapi`。
  - `internal/app/providers/http.go`：
    - 仅在 `OPENAPI_ENABLED=true` 时创建 `OpenAPIHandler`。
    - 创建时从 `platform.Config.OpenAPI.AssetRoot` 显式注入资源根目录。
    - `ProviderHTTP` 改为返回 `(HTTPDeps, error)`；OpenAPI 静态资源缺失时返回错误并不创建 router/server。
  - `internal/app/bootstrap.go`：
    - 包装 HTTP provider 错误为 `provide http dependencies: ...`，让启动期资源校验错误向上传递。
  - `api/openapi/assets.go`：
    - 移除 Go embed。
    - 仅保留 `AssetRoot`、`SpecPath`、`SwaggerDirPath`、`SwaggerIndexPath`、`SwaggerCSSPath`、`SwaggerBundlePath` 等资源路径常量。
    - 具体资源常量均表示相对于 `AssetRoot` / `OPENAPI_ASSET_ROOT` 的路径，调用方不再需要拿目录名和文件名自行拼接。
    - 删除测试专用的旧 `SpecYAML()` / `SpecPath()` 方法；当前 `SpecPath` 是资源路径常量，避免生产 package 继续承担测试便利职责。
  - `internal/http/openapi_contract_test.go`、`internal/http/router_contract_test.go`：
    - 各自新增本文件内的 OpenAPI 契约加载方法，使用 `runtime.Caller` 从测试文件位置定位 `api/openapi/eventhub.yaml`。
    - 继续复用 `api/openapi/assets.go` 中的资源路径常量，不新增共享 test helper。
  - `internal/http/router.go`：
    - `/swagger/` 和 `/swagger/index.html` 返回 HTML。
    - `/swagger/*` 返回本地 Swagger UI 静态资源。
  - `Dockerfile`：
    - 运行镜像复制 `api/openapi/eventhub.yaml` 和 `api/openapi/swagger/`，保证容器内显式开启文档入口时资源存在。
    - 默认声明 `OPENAPI_ASSET_ROOT=/app/api/openapi`。
  - `configs/*.env.example`、`docker-compose.yml`、`README.md`：
    - 记录 `OPENAPI_ASSET_ROOT`；本地 dev/test 使用 `api/openapi`，容器 / prod-like 环境使用 `/app/api/openapi`。
    - 补充说明相对路径按进程当前工作目录解析；使用本地默认值时应从仓库根目录启动。
    - 说明 `OPENAPI_ENABLED=true` 时会启动期校验资源存在，禁用时不校验。
  - `internal/http/router_test.go`、`internal/app/providers/http_test.go`：
    - 增加本地静态资源、CDN 禁止项、显式 asset root 注入、禁用时统一 404 的覆盖。
    - 测试夹具写入本地 YAML/HTML/CSS/JS 时复用 `api/openapi/assets.go` 中的根目录相对路径常量。
  - `docs/ai/adr/0019-prod-disable-api-docs.md`：
    - 更新文档入口启用时的资源来源说明。
- 删除了什么
  - 删除 Go embed 依赖，不再把 `eventhub.yaml` 编译进二进制用于运行时返回。
  - 删除 Go 文件中的 Swagger UI HTML 常量，避免入口 HTML 与其他静态资源管理方式不一致。
  - 删除生产包中专为测试提供的旧 `SpecYAML()` / `SpecPath()` 方法。
  - 删除 handler 中 `resolveDefaultAssetRoot`、`defaultAssetRootSearchStarts`、`findAssetRoot`、`hasOpenAPISpec` 等自动搜索逻辑。
  - 删除 handler 中区分 YAML 与 Swagger UI 的多套静态文件拼接 helper，改为统一资源表驱动。
- 是否更新 Java-Go parity 记录
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md` 的 OpenAPI / Swagger 行，记录 Go 使用本地静态资源、prod 默认关闭仍不注册文档路由。

## 3. 为什么这样设计
- 关键设计原因
  - 本地 vendored 文件能消除运行时 CDN 依赖，满足离线可用和企业网络可重复部署。
  - 只 vendoring Swagger UI 运行必需的 HTML/CSS/JS，避免引入 Node 构建链或暴露整个 npm 包目录。
  - `OpenAPIHandler` 继续是 HTTP 文档 handler，不引入 service/repository，也不改变业务分层。
  - `OPENAPI_ASSET_ROOT` 让资源根目录成为配置契约，composition root 负责注入，避免 HTTP handler 猜测运行目录。
  - `NewOpenAPIHandler(assetRoot string)` 让测试和 provider 都通过同一种显式构造方式传入目录，直接证明 `/openapi.yaml` 来自本地文件系统而不是 embed。
  - 契约测试各自加载 `eventhub.yaml`，让测试意图留在测试文件内；生产 `api/openapi` 包只对外提供运行时和测试都可复用的资源根目录相对路径常量。
  - 对 handler 来说 `/openapi.yaml` 和 `/swagger/*` 都是受控本地静态文件；统一表驱动让白名单、磁盘路径和 content type 一处维护，避免 helper 过度分裂。
  - 启动期校验符合企业部署可重复性的诉求：开启文档入口时，路径配置或资源复制错误应在进程启动阶段暴露，而不是等到首次请求才返回 404。
- 与 Go 项目当前阶段的匹配点
  - 保持 chi router 显式注册路由的现有方式。
  - 保持 `OPENAPI_ENABLED` 控制 handler 是否装配；禁用时静态资源路径根本不注册。
  - Dockerfile 显式复制静态资源并配置 `/app/api/openapi`，符合当前二进制 + 配置资产的部署模型。
- 与 Java 版业务语义的对齐方式
  - 继续对齐 Java Springdoc 非生产可用、生产默认关闭的安全目标。
  - 不复刻 Java 的 Springdoc/WebJar 自动资源机制；Go 端用本地文件和显式路由表达同等可访问语义。

## 4. 替代方案
- 方案 A：继续使用外部 CDN。
  - 没有采用，因为企业内网和离线部署不可重复，且本次明确要求移除 CDN。
- 方案 B：Swagger UI 和 `eventhub.yaml` 都使用 Go embed。
  - 没有采用，因为 `/openapi.yaml` 需要不再依赖内嵌方式；同时本地文件更便于部署侧审计和替换。
- 方案 C：引入 npm 构建，在 Docker build 时下载 `swagger-ui-dist`。
  - 没有采用，因为当前 Go 仓库没有前端构建链，引入 Node 会增加构建时间、网络依赖和供应链控制面。
- 方案 D：直接 `http.FileServer` 暴露 `api/openapi` 整个目录。
  - 没有采用，因为它会暴露 README/LICENSE 等非运行资源，且缺失文件时不是统一错误 envelope。
- 方案 E：handler 构造时自动搜索工作目录或可执行文件目录上方的 `api/openapi`。
  - 没有采用，因为这会把部署路径决策留在 HTTP handler 内，行为隐式，不如配置项适合企业部署审计。
- 方案 F：继续按 YAML、Swagger HTML、Swagger CSS/JS 分别保留 helper 和路径拼接。
  - 没有采用，因为这些资源在运行时都是静态文件，分开处理会让 handler 暴露多套不必要的路径规则。
- 方案 G：继续只在首次访问 `/openapi.yaml` 或 `/swagger/*` 时发现资源缺失。
  - 没有采用，因为它会把部署错误延迟到用户请求阶段；企业部署更需要启动期快速失败和明确日志。

## 5. 测试与验证
- 已跑测试
  - `go test ./internal/config ./internal/app/providers ./internal/http -run 'TestLoadOpenAPI|TestProviderHTTPRegistersOpenAPIWhenEnabled|TestOpenAPIEndpoints' -count=1`：本轮配置化 asset root 红灯后通过。
  - `go test ./internal/http ./internal/app/providers -run 'TestOpenAPI|TestProviderHTTP' -count=1`：通过。
  - `go test ./internal/http -run 'TestRouterContractRoutesMatchOpenAPISpec|TestOpenAPIResponseContractsValidateRealRouterResponses' -count=1`：本轮边界收口红灯后通过。
  - `go test ./internal/app ./internal/app/providers -run 'TestBootstrapWrapsOpenAPIAssetProviderErrors|TestProviderHTTP|TestRunWrapsHTTPServerErrors' -count=1`：本轮启动期校验红灯后通过。
  - `go test ./...`：通过。
- TDD 红灯记录
  - 本轮配置化 asset root 前，先补配置 / provider / router 测试，首次运行目标测试失败：
    - `cfg.OpenAPI.AssetRoot undefined`。
    - `unknown field AssetRoot in struct literal of type config.OpenAPIConfig`。
    - `too many arguments in call to openapihandler.NewOpenAPIHandler`。
  - 接入配置和显式构造后，router 测试先返回 `COMMON-404`，暴露测试里不能继续用包目录相对的 `api/openapi`；随后改为测试显式指向仓库根资源目录。
  - 本轮边界收口前，先把 `internal/http/openapi_contract_test.go` 和 `internal/http/router_contract_test.go` 改为调用各自的本地加载方法，首次运行目标测试失败：
    - `undefined: loadOpenAPIResponseContractDocument`。
    - `undefined: loadRouterContractOpenAPIDocument`。
  - 首次运行目标测试失败：
    - `NewOpenAPIHandlerWithAssetRoot` 未定义。
    - `/swagger/swagger-ui.css` 被旧 wildcard 返回为 `text/html`。
  - 第二轮将 HTML 抽成本地文件前，`TestOpenAPIEndpointsServeLocalStaticAssetsWhenEnabled` 失败，因为 `/swagger/` 仍返回 Go 常量中的 HTML，而不是临时 asset root 的 `swagger/index.html`。
  - 本轮表驱动收口前，先让测试改用 `SpecPath`、`SwaggerIndexPath`、`SwaggerCSSPath`、`SwaggerBundlePath` 等根目录相对路径常量，首次运行目标测试失败：
    - `undefined: openapispec.SpecPath`。
    - `undefined: openapispec.SwaggerDirPath`。
    - `undefined: openapispec.SwaggerIndexPath`。
    - `undefined: openapispec.SwaggerCSSPath`。
    - `undefined: openapispec.SwaggerBundlePath`。
  - 本轮启动期校验前，先把 provider/bootstrap 测试改为期望 `ProviderHTTP` 返回错误，并新增资源缺失用例，首次运行目标测试失败：
    - `assignment mismatch: 2 variables but ProviderHTTP returns 1 value`。
    - `TestBootstrapWrapsOpenAPIAssetProviderErrors` 期望资源缺失时启动失败，但旧实现未失败。
  - 全量测试首次运行时 `TestRunWrapsHTTPServerErrors` 失败，因为该测试只想验证端口占用错误，但默认 dev OpenAPI 开启导致先触发 asset root 相对路径校验；随后在该测试中显式设置 `OPENAPI_ENABLED=false`，隔离非目标依赖。
  - 实现后同一目标测试通过。
- 跑了哪些质量门禁
  - 已对改动 Go 文件运行 `gofmt`。
  - `go vet ./...`：通过。
  - `make openapi-check`：通过，`api/openapi/gen/eventhub.gen.go` 无漂移。
  - `make lint`：通过，`0 issues.`。
  - `docker compose config --quiet`：前序本地静态资源迁移已验证通过。
  - `make docker-build`：前序本地静态资源迁移已验证通过；首次遇到 Docker Hub frontend 拉取 EOF，重试后成功构建并复制 `/app/api/openapi/eventhub.yaml` 与 `/app/api/openapi/swagger`。
  - `git diff --check`：通过。
  - `git ls-files '*.go' | xargs gofmt -l`：无输出。
- 手工验证了哪些场景
  - 通过 router/provider `httptest` 验证：
    - `OPENAPI_ASSET_ROOT` 默认值为 `api/openapi`，显式设置时覆盖默认值，空白值回退默认值。
    - provider 启用 OpenAPI 时使用 `Config.OpenAPI.AssetRoot` 中的临时目录，不再依赖 handler 自动查找仓库目录。
    - 启用时 `/swagger/` HTML 不包含 `https://cdn`、`unpkg`、`jsdelivr`。
    - 启用时 `/swagger/` 返回临时 asset root 中的 `swagger/index.html` 内容，证明 HTML 不再内嵌在 Go 文件中。
    - 启用时 `/openapi.yaml` 返回临时本地文件内容。
    - 启用时 `/swagger/swagger-ui.css` 返回 `text/css`。
    - 启用时 `/swagger/swagger-ui-bundle.js` 返回 JavaScript content type。
    - YAML、HTML、CSS、JS 测试文件均按 `api/openapi/assets.go` 中的根目录相对路径写入，验证调用方不再自行拼接资源文件名。
    - 启用 OpenAPI 且 asset root 缺失时，`ProviderHTTP` 返回包含 `openapi asset root` 与缺失资源路径的错误，且不返回 router/server。
    - 禁用 OpenAPI 时，即使 asset root 指向不存在目录，也不执行资源校验，文档入口仍不注册并返回 `COMMON-404`。
    - Bootstrap 能把 OpenAPI 资源校验失败包装成 `provide http dependencies` 阶段错误。
    - 禁用时 `/openapi.yaml`、`/swagger/`、`/swagger/index.html`、CSS、JS 均返回 `COMMON-404`。
- Java-Go parity 如何验证
  - 对照 ADR-0019 和既有 parity matrix：prod 默认不注册文档入口的语义保持不变；差异只在 Go 文档资源由 CDN/embed 改为本地静态文件。
- 结果如何
  - 目标测试和完整质量门禁均通过。

## 6. 已知限制
- 当前只 vendoring Swagger UI 运行所需的 HTML/CSS/JS，没有提供 source map；浏览器开发者工具请求 map 时可能返回 404，但不影响 Swagger UI 离线使用。
- 本地静态资源依赖部署产物复制；Dockerfile 已覆盖容器路径，其他部署方式需要保留 `api/openapi/eventhub.yaml` 和 `api/openapi/swagger/`。
- 显式开启文档入口的部署必须保证 `OPENAPI_ASSET_ROOT` 指向包含 `eventhub.yaml` 和 `swagger/` 的目录；配置错误时会启动失败。
- 进程启动后如果资源文件被删除，请求期仍会按统一错误映射返回 `COMMON-404` 或内部错误；启动期校验不替代运行时文件系统错误处理。
- 当前未加 Cache-Control、ETag 或带 hash 的版本化资源路径，后续如前端缓存成为问题可单独设计。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - Swagger UI 在无公网环境中仍可使用，更符合企业部署演示和面试环境的不确定网络条件。
- 对微服务 / 云原生演进的影响
  - 后续拆服务时可继续把每个服务的 OpenAPI 契约和 Swagger UI 资源作为显式部署资产。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - OpenAPI 契约仍以 `api/openapi/eventhub.yaml` 为源，`make openapi-check` 继续作为漂移门禁。
  - Swagger UI 升级时按 `api/openapi/swagger/README.md` 的 npm pack 流程替换文件，并同步运行 `go test ./...` 和 `make openapi-check`。
