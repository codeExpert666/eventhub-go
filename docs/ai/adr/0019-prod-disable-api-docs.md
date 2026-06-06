# ADR：生产环境默认关闭 API 文档入口

## 标题
prod 默认不注册 `/openapi.yaml` 和 `/swagger/*`

## 状态
- accepted

## 背景
Java 版在非生产环境默认开启 Springdoc OpenAPI JSON 和 Swagger UI，方便联调和学习演示；在 `application-prod.yml` 中默认关闭 `springdoc.api-docs.enabled` 和 `springdoc.swagger-ui.enabled`，避免生产环境暴露接口路径、请求响应模型、枚举和错误码等契约信息。Java `OpenApiProductionSecurityTest` 还验证：生产环境不仅未认证访问拿不到文档，携带管理员 token 也不应取得文档资源。

Go 版本次新增 `/openapi.yaml` 和 `/swagger/*`。关键决策是：prod 下应该认证后允许访问、只允许 ADMIN 访问，还是默认完全不注册文档入口。

## 决策
Go 版选择：

- 新增 `OPENAPI_ENABLED` 环境变量。
- `EVENTHUB_ENV=dev` 和 `EVENTHUB_ENV=test` 时，`OPENAPI_ENABLED` 未配置默认 `true`。
- `EVENTHUB_ENV=prod` 时，`OPENAPI_ENABLED` 未配置默认 `false`。
- `OPENAPI_ENABLED` 显式配置时覆盖环境默认值。
- `ProviderHTTP` 根据 `platform.Config.OpenAPI.Enabled` 决定是否创建 `OpenAPIHandler`。
- `NewRouter` 只在存在 `OpenAPIHandler` 时注册：
  - `GET /openapi.yaml`
  - `GET /swagger`
  - `GET /swagger/`
  - `GET /swagger/*`
- prod 默认关闭时，文档路径不注册，请求落入统一 `COMMON-404`。
- prod 默认关闭不受管理员 token 影响；文档资源不存在，而不是认证后可见。

## 备选方案
- 方案 1：所有环境默认开放文档入口。
- 方案 2：prod 下开放 `/openapi.yaml`，只关闭 Swagger UI。
- 方案 3：prod 下文档入口要求登录。
- 方案 4：prod 下文档入口要求 ADMIN。
- 方案 5：prod 默认不注册文档入口，需要时通过 `OPENAPI_ENABLED=true` 显式开启。

## 决策理由
选择方案 5，原因是：

- 对齐 Java prod 关闭 Springdoc 资源本身的核心安全语义。
- OpenAPI 会暴露接口路径、字段、枚举、错误码和鉴权边界，认证保护不能消除 schema 泄露风险。
- Go 当前没有 Spring Security 的“路径存在但未匿名放行”中间态；不注册路由更直接，也更符合当前 chi router 结构。
- 文档入口是开发和测试联调能力，不是生产业务能力。
- 显式 `OPENAPI_ENABLED=true` 保留受控非默认入口，适合临时内网演示或排障，但部署侧必须主动承担风险。

## 影响
- 好处
  - prod 默认暴露面最小。
  - 管理员 token 不能绕过默认关闭策略。
  - router 行为简单：启用才注册，禁用即 `COMMON-404`。
  - dev/test 默认联调体验不受影响。
- 代价
  - Go prod 未认证访问文档路径返回 `COMMON-404`，而 Java 在未认证且路径受 Spring Security 管控时可能先返回 `AUTH-401`；这是框架结构差异，安全目标保持一致。
  - 如果生产确实需要查看文档，必须显式打开开关或使用非生产环境副本。
  - Swagger UI HTML 使用外部 CDN 资源，受网络策略影响；但 prod 默认关闭，不增加生产运行依赖。
- 后续可能需要调整的地方
  - 如果需要生产临时文档，应补充内网限制、认证、审计和短期自动关闭策略。
  - 如部署到网关后，可在网关层额外屏蔽 `/openapi.yaml` 和 `/swagger/*`。
  - CI 或部署检查可禁止 prod profile 与 `OPENAPI_ENABLED=true` 同时出现，除非有显式变更审批。
