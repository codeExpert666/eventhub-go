# OpenAPI 本地静态资源设计

## 1. 背景
- 当前 Go 版 `internal/http/handler/openapi/handler.go` 返回最小 Swagger UI HTML，HTML 通过外部 CDN 加载 `swagger-ui-dist@5.17.14` 的 CSS 与 JS。
- 当前 `/openapi.yaml` 曾由 `api/openapi/spec.go` 通过 Go embed 内嵌后返回，不依赖 CDN，但运行时内容固定在编译产物中。
- 企业部署更关注可重复性和离线可用性：容器、内网和受限环境不应在访问 Swagger UI 时依赖公网 CDN；OpenAPI 契约也应作为部署资产显式存在，便于审计和替换。
- Java 版通过 Springdoc 暴露 API 文档，并在 prod profile 默认关闭；Go 版已通过 `OPENAPI_ENABLED` 和 ADR-0019 对齐“prod 默认不注册文档入口”的安全语义。

## 2. 目标
- 将 Swagger UI 运行时依赖的前端文件和入口 HTML vendoring 到仓库本地，保持版本来源可追溯。
- Swagger UI HTML 入口文件仅引用本服务路径：
  - `/swagger/swagger-ui.css`
  - `/swagger/swagger-ui-bundle.js`
- 修改 `/openapi.yaml` 返回逻辑，从本地静态文件 `api/openapi/eventhub.yaml` 读取，不再依赖 Go embed。
- 新增 `OPENAPI_ASSET_ROOT` 配置项，显式声明 OpenAPI 静态资源根目录：
  - 未配置时默认 `api/openapi`，保持本地开发从仓库根目录启动的便利性。
  - Docker / prod-like 部署可显式设置为 `/app/api/openapi`，避免 handler 在运行时猜测工作目录或可执行文件目录。
- `OPENAPI_ENABLED=true` 时在 HTTP provider / handler 构造阶段校验静态资源是否存在，配置错误或资源缺失时启动失败，而不是等到访问 `/swagger/` 或 `/openapi.yaml` 时才返回 404。
- 新增或调整路由，使 `/swagger/` 返回 HTML，`/swagger/*` 下的本地静态文件返回对应资源。
- 保持 `OPENAPI_ENABLED=false` 时不注册 `/openapi.yaml`、`/swagger/` 和 Swagger UI 静态资源路径，继续返回统一 `COMMON-404`。
- Docker 运行镜像复制 OpenAPI 静态资源目录，保证显式开启文档入口时容器内也能离线访问。

## 3. 非目标
- 不修改业务 API 路径、请求字段、响应字段、错误码、分页语义或 OpenAPI 契约内容。
- 不引入 Node、npm、前端构建流水线或运行时 CDN fallback。
- 不改 `OPENAPI_ENABLED` 默认规则：dev/test 默认开启，prod 默认关闭。
- 不把 OpenAPI 静态资源根目录配置做成必填项；本地开发仍保留默认值。
- 不把业务 handler 改造成 generated server interface。
- 不引入 OpenAPI 请求校验 middleware。

