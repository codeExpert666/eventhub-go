# OpenAPI Request Contract Gate 实现说明

## 1. 本次改动解决了什么问题

本文记录 design 035 / ADR-0027 的阶段化落地。

阶段一已完成 `internal/http/validation` 到 `internal/http/requesterror` 的命名与职责收口：`requesterror` 只构造 HTTP request 相关 `AppError`，不执行运行时 OpenAPI request validation。

本次阶段二解决 runtime request contract gate 的启动期前置能力：

- `internal/config` 新增 `OPENAPI_REQUEST_VALIDATION_ENABLED` 与 `OPENAPI_SPEC_PATH`。
- `internal/http/contract` 新增只从文件系统加载 OpenAPI YAML 的 spec loader。
- provider 在 request validation 开启时加载、ResolveRefs 并 Validate `eventhub.yaml`；本阶段不接入 router middleware。
- 配置示例、Dockerfile、docker-compose 和 README 明确区分文档入口与 runtime request validation。

本次不改变对外 API、错误码、响应 envelope、JWT claim、业务 handler、service/domain/repository 或数据库行为。

## 2. 改动内容
- 新增了什么
  - `api/openapi.DefaultSpecPath`：runtime request contract gate 默认读取 `api/openapi/eventhub.yaml`。
  - `config.OpenAPIConfig.RequestValidationEnabled`：由 `OPENAPI_REQUEST_VALIDATION_ENABLED` 控制，dev/test 默认开启，prod 默认关闭。
  - `config.OpenAPIConfig.SpecPath`：由 `OPENAPI_SPEC_PATH` 控制，空白值回退默认 `api/openapi/eventhub.yaml`。
  - `internal/http/contract.LoadSpec` 与 `contract.Spec`：使用 `kin-openapi/openapi3.Loader` 从文件系统加载 spec，执行 ResolveRefs 和 Validate。
  - `HTTPDeps.RequestContract`：provider 层保存启动期构造完成的 OpenAPI contract spec，暂不传给 router。
  - 配置和 loader/provider 测试，覆盖默认值、显式覆盖、空白回退、缺失文件失败、非法 spec 失败。
- 修改了什么
  - `ProviderHTTP` 在 `RequestValidationEnabled=true` 时调用 `contract.LoadSpec`，失败则以 `initialize openapi request contract` 包装并阻止启动。
  - `configs/dev.env.example`、`configs/test.env.example`、`configs/prod.env.example` 增加新环境变量示例。
  - `Dockerfile` 与 `docker-compose.yml` 增加 prod-like 默认 `OPENAPI_SPEC_PATH=/app/api/openapi/eventhub.yaml`，并保持 request validation 默认关闭。
  - `README.md` 说明 `OPENAPI_ENABLED`、`OPENAPI_ASSET_ROOT`、`OPENAPI_REQUEST_VALIDATION_ENABLED`、`OPENAPI_SPEC_PATH` 的边界。
  - 少量 app 测试显式关闭 request validation，避免与非目标启动错误断言互相干扰。
- 删除了什么
  - 没有删除生产代码。
  - 没有恢复 Go embed，也没有恢复 `api/openapi/spec.go:SpecYAML()`。
- 是否更新 Java-Go parity 记录
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
  - 整体 request contract gate 仍是已决策、分阶段落地状态；阶段二的配置、文件系统 spec loader 与 provider 启动期校验已经实现，router middleware、violation 映射和 security requirement 迁移仍待后续阶段。

## 3. 为什么这样设计
- 关键设计原因
  - `OPENAPI_ENABLED` 只控制 `/openapi.yaml` 和 `/swagger/*` 文档入口；runtime request validation 使用独立 `OPENAPI_REQUEST_VALIDATION_ENABLED`，避免文档开关影响业务 API 安全边界。
  - `OPENAPI_SPEC_PATH` 直接指向 YAML 文件，和 `OPENAPI_ASSET_ROOT` 的“文档静态资源根目录”职责分开。
  - dev/test 默认开启 request validation，可以让本地与 CI 更早发现 spec 路径和契约合法性问题；prod 默认关闭，避免未显式部署 spec 路径时改变现有生产启动风险。
  - provider 在启动期加载和 validate spec，使配置错误、缺失文件或非法 OpenAPI 在服务接流量前失败。
  - `internal/http/contract` 是 HTTP 边界能力，不下沉到 handler/service/domain/repository。
