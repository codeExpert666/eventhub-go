# Router 与 OpenAPI 契约一致性测试设计

## 1. 背景
- 当前 Go 版采用 spec-first OpenAPI，`api/openapi/eventhub.yaml` 是 API 契约源，运行时 HTTP 路由则在 `internal/http/router.go` 中手写注册。
- `api/openapi/gen` 中已经生成 chi server wrapper，但当前尚未接入运行时 router，因此新增、删除或改名接口时，存在 router 与 OpenAPI `paths`/`methods` 漂移风险。
- `/openapi.yaml` 与 `/swagger/*` 文档路由是否注册由 `OPENAPI_ENABLED` 控制，是文档发布能力，不属于业务 API 契约覆盖范围。
- `/actuator/*` 在 OpenAPI 中声明，并且运行时 router 已注册，应纳入契约一致性覆盖。
- Java 版通过 controller 注解和 Springdoc 更接近“运行时接口声明生成文档”的模式；Go 版当前选择手写 router + spec-first YAML，所以需要额外测试固定两者一致性。

## 2. 目标
- 新增测试枚举运行时 router 中的业务 API method/path。
- 使用 `kin-openapi` 从 `api/openapi/eventhub.yaml` 对应的嵌入 spec 读取 OpenAPI `paths`/`methods`。
- 对比范围包含：
  - `/api/**`
  - `/actuator/**`
- 对比范围排除：
  - `/openapi.yaml`
  - `/swagger`
  - `/swagger/*`
  - `OPTIONS` 或框架自动生成方法。
- 对 chi 路由参数格式与 OpenAPI path 参数格式做归一化，例如 `{userId}` 与未来可能出现的 `{userId:regex}` 都归一为 `{userId}`。
- 测试失败时分别输出：
  - router 有但 spec 没有的 method/path。
  - spec 有但 router 没有的 method/path。
- 保证 `go test ./...` 与 `make openapi-check` 通过。

## 3. 非目标
- 不改变业务 handler、service、repository、middleware 或 app provider 行为。
- 不接入 generated chi server wrapper。
- 不引入运行时 OpenAPI request/response 校验 middleware。
- 不调整 `OPENAPI_ENABLED` 的文档路由开关语义。
- 不删除任何业务路由；如发现真实差异，优先修正 OpenAPI spec 或测试归一化逻辑。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `internal/http/router_contract_test.go`：新增运行时 router 与 OpenAPI paths/methods 一致性测试。
  - `docs/ai/design/023-router-openapi-contract-test.md`：记录测试设计。
  - `docs/ai/implementation/023-router-openapi-contract-test.md`：记录实现和验证。
  - `docs/ai/parity/java-go-parity-matrix.md`：更新 OpenAPI / Swagger 或测试策略索引。
- 不涉及 API 字段、错误码、数据库、sqlc、migration、缓存、JWT 或运行时鉴权语义。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 影响。该测试属于 Go 版 spec-first 与手写 router 之间的契约质量门禁，补足 Java controller/Springdoc 模式下不明显的漂移风险。

## 5. 领域建模
- `RouteContract`
  - 测试内部的 method/path 键，不是运行时领域对象。
  - 由 HTTP method 和规范化后的 path 组成，例如 `PATCH /api/v1/admin/users/{userId}/status`。
- `RouterRouteSet`
  - 通过 `chi.Walk` 从完整运行时 router 枚举。
  - 只保留 `/api/` 和 `/actuator/` 前缀，过滤文档路由与 `OPTIONS`。
- `SpecRouteSet`
  - 通过 `kin-openapi` 解析 OpenAPI `paths`。
  - 只保留相同覆盖范围和 method 过滤规则。
- 与 Java 版领域对象的对应关系：
  - 不新增业务领域对象。
  - Java controller method + Springdoc operation 对应 Go OpenAPI operation；Go router registration 对应 Java MVC handler mapping。