## 4. 影响范围
- Go package / 模块：
  - `api/openapi/assets.go`：移除 embed 依赖，仅保留 OpenAPI 资源根目录，以及相对于该根目录的 YAML / Swagger UI 静态资源路径常量。
  - `api/openapi/swagger/`：新增 Swagger UI 本地 HTML、CSS、JS 静态资源。
  - `internal/http/handler/openapi`：通过统一静态资源白名单表从本地文件系统读取 YAML、HTML、CSS 与 JS。
  - `internal/config`：新增 `OpenAPIConfig.AssetRoot` 与 `OPENAPI_ASSET_ROOT` 解析，默认值使用本地资源常量。
  - `internal/app/providers/http.go`：继续只在 `OPENAPI_ENABLED=true` 时装配 handler，并从 `platform.Config.OpenAPI.AssetRoot` 显式注入资源根目录；资源缺失时返回 provider 错误并阻止启动。
  - `internal/app/bootstrap.go`：接收并包装 HTTP provider 错误，保证启动期配置问题能向上传递。
  - `internal/http/router.go`：保持 `/openapi.yaml`、`/swagger/`、`/swagger/index.html`、`/swagger/*` 的文档入口路由注册边界。
  - `internal/http/openapi_contract_test.go`、`internal/http/router_contract_test.go`：各自从本测试文件定位 `api/openapi/eventhub.yaml`，不再依赖生产包中的测试专用读取方法。
  - `Dockerfile`：复制 `api/openapi` 到运行镜像，并显式声明 runtime 资源根目录。
  - `configs/*.env.example`：记录 `OPENAPI_ASSET_ROOT` 默认或部署建议值。
  - `internal/config/config_test.go`、`internal/http/router_test.go`、`internal/app/providers/http_test.go`：补齐配置默认值、显式注入、启用和禁用场景。
- API / 外部接口：
  - 文档入口路径不新增业务契约；`/swagger/index.html`、`/swagger/swagger-ui.css` 和 `/swagger/swagger-ui-bundle.js` 仅在文档入口启用时存在。
- 数据、缓存、外部服务：
  - 不涉及数据库、Redis、migration 或 sqlc。
  - 移除浏览器访问 Swagger UI 时的公网 CDN 依赖。
- `docs/ai/parity/java-go-parity-matrix.md`：需要更新 OpenAPI / Swagger 行，记录 Go 版本地静态资源与 Java Springdoc 的实现差异。

## 5. 领域建模
- `OpenAPIContract`
  - 物理文件：`api/openapi/eventhub.yaml`。
  - 作为 spec-first 契约源继续存在；运行时通过本地文件读取，不再编译进二进制。
  - 测试需要加载契约时，由对应测试文件使用 `runtime.Caller` 定位仓库内文件，避免把测试便利方法放回生产 package。
- `OpenAPIAssetRoot`
  - 配置来源：`OPENAPI_ASSET_ROOT`。
  - 默认值：`api/openapi`。
  - 运行时使用者：`internal/app/providers/http.go` 在启用 OpenAPI 文档入口时把该值注入 `OpenAPIHandler`。
  - 约束：handler 只基于传入根目录拼接白名单内资源，不再向上搜索目录；相对路径相对于进程当前工作目录，因此开发 / 测试配置文件需要说明从仓库根目录启动，或改由启动命令注入绝对路径。
  - 启用校验：`OPENAPI_ENABLED=true` 时必须能读取 `eventhub.yaml`、`swagger/index.html`、`swagger/swagger-ui.css`、`swagger/swagger-ui-bundle.js`，否则 HTTP provider 返回错误。
- `OpenAPIStaticAsset`
  - 物理根目录：`OPENAPI_ASSET_ROOT`，本地默认 `api/openapi`。
  - YAML 资源路径：`eventhub.yaml`。
  - Swagger UI 资源路径：`swagger/index.html`、`swagger/swagger-ui.css`、`swagger/swagger-ui-bundle.js`。
  - `api/openapi/assets.go` 对外暴露这些相对于资源根目录的路径，调用方无需再拿目录名和文件名自行拼接。
  - 与 Java Springdoc 内置 WebJar / static resource handler 的语义对应，但 Go 端用仓库 vendored 文件和显式白名单表管理。
- `OpenAPIHandler`
  - 仍是 HTTP 文档入口 handler，不访问 service、repository、database 或安全上下文。
  - 持有由配置传入的 asset root，测试可注入临时目录验证本地文件读取行为。
  - 不负责解析环境变量、查找工作目录或猜测可执行文件位置。
  - 不区分 “YAML handler” 与 “Swagger UI asset handler” 的文件读取逻辑；所有公开文档 URL 都先查询同一张静态资源表，再复用同一套路径清洗、文件存在性检查和统一错误映射。

