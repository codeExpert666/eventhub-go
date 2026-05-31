# 错误响应契约 ADR

## 标题
Go 版 EventHub 使用 APIResponse 与 AppError 统一表达成功和失败响应

## 状态
- accepted

## 背景
Java 版 EventHub 已经通过 `ApiResponse<T>`、`ErrorCode`、`BusinessException` 和 `GlobalExceptionHandler` 建立了统一响应与错误码语义。

Go 版需要复刻这些外部契约：

- 成功响应使用 `COMMON-000` 和 `成功`。
- validation 错误使用 `COMMON-400`。
- 业务失败使用 `COMMON-401`。
- 资源不存在使用 `COMMON-404`。
- 未预期系统错误使用 `COMMON-500`。
- 认证、授权和账号冲突预留 `AUTH-401/AUTH-403/AUTH-409`。
- 响应体包含 `code/message/data/requestId/timestamp`。

Go 没有 Java 异常体系和全局 ControllerAdvice。若不提前固定错误响应契约，后续 handler/service 很容易分别返回不同 JSON 结构，破坏 Java-Go parity。

## 决策
Go 版 EventHub 使用以下错误响应契约：

- `internal/apperror.Code` 统一维护应用错误码、默认消息和 HTTP status。
- `internal/apperror.AppError` 表达可预期应用失败，后续业务层通过显式错误返回。
- `internal/http/response.APIResponse` 统一输出 `code/message/data/requestId/timestamp`。
- handler 层负责把 validation 或 `AppError` 映射为 HTTP 响应。
- recover middleware 只处理未预期 panic，并统一返回 `COMMON-500`。
- 如果 panic 发生时 HTTP 响应尚未提交，recover middleware 写出 `COMMON-500`；如果响应头或响应体已经提交，则只记录 panic 日志，不再追加统一错误体。
- 不用 `panic` 表达业务错误。

## 备选方案
- 方案 1：只使用 HTTP status，不返回应用错误码。
- 方案 2：每个 handler 自行组织错误 JSON。
- 方案 3：业务失败通过 panic/recover 统一处理。
- 方案 4：使用 `Code` + `AppError` + `APIResponse`。
- 方案 5：recover middleware 缓冲全部响应，panic 时总是改写为 `COMMON-500`。

## 决策理由
选择当前方案的原因：

- Java 版已经把错误码作为外部契约，Go 版必须保留机器可识别的 `code` 字段。
- `AppError` 与 Go 显式错误返回习惯一致，也能承接 Java `BusinessException` 的错误码和消息语义。
- 统一 `response` package 可以保证成功、失败、requestId 和 timestamp 字段一致，避免各 handler 重复拼装。
- recover middleware 的职责限制在系统兜底，能防止业务代码把正常失败伪装成 panic。
- HTTP 响应一旦提交就不能可靠改写状态码和响应体；此时继续追加 `COMMON-500` 会污染客户端响应，记录日志比返回损坏 JSON 更可控。
- validation 错误可以携带字段级 `data`，对齐 Java `GlobalExceptionHandler` 的字段错误 map。

## 影响
- 好处：
  - 前端、测试、日志和后续 OpenAPI 可以依赖稳定响应结构。
  - 后续业务模块只需返回 `AppError`，不必重复维护 HTTP status 和错误码映射。
  - panic 与业务失败边界清晰。
  - 已提交响应后的 panic 不会再把错误 JSON 拼接到原响应后面，降低客户端解析失败和误判风险。
- 代价：
  - handler 需要显式检查 validation 和 service 错误。
  - 已提交响应后的 panic 无法再对客户端表达标准 `COMMON-500`，只能依赖服务端日志排查。
  - 新增业务错误码时必须维护 `apperror.Code`、测试和 parity matrix。
- 后续可能需要调整的地方：
  - auth 模块落地后需要补齐 `AUTH-401/AUTH-403/AUTH-409` 的真实响应链路。
  - OpenAPI 引入后需要把 `APIResponse` 和错误响应模型固化到契约文件。
  - 如果后续字段级错误结构需要更丰富信息，可以在保持 `data` 外层语义稳定的前提下扩展内部模型。
  - 如果未来出现强需求，要求所有 panic 都能改写为统一错误体，需要单独评估响应缓冲、streaming 行为和内存成本。
