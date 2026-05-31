# Web Router 选择 ADR

## 标题
Go 版 EventHub HTTP 路由层采用 chi

## 状态
- accepted

## 背景
Go 版 EventHub 需要建立 HTTP 工程底座，对齐 Java 版基础接口契约，同时保留 Go 生态自然写法。

本次需要支持：

- `GET /api/v1/system/ping`
- `POST /api/v1/system/echo`
- `GET /actuator/health`
- `HEAD /actuator/health`
- `GET /actuator/info`
- `HEAD /actuator/info`
- requestId middleware
- recover middleware
- 统一 NotFound / MethodNotAllowed 响应

后续用户、活动、场次、票种、订单、支付等模块会继续增加路由分组、路径参数和中间件组合。路由器选择会影响长期 package 边界、测试方式和 Java-Go parity 维护方式，因此需要 ADR 固化。

## 决策
Go 版 EventHub 采用 `github.com/go-chi/chi/v5` 作为 HTTP 路由层。

`chi` 只负责路由匹配、中间件编排和后续模块分组；统一响应、错误映射、validation、日志和业务分层由项目内部 package 控制。

## 备选方案
- 方案 1：标准库 `net/http` + `http.ServeMux`
- 方案 2：`github.com/go-chi/chi/v5`
- 方案 3：Gin、Echo、Fiber 等完整 Web 框架

## 决策理由
选择 `chi` 的原因：

- 与标准库 `http.Handler` 完全兼容，httptest 成本低，后续可以自然组合自定义 middleware。
- 相比 `ServeMux`，`chi` 在路由分组、路径参数、middleware 链和 NotFound/MethodNotAllowed 定制上更适合后续业务模块扩展。
- 相比 Gin/Echo/Fiber，`chi` 更轻量，不强迫项目采用框架上下文、响应模型或错误模型，便于保持 `AppError` 和 `APIResponse` 的项目级契约。
- Java 版的 Spring MVC 能力不应逐行迁移；Go 版只需要保留 API 路径、HTTP 方法、响应字段和错误语义。
- `chi` 的依赖面较小，适合作为当前工程底座阶段的第一项 Web 依赖。
- Actuator HEAD 契约可以通过 `router.Head` 显式注册，避免把 HEAD 隐式映射到 GET 后在测试中写出响应体，也让方法级公开边界更清楚。

## 影响
- 好处：
  - 后续可以按业务模块组织路由，保持 handler 层清晰。
  - requestId、recover、认证、权限、访问日志等 middleware 可以逐步叠加。
  - 测试仍使用标准库 `httptest`，不绑定框架专用测试工具。
- 代价：
  - 引入了一个第三方依赖，需要通过 `go.mod/go.sum` 管理。
  - 团队需要遵守项目内部响应和错误约定，避免直接使用 router 层写散乱响应。
- 后续可能需要调整的地方：
  - 如果未来需要 OpenAPI-first 路由生成，可能重新评估 router 与生成代码的边界。
  - 如果微服务拆分后各服务技术栈不同，需要在服务模板层复用或替换该选择。