## 6. API 设计
- `GET /openapi.yaml`
  - 仅 `OPENAPI_ENABLED=true` 时注册。
  - 成功返回 `200`，`Content-Type: application/yaml; charset=utf-8`。
  - 响应体来自 `${OPENAPI_ASSET_ROOT}/eventhub.yaml`，未配置时等价于 `api/openapi/eventhub.yaml`。
  - 文件缺失时返回统一 `COMMON-404`，避免泄露运行目录细节。
- `GET /swagger`
  - 仅 `OPENAPI_ENABLED=true` 时注册。
  - 重定向到 `/swagger/`。
- `GET /swagger/` 和 `GET /swagger/index.html`
  - 返回本地文件 `${OPENAPI_ASSET_ROOT}/swagger/index.html`，未配置时等价于 `api/openapi/swagger/index.html`。
  - HTML 中只引用本服务路径，不包含 `https://cdn`、`unpkg`、`jsdelivr` 等外部 CDN 地址。
- `GET /swagger/swagger-ui.css`
  - 返回本地 CSS，`Content-Type` 包含 `text/css`。
- `GET /swagger/swagger-ui-bundle.js`
  - 返回本地 JS，`Content-Type` 包含 `javascript`。
- 静态资源映射：
  - `/openapi.yaml` -> `eventhub.yaml`
  - `/swagger/` -> `swagger/index.html`
  - `/swagger/index.html` -> `swagger/index.html`
  - `/swagger/swagger-ui.css` -> `swagger/swagger-ui.css`
  - `/swagger/swagger-ui-bundle.js` -> `swagger/swagger-ui-bundle.js`
  - handler 只服务表中登记的资源；未知 `/swagger/*` 路径返回统一 `COMMON-404`。
- 禁用场景：
  - `/openapi.yaml`、`/swagger/`、`/swagger/swagger-ui.css`、`/swagger/swagger-ui-bundle.js` 均不注册，落入 router 统一 `COMMON-404`。
- 配置项：
  - `OPENAPI_ENABLED`：控制是否注册文档入口，既有语义不变。
  - `OPENAPI_ASSET_ROOT`：控制文档入口启用后从哪个本地目录读取 `eventhub.yaml` 与 `swagger/` 静态资源；空白或未设置时回退到 `api/openapi`。
  - 当 `OPENAPI_ASSET_ROOT` 为相对路径时，它相对于进程当前工作目录；本地 dev/test 示例值 `api/openapi` 只适用于从仓库根目录启动的场景。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Java 使用 Springdoc / Swagger UI 自动资源处理；Go 使用 vendored 静态文件和 chi router 显式路由。
  - Go 保持 `/openapi.yaml` 与 `/swagger/*` 路径，不复刻 Java `/v3/api-docs` 和 `/swagger-ui.html`。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 本次只涉及静态文件与生成产物一致性。
  - `make openapi-check` 继续验证 OpenAPI 契约合法性和 generated file 漂移。

## 8. 关键流程
- 正常流程：
  1. `config.Load()` 计算 `OpenAPI.Enabled`。
  2. `config.Load()` 读取 `OPENAPI_ASSET_ROOT`，未配置时使用 `api/openapi`。
  3. `ProviderHTTP` 仅在启用时使用配置中的 asset root 创建 `OpenAPIHandler`。
  4. `NewOpenAPIHandler` 校验白名单内所有本地静态资源存在且不是目录。
  5. 校验通过后 `NewRouter` 注册 `/openapi.yaml`、`/swagger`、`/swagger/`、`/swagger/*`。
  6. handler 根据请求 URL 查询静态资源白名单表，得到资源根目录下的相对路径和响应 `Content-Type`。
  7. `/swagger/` 返回本地 `swagger/index.html`。
  8. 浏览器请求 `/swagger/swagger-ui.css`、`/swagger/swagger-ui-bundle.js`。
  9. Swagger UI 请求 `/openapi.yaml`，handler 从同一张资源表读取 `eventhub.yaml`。