## 6. API 设计
- 本次不新增或修改运行时 HTTP API。
- 测试覆盖当前 OpenAPI 已声明且 router 已注册的 API：
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout`
  - `GET /api/v1/me`
  - `GET /api/v1/admin/users`
  - `PATCH /api/v1/admin/users/{userId}/status`
  - `GET /api/v1/system/ping`
  - `POST /api/v1/system/echo`
  - `GET /actuator/health`
  - `HEAD /actuator/health`
  - `GET /actuator/info`
  - `HEAD /actuator/info`
- 错误码 / 异常场景：
  - 运行时错误码不变。
  - 测试失败信息按差异方向分组，便于判断是 router 多注册、spec 漏写，还是 spec 有但 router 漏注册。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Java 版 Springdoc 可从 controller 注解生成文档；Go 版当前手写 router 和 YAML 是两个源，因此用测试弥补漂移风险。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 不涉及持久化。

## 8. 关键流程
- 正常流程：
  1. 测试使用现有 auth 集成测试 helper 构造完整 router，包含 system、auth、user、RBAC middleware 和 actuator 路由。
  2. 将 `http.Handler` 断言为 `chi.Routes`，通过 `chi.Walk` 枚举 method/path。
  3. 过滤非契约路由和 `OPTIONS`，归一化 path 参数。
  4. 使用 `kin-openapi` 加载并 validate `eventhub.yaml` 对应 spec。
  5. 遍历 `doc.Paths.Map()` 和 `PathItem.Operations()`，得到 spec method/path 集合。
  6. 对 router 集合和 spec 集合做双向差集。
  7. 如果存在差异，按固定排序输出 `router 有但 spec 没有` 与 `spec 有但 router 没有`。
- 异常流程：
  - router 不是 `chi.Routes` 时测试失败，提示当前测试依赖 chi 路由树。
  - OpenAPI spec 加载或 validate 失败时测试失败。
  - 新增路由未同步 YAML、删除路由未同步 YAML、method 写错或 path 参数格式漂移时测试失败。
- 状态流转：
  - 不涉及业务状态机。
- handler / service / repository / sqlc/database 分工：
  - 测试只观察 router 和 OpenAPI 文档。
  - 不改变 handler、service、repository、sqlc/database 分层。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无。
- 如何防重复提交：无运行时写操作。
- 事务边界在哪里：无数据库事务。
- 缓存放在哪里，为什么：不涉及缓存。

## 10. 权限与安全
- 哪些角色能访问：
  - 本次不改变接口访问角色。
  - admin API 仍由 `AuthMiddleware` 与 `RequireRole("ADMIN")` 控制。
- 鉴权与鉴别约束：
  - 测试需要构造带 auth/user 依赖和 auth middleware 的完整 router，确保受保护路由也被枚举。
  - 测试不发送 HTTP 请求，不验证 token、principal 或角色判断。
- JWT claim 边界：
  - 不修改 JWT，不把角色、邮箱、用户名或用户状态写入 JWT。
- 是否涉及敏感信息、审计或操作日志：
  - 不涉及敏感信息或审计日志。

## 11. 测试策略
- 单元测试：
  - 新增 `TestRouterContractRoutesMatchOpenAPISpec`。
  - 通过 helper 覆盖 route 集合差异、路径参数归一化和稳定排序。
- service / repository 测试：
  - 不新增；本次不触碰业务服务和持久化。
- migration / sqlc 验证：
  - 不运行；本次没有数据库、SQL 或 sqlc 配置变化。
- 接口验证：
  - 使用 `chi.Walk` 验证运行时已注册 method/path，而不是只检查 YAML。
- OpenAPI validate：
  - 测试内调用 `doc.Validate(context.Background())`。
  - 最终运行 `make openapi-check`。
- 异常场景验证：
  - TDD RED 阶段先让新增漂移测试暴露当前真实差异或缺失测试逻辑。
  - 若发现真实差异，优先修正 OpenAPI spec 或归一化逻辑，不删除业务路由。
- Java-Go parity 验证：
  - 更新 parity matrix，说明 Go 版新增 router/spec 双向一致性门禁。
- 需要运行的命令：
  - `gofmt -w internal/http/router_contract_test.go`
  - `go test ./internal/http -run TestRouterContractRoutesMatchOpenAPISpec -count=1`
  - `go test ./...`
  - `go vet ./...`
  - `make openapi-check`

## 12. 风险与替代方案
- 当前方案的风险：
  - 如果后续 router 使用非 chi 实现，测试需要改造枚举方式。
  - 如果新增非 `/api/`、非 `/actuator/` 的业务 API，需要同步更新过滤范围和设计说明。
  - 该测试只比较 method/path，不校验请求字段、响应字段或安全策略；这些仍由既有 OpenAPI policy 与业务集成测试覆盖。
- 备选方案：
  - 方案 A：接入 generated chi server wrapper，让生成代码负责路由注册。
  - 方案 B：只用 `make openapi-check` 和既有 OpenAPI policy test。
  - 方案 C：手工维护一份 expected route list。
- 为什么不选备选方案：
  - 不选方案 A：会改变运行时 router 装配方式，范围超过本次测试门禁目标。
  - 不选方案 B：OpenAPI validate/generate 不能证明运行时 router 已注册同一批 path/method。
  - 不选方案 C：第三份手工列表本身也会漂移，无法降低维护风险。
- 后续可演进点：
  - 后续接入 generated chi server wrapper 后，可把此测试保留为端到端契约回归。
  - 如未来需要覆盖安全策略，可在 router middleware 元数据或 OpenAPI extension 之间继续增加机器校验。
