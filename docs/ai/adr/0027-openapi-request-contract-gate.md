# ADR-0027 OpenAPI Request Contract Gate

## 标题
使用显式文件系统 OpenAPI spec 和独立 contract gate 执行运行时请求契约校验

## 状态
- accepted

## 背景
Go 版 EventHub 已经采用 spec-first OpenAPI，并通过 `oapi-codegen` 生成 models、chi server 和 strict-server。strict-server 已成为生产业务与 actuator route 的运行时 source-of-truth，但它主要负责 route registration、typed request/response object、JSON body decode 和 generated response writer，不是完整 incoming request validation 引擎。

当前 HTTP 入参错误构造位于 `internal/http/validation`，但该包的真实职责是把 malformed body、body 字段错误和 path/query 参数错误构造成 `AppError`。如果新增 `internal/http/requestvalidation`，两个包名都会表达 validation，读者难以判断哪一个真正执行 request validation。

另外，OpenAPI 文档入口已在 ADR-0019 和 design/implementation 026 中从 Go embed 改为本地静态资源与显式部署资产。request validation 需要加载 OpenAPI spec，但不能借此恢复旧的 embed 读取方式，也不能让 `OPENAPI_ENABLED` 这个文档入口开关控制业务 API 的 runtime contract gate。

## 决策
采用以下决策：

- 将 `internal/http/validation` 重命名为 `internal/http/requesterror`。
  - 该包只负责构造 HTTP request 相关 `AppError`，不执行真正校验。
  - 推荐函数名使用 `MalformedBody`、`InvalidBody`、`InvalidParameters`、`UnsupportedContentType` 等错误语义，而不是 `XxxValidationError`。
- 新增 `internal/http/contract`。
  - 该包负责 OpenAPI request contract gate：加载 spec、匹配 operation、校验 path/query/header/cookie/body/content-type/security requirement、body replay 和 violation -> `AppError` 映射。
  - 使用 `kin-openapi/openapi3filter` 作为 OpenAPI 语义执行核心，但项目持有 middleware、错误 envelope、security bridge 和 extension policy。
- 新增 `OPENAPI_SPEC_PATH`。
  - 用于 runtime request contract gate 从文件系统加载 `eventhub.yaml`。
  - 默认可指向 `api/openapi/eventhub.yaml`，Docker/prod-like 部署建议使用 `/app/api/openapi/eventhub.yaml`。
  - 不使用 Go embed，不恢复 `SpecYAML()`。
- 新增 `OPENAPI_REQUEST_VALIDATION_ENABLED`。
  - 控制 runtime request contract gate 是否启用。
  - 与 `OPENAPI_ENABLED` 独立；后者只控制 `/openapi.yaml` 和 `/swagger/*` 文档入口是否注册。
- 将 security requirement 迁入 OpenAPI contract gate。
  - operation `security` 中的 `BearerAuth` 表示认证要求。
  - `x-required-roles: [ADMIN]` 表示管理员角色要求。
  - JWT 不携带角色、邮箱、用户名或用户状态；动态权限仍由服务端查询或受控缓存获得。

## 备选方案
- 方案 1：直接使用 `github.com/oapi-codegen/nethttp-middleware` 默认 middleware。
- 方案 2：继续保留 `internal/http/validation`，新增 `internal/http/requestvalidation`。
- 方案 3：把 `eventhub.yaml` 重新 Go embed 到二进制中，供 request validation 使用。
- 方案 4：继续只依赖 strict-server 和 handler 内手写字段校验。
- 方案 5：把 OpenAPI request validation 分散到各业务 handler。

## 决策理由
- 项目自有 `contract` 包可以保持企业级边界：
  - 统一错误 envelope。
  - 中文业务错误消息与稳定 `data.violations`。
  - `AUTH-401` / `AUTH-403` 映射。
  - `x-required-roles` 扩展。
  - body replay 和 body size policy。
  - 后续审计、指标、灰度开关。
- `requesterror` 比 `validation` 更准确。它表达的是“HTTP request 错误构造”，不是校验执行器。
- `contract` 比 `requestvalidation` 更准确。目标能力不只是字段 validation，还包括 operation matching、content-type、security requirement、OpenAPI extension policy。
- 显式 `OPENAPI_SPEC_PATH` 延续本仓库本地静态资源 / 部署资产方向，避免恢复 embed，也避免把文档入口开关和 runtime API 安全边界混在一起。
- OpenAPI security metadata 成为 runtime security source 后，可以移除基于 path 前缀的 ADMIN 判断，降低手写安全规则与 spec 漂移风险。

## 影响
- 好处
  - request contract 执行集中化，OpenAPI 不再只是生成与文档源。
  - 命名边界更清楚：`requesterror` 构造错误，`contract` 执行契约。
  - security requirement 与 `x-required-roles` 可被 runtime 和 policy test 同时约束。
  - 不恢复 embed，保持部署资产可审计、可替换。
- 代价
  - 需要新增配置、provider 装配和 contract middleware 测试。
  - 运行时会比当前 strict-server 更严格，可能暴露现有 spec 与 handler 容忍行为之间的差异。
  - body replay 需要谨慎处理内存上限和错误映射。
- 后续可能需要调整的地方
  - 如果 API 规模扩大，可将 `contract` 内部的 violation mapper、security bridge 和 body reader 拆成更细文件。
  - 如果发现未知 query/header/cookie 策略需要灰度，可增加 warn/reject 模式。
  - 如果后续引入网关层 request validation，本服务内 contract gate 可保留为 defense-in-depth 或按环境关闭。
