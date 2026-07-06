# ADR-0025 OpenAPI Strict Server Runtime Router

## 标题
运行时业务与 actuator 路由和适用 HTTP 传输模型由 OpenAPI generated strict server 接管

## 状态
- accepted

## 背景
Go 版 EventHub 已经选择 spec-first OpenAPI，`api/openapi/eventhub.yaml` 是 API 契约源。此前 `oapi-codegen` 只生成 models 与 chi server interface，生产 router 仍在 `internal/http/router.go` 手写注册 `/api/**` 与 `/actuator/**`。

为了弥补 YAML 与手写 router 的双源维护风险，仓库曾新增 `internal/http/router_contract_test.go` 和 `internal/http/openapi_contract_test.go`。这些测试能发现漂移，但不能消除漂移的根因：运行时 path/method 仍由手写 route list 决定。

本次需要把 API 路径和方法的运行时事实来源收敛到 `api/openapi/eventhub.yaml` 及其 generated code，同时保留 Go 项目的 handler -> service -> repository -> sqlc/database 分层。

接入 strict server 后，generated code 已提供 request body、query/path 参数对象、typed success envelope 和统一 `ErrorResponse`。如果业务 handler 仍先把 generated request body 转为项目自有 HTTP DTO，再转 service Command，或先构造项目自有 `APIResponse{Data any}` 再转 generated `ApiResponseXxx`，HTTP 层会继续保留两套等价结构，增加转换噪音和契约漂移风险。

## 决策
选择使用当前固定的 `oapi-codegen v2.5.0` 生成 `models`、`chi-server` 和 `strict-server`。生成物仍属于 `api/openapi/gen` package；初始 strict-server 迁移时输出到 `api/openapi/gen/eventhub.gen.go`，后续由 ADR-0026 只调整物理文件布局，拆为 `models.gen.go` 和 `server.gen.go`，不改变本 ADR 的 runtime router 决策。

生产 router 改为：

- 创建 chi router 并注册应用级 middleware。
- 创建实现 `gen.StrictServerInterface` 的生产 strict adapter；该 adapter 作为 `internal/http` 内部生成接口适配层存在，不放入 `internal/http/handler/api` 非业务子包。
- 使用 generated `gen.NewStrictHandlerWithOptions` 和 `gen.HandlerWithOptions` 注册 `eventhub.yaml` 声明的业务与 actuator routes。
- 对 OpenAPI 已声明且 generated model 适用的业务 API，handler 直接校验 generated request model，并直接构造 generated typed response model。
- 删除与 generated model 同形的 `internal/http/dto/<module>` 镜像类型；未来只有 generated model 不适用或非 OpenAPI HTTP 面才新增 DTO，并在设计文档说明。
- 删除项目自有 `response.APIResponse`、`Success`、`Failure`、`WriteSuccess`、`WriteJSON`、`WriteStatus`；`internal/http/response` 保留 `SuccessMeta`、`ErrorMeta`、`ErrorBody` 和 `WriteError`。
- 将 `AppError` 的结构化上下文从 `data any` 改为 `details Details`，其中 `type Details map[string]any`；错误响应使用 generated `ErrorResponse.data` 输出 details。
- 继续在 router 中手写注册 OpenAPI / Swagger docs routes，因为它们受 `OPENAPI_ENABLED` 控制，不属于业务 API strict-server 范围。
- 继续由 router 统一设置 `NotFound` / `MethodNotAllowed` 错误响应。

删除旧补救型漂移测试：

- `internal/http/router_contract_test.go`
- `internal/http/openapi_contract_test.go`

保留 OpenAPI governance gates：

- `api/openapi/openapi_policy_test.go`
- `make openapi-lint`
- `make openapi-validate`
- `make openapi-check`
- `make openapi-breaking-check`

## 备选方案
- 方案 1：只新增编译期 adapter，继续用手写 router。
- 方案 2：继续保留手写 router 和两个补救型 contract tests。
- 方案 3：新增 `api/openapi/genstrict` 实验目录，双链路评估后再切换。
- 方案 4：直接把 OpenAPI request validation middleware 接入生产链路。
- 方案 5：使用 generated strict server 接管生产 router。
- 方案 6：使用 generated strict server 接管 router，但继续保留项目 HTTP DTO 和 `APIResponse{Data any}`。

## 决策理由
- 方案 5 直接消除手写业务 route list 与 OpenAPI paths/methods 并行维护的问题。
- `strict-server` 让 handler 层以 typed request/response object 适配 generated contract，同时 service/domain/repository 不依赖 generated model。
- 对 HTTP 边界而言，generated request/response model 已是 OpenAPI 契约的 Go 表达；保留同形 DTO 只会制造重复转换，不增加业务隔离。真正需要隔离的是 HTTP/generated model 与 service/domain/repository，这个边界仍由 handler 到 Command/Query/Result 的映射承担。
- `SuccessMeta` 抽出成功响应公共字段，避免每个 generated `ApiResponseXxx` 重复计算 `code/message/requestId/timestamp`，同时不恢复 `data any`。
- `WriteError` 留在 `internal/http/response`，因为 router、middleware 和 OpenAPI docs handler 无法通过 strict handler return value 写错，仍需要统一 writer。
- `Details map[string]any` 与 OpenAPI `ErrorResponse.data` 的 object 形态一致，比任意 `data any` 更能约束错误详情边界。
- 生成物继续限制在 `api/openapi/gen` package 内，避免实验目录和双链路维护成本；物理文件拆分只影响可读性，不改变 runtime route source-of-truth。
- docs routes 仍由 `OPENAPI_ENABLED` 控制，保持 prod 默认关闭文档入口的既有安全策略。
- OpenAPI lint/validate/check/breaking-check 仍各自承担不同质量边界，strict-server 不替代文档质量、结构合法性、生成漂移和跨分支兼容性检查。

## 影响
- 好处
  - `api/openapi/eventhub.yaml` 成为 API path/method 的契约源和运行时 source-of-truth。
  - 删除手写业务 route list 后，新增 API 必须先进入 spec，再经 generated route 注册进入生产 router。
  - 旧 router/spec 漂移测试不再承担补救职责，测试策略更聚焦。
- 代价
  - handler 层需要理解 generated strict request/response object。
  - handler 层对 generated model 的依赖更直接，因此必须防止 generated model 下沉到 service/domain/repository。
  - 非 OpenAPI HTTP 面如果未来出现，不能再随手复用已删除的 `APIResponse`，需要先说明契约来源和响应形态。
  - request body decode 由 generated strict handler 先执行，需要显式配置 request error handler 以保持既有错误 envelope。
  - auth/RBAC middleware 需要在 generated wrapper 外围按 OpenAPI security 语义编排，确保执行顺序不改变认证错误优先级。
- 后续可能需要调整的地方
  - 如果未来需要完整 incoming request validation，应单独设计 OpenAPI validation 层，并验证错误码、错误消息与 Java-Go parity。
  - 如果未来新增确实无法由 OpenAPI generated model 承载的 HTTP DTO，应在 design / implementation note 中说明它不是 generated model 的重复影子。
  - 如果新增 API version，例如 `/api/v2`，需要同步调整 breaking-check 策略和 generated router 使用方式。
  - 随业务模块增多，strict adapter 可以继续按模块拆分内部文件，但不应回到手写 route list，也不应把非业务聚合职责伪装成 `handler` 业务子包。