- 异常流程：
  - OpenAPI 禁用时，所有文档路径不注册，统一返回 `COMMON-404`。
  - `OPENAPI_ASSET_ROOT` 指向不存在的目录或缺少文件时，HTTP provider 返回启动错误；Bootstrap 包装为 `provide http dependencies: ...`，避免把错误延迟到首次 HTTP 请求。
  - 未登记的 `/swagger/*` 静态文件不尝试拼接磁盘路径，直接返回统一 `COMMON-404`。
- 状态流转：
  - 不涉及业务状态机，仅涉及文档入口 enabled/disabled 配置状态。
- handler / service / repository / sqlc/database 分工：
  - OpenAPI handler 只处理 HTTP 静态资源输出。
  - config 只负责从环境变量解析结构化配置。
  - provider 作为 composition root 负责把配置注入 handler。
  - handler 构造函数只校验自身要服务的静态资源，不解析环境变量，不搜索项目根目录。
  - 不新增 service、repository 或 sqlc query。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无。
- 如何防重复提交：无，本次没有写业务操作。
- 事务边界在哪里：无数据库事务。
- 缓存放在哪里，为什么：
  - 不引入 Redis 或业务缓存。
  - 由浏览器和标准 HTTP 静态文件响应处理基础缓存语义；如后续需要强缓存、ETag 或版本化路径，可在文档入口稳定后单独设计。

## 10. 权限与安全
- 哪些角色能访问：
  - dev/test 默认任何调用方可访问文档入口。
  - prod 默认任何调用方不可访问，因为路由不注册。
  - 管理员 token 不能绕过 prod 默认关闭语义。
- 鉴权与鉴别约束：
  - 文档入口是否存在由 `OPENAPI_ENABLED` 决定，不走 auth middleware。
- JWT claim 边界：
  - 不修改 JWT，不新增 claim。
- 敏感信息、审计或操作日志：
  - OpenAPI 文档仍会暴露接口契约，因此 prod 默认关闭必须保持。
  - `OPENAPI_ASSET_ROOT` 只是本地路径配置，不应指向包含额外敏感文件的目录；handler 白名单仍限制只暴露 `eventhub.yaml` 和登记的 Swagger UI 文件。
  - YAML、HTML、CSS、JS 共用一张公开路径白名单，减少新增资源时遗漏路径清洗或错误映射的概率。
  - 启动期校验会暴露本地路径到启动错误日志，但该错误只面向部署 / 运维侧，不作为 HTTP 响应返回给外部客户端。
  - 本次减少公网资源依赖，降低受限网络和供应链漂移风险。

## 11. 测试策略
- 单元 / router 测试：
  - `config.Load()` 默认 `OpenAPI.AssetRoot == api/openapi`。
  - `OPENAPI_ASSET_ROOT` 显式配置时覆盖默认值，空白值回退默认值。
  - 启用时 `/swagger/` 返回 HTML，且不包含 `https://cdn`、`unpkg`、`jsdelivr`。
  - 启用时 `/swagger/` 返回临时 asset root 中的本地 `swagger/index.html` 内容，验证入口 HTML 不再内嵌在 Go 代码中。
  - 启用时 `/openapi.yaml` 返回本地静态文件内容；通过临时 asset root 验证不依赖 embed。
  - 启用时 `/swagger/swagger-ui.css` 和 `/swagger/swagger-ui-bundle.js` 返回 200 与合理 `Content-Type`。
  - 测试夹具通过 `api/openapi/assets.go` 中的根目录相对路径常量写入 YAML/HTML/CSS/JS，避免测试再次自行拼接 Swagger 子目录与文件名。
  - 禁用时 `/openapi.yaml`、`/swagger/`、`/swagger/swagger-ui.css`、`/swagger/swagger-ui-bundle.js` 均返回 `COMMON-404`。