- 与 Go 项目当前阶段的匹配点
  - 只新增 spec loader 和 provider 能力，不改 router middleware，不改变请求处理行为。
  - 继续复用既有 `kin-openapi` 依赖，不引入新重量级框架。
  - `LoadSpec` 使用文件系统路径，延续 OpenAPI 文档资产不 embed 的仓库决策。
- 与 Java 版业务语义的对齐方式
  - Java/Spring 通过 MVC 参数绑定、Bean Validation 和安全 filter chain 执行请求契约治理。
  - Go 端本阶段先建立 spec-first contract gate 的启动期基础；后续 middleware 阶段再执行 path/query/body/content-type/security 的运行时校验。

## 4. 替代方案
- 方案 A：prod 也默认开启 `OPENAPI_REQUEST_VALIDATION_ENABLED`。
- 方案 B：复用 `OPENAPI_ASSET_ROOT` 拼出 runtime spec 路径。
- 方案 C：把 `eventhub.yaml` Go embed 到二进制中供 loader 使用。
- 方案 D：本阶段直接把 contract middleware 接进 router。
- 为什么没有采用
  - 不采用方案 A：prod-like 环境需要显式确认部署资产路径，默认开启可能让未配置 spec 的现有部署启动失败。
  - 不采用方案 B：asset root 服务文档静态资源，spec path 服务 runtime contract gate；合并会让开关和路径语义继续缠在一起。
  - 不采用方案 C：与 ADR-0027 和既有 OpenAPI 本地静态资源决策冲突。
  - 不采用方案 D：本阶段目标是 loader/provider 能力，不提前扩大到 runtime 请求行为变化。

## 5. 测试与验证
- 跑了哪些测试
  - `go test ./internal/config ./internal/http/contract ./internal/app/providers -count=1`：通过。
  - `go test ./...`：通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt`：已对本次修改的 Go 文件执行。
  - `go vet ./...`：通过。
  - `git diff --check`：通过。
  - `sqlc generate`：未运行，本次不涉及 SQL、schema、repository 或 sqlc 配置。
  - OpenAPI validate / generate：未运行，本次不改变 OpenAPI YAML、生成配置或 generated code。
- 手工验证了哪些场景
  - 通过测试覆盖默认 dev/test 开启、prod 关闭、显式覆盖、空白路径回退。
  - 通过 loader 测试覆盖文件系统加载、ref value 可用、缺失文件失败、非法 spec Validate 失败。
  - 通过 provider 测试覆盖 request validation enabled 时加载 spec，spec 缺失时 provider 返回启动错误。
- Java-Go parity 如何验证
  - 主 parity matrix 已记录阶段二已落地能力和仍待后续阶段的 middleware/security/error mapping。
  - 本阶段没有改变 Java-Go 对外 API 契约，只推进 Go 端 OpenAPI contract governance 基础设施。
- 结果如何
  - 阶段目标测试和全量 Go 测试均已通过。

## 6. 已知限制
- 本阶段尚未把 request contract gate 接入 router middleware；运行时请求 path/query/body/content-type/security 校验仍保持当前 strict-server 与 handler 行为。
- `internal/http/contract` 目前只有 spec loader，后续还需要 middleware、body replay、violation mapper、security bridge 和角色扩展策略。
- `HTTPDeps.RequestContract` 当前只证明 provider 已构造并校验 spec；后续接 middleware 时再决定最终注入形状。
- prod 默认关闭 request validation，部署侧需要显式开启并提供 `OPENAPI_SPEC_PATH` 才会执行启动期 spec 校验和后续 runtime gate。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - OpenAPI 不再只是文档和代码生成源，已经具备向运行时 contract gate 演进的清晰启动期边界。
- 对微服务 / 云原生演进的影响
  - 显式文件系统 `OPENAPI_SPEC_PATH` 便于容器、Kubernetes ConfigMap/镜像资产、网关或 sidecar 共享同一份 spec。
  - prod-like 示例使用绝对路径，减少运行目录差异导致的部署不确定性。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续阶段应在 `internal/http/contract` 中继续添加 middleware 和 violation mapping，并保持 `requesterror` 只构造 `AppError`。
  - 接入 router 前需要补充真实 router 集成测试，验证 invalid path/query/body/content-type 不进入 strict handler。
  - service/domain/repository 继续不依赖 `api/openapi/gen`、`internal/http/contract` 或 `internal/http/requesterror`。
  - 不影响 migration、sqlc 或 OpenAPI generated code。
