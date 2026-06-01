# ADR-0006: HTTP DTO 与 VO 边界

## 标题
ADR-0006: HTTP DTO 与 VO 边界

## 状态
- Accepted

## 背景
Java 项目中 `VO` 常用于响应对象或展示对象，但 Go 版如果照搬 `vo` 目录，容易与 DDD Value Object 混淆。

Go 项目已经有 `internal/http/response` 用于统一 `APIResponse` envelope 和 writer。后续 auth、user、event、reservation、order、ticket 会产生大量请求和响应结构体，需要在业务实现前先固化边界，避免业务 response DTO、统一响应工具和 domain value object 混用。

2026-06-02 的 HTTP 模块化组织规则进一步细化了 DTO 放置方式：HTTP DTO 仍属于 `internal/http/dto` 这条传输层边界，但具体业务 DTO 默认进入 `internal/http/dto/<module>` 子包，并在子包内优先拆分 `request.go` 与 `response.go`。

本决策对齐 Java 版 request DTO、response DTO 和 VO 的 HTTP 契约语义，但不复制 Java 命名习惯。

## 决策
- 不创建 `internal/http/vo`。
- HTTP 请求和响应结构体统一放在 `internal/http/dto/<module>` 子包下。
- `internal/http/dto/<module>` 承载对应模块的 JSON request body、query 参数对象、path 参数辅助对象、HTTP response data 对象、list item / summary / detail response 对象。
- DTO 子包内优先使用 `request.go` 与 `response.go` 分离请求和响应；复杂模块可继续按 use case 拆文件。
- `internal/http/response` 只放统一 `APIResponse` envelope 和 writer，例如 `Success`、`Failure`、`WriteSuccess`、`WriteError`、`WriteJSON`、`WriteStatus`。
- DDD Value Object 放 domain 层，例如 `internal/domain/common` 或 `internal/domain/<domain>`。
- service 不依赖 `internal/http/dto`，handler 负责 DTO 与 service Command / Query、service result / domain model 之间的映射。
- repository/mysql 负责 sqlc row 与 domain model 的映射，sqlc generated model 不作为 HTTP DTO 暴露。

## 备选方案
- 方案 1：创建 `internal/http/vo` 放响应对象。
- 方案 2：拆分 `internal/http/request` 和 `internal/http/response` 存业务请求/响应。
- 方案 3：将 request / response 统一放 `internal/http/dto` 根 package。
- 方案 4：将 request / response 放入 `internal/http/dto/<module>` 子包。

## 决策理由
- 避免 `VO` 语义混淆：Java View Object 在 Go 版归入 HTTP DTO，DDD Value Object 在 Go 版归入 domain。
- 避免业务 response 和统一 response writer 混淆：`internal/http/response` 专注 envelope 和 writer。
- 方便 OpenAPI schema 和 Java-Go API parity 管理：HTTP 请求和响应结构体集中在 `internal/http/dto`。
- 保持 service/domain 不依赖 HTTP：service 使用 Command / Query，domain model 不承担 HTTP JSON 契约职责。
- 更符合 Go 使用 package 边界表达职责的习惯：通过模块子包、文件名和类型后缀共同表达用途。
- 避免 `internal/http/dto` 根 package 随业务增长变成所有模块 DTO 的混合命名空间。

## 影响
- 好处：
  - 后续新增 auth/user/event/order/ticket 接口时，DTO 默认放 `internal/http/dto/<module>`。
  - 具体业务响应不会污染统一 response envelope / writer 包。
  - domain value object 不会被误放到 HTTP 层。
  - handler/service/repository/sqlc 边界更容易在 code review 和 Codex 生成代码时检查。
- 代价：
  - Go 版不会逐字照搬 Java `VO` 命名，需要在 parity matrix 中长期记录差异。
  - OpenAPI 或外部生成代码若产生 `DTO` 后缀，需要在设计文档中说明兼容理由。
- 后续可能需要调整的地方：
  - 如果未来出现强兼容需求，需要在设计文档和 ADR 中说明为什么偏离本规范。
  - 引入具体业务 DTO 后，implementation note 必须写明 DTO 与 service command/domain model 的映射关系。
  - 如果某个模块的 DTO 数量继续增长，可以在同一模块子包内从 `request.go` / `response.go` 演进为按 use case 拆分。