- provider 测试：
  - 继续覆盖 `OPENAPI_ENABLED=true/false` 对路由注册的影响，并补充静态资源路径。
  - 启用时 provider 必须使用 `Config.OpenAPI.AssetRoot` 注入 handler；用临时目录内容验证不再由 handler 自动搜索仓库目录。
  - 启用时 asset root 缺失、缺少 `eventhub.yaml` 或缺少 Swagger UI 文件应返回 provider 错误，不创建 router/server。
  - 禁用时即使 asset root 无效，也不校验资源、不注册文档入口。
- migration / sqlc 验证：
  - 不适用，本次无数据库变化。
- 接口验证：
  - `go test ./...` 覆盖 handler/router/provider 行为。
- OpenAPI validate：
  - `make openapi-check` 继续覆盖契约校验、生成与 generated file 漂移。
- Java-Go parity 验证：
  - 对照 Java prod 文档关闭语义和 Go ADR-0019，确认 prod 默认关闭行为不变。
  - 契约测试加载方式属于 Go-only 测试结构调整，不改变 Java-Go 业务语义；parity matrix 只需保持 OpenAPI / Swagger 行的 Go 目标文件索引准确。
- 需要运行的命令：
  - `gofmt`。
  - `go test ./...`。
  - `go vet ./...`。
  - `make openapi-check`。
  - 如可用，运行 `make lint` 或 `golangci-lint run`。

## 12. 风险与替代方案
- 当前方案的风险：
  - 本地静态资源需要随部署产物一起复制；Dockerfile 必须显式包含 `api/openapi`。
  - 部署侧如果配置了错误的 `OPENAPI_ASSET_ROOT`，显式开启文档入口时应用会启动失败；这更早暴露错误，但也要求 dev/test 从仓库根目录启动或注入绝对路径。
  - 相比 embed，本地文件在运行时可能缺失，因此启用文档入口时需要启动期校验；如果资源在进程启动后被删除，handler 仍按统一错误映射处理。
  - vendored 前端文件会增加仓库体积。
- 备选方案：
  - 方案 A：继续 CDN。离线和企业网络不可重复，不满足本次验收。
  - 方案 B：把 Swagger UI 和 OpenAPI YAML 全部 Go embed。离线可用，但与“openapi.yaml 不再依赖内嵌方式”的要求冲突，也不利于部署侧替换静态契约文件。
  - 方案 C：引入 npm 构建，在镜像构建时下载 `swagger-ui-dist`。可自动化更新，但增加 Node 工具链、网络下载和构建复杂度。
  - 方案 D：用 `http.FileServer` 直接暴露整个目录。实现更少，但缺失文件时默认响应不是统一 envelope，也更容易暴露不希望公开的说明文件。
  - 方案 E：handler 构造时自动从工作目录、可执行文件目录向上查找 `api/openapi`。本地启动方便，但部署行为隐式，职责落在 HTTP handler 内，不利于企业部署审计。
  - 方案 F：继续按 YAML、Swagger HTML、Swagger CSS/JS 分别保留 helper 和拼接逻辑。可读性在文件少时尚可，但会让“静态文件服务”被拆成多套路径补齐规则。
  - 方案 G：继续只在首次请求时发现资源缺失并返回 `COMMON-404`。对客户端响应稳定，但部署错误暴露太晚。
- 为什么不选备选方案：
  - 当前阶段最需要的是小范围、可审计、离线可运行的文档入口；vendored 指定文件 + 显式配置注入足以满足需求。
  - 不继续使用 handler 自动搜索，是为了让资源根目录成为应用配置契约，而不是 handler 对运行目录的隐式猜测。
  - 不继续保留多套 helper，是因为 YAML 和 Swagger UI 在运行时都是受控静态文件；统一表驱动能让公开 URL、磁盘相对路径和 Content-Type 一处登记。
  - 不继续保留请求期才发现缺失资源，是因为企业部署更需要启动期失败和明确日志，便于在发布阶段发现错误路径或漏复制资源。
- 后续可演进点：
  - 后续可增加资源 hash、ETag、Cache-Control 或版本化路径。
  - Swagger UI 升级时通过实现说明记录来源版本、下载命令和文件清单。
