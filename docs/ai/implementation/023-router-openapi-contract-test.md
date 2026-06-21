# Router 与 OpenAPI 契约一致性测试实现说明

## 1. 本次改动解决了什么问题

本次新增运行时 router 与 `api/openapi/eventhub.yaml` 的 path/method 双向一致性测试，解决当前 Go 版手写 `internal/http/router.go` 与 spec-first OpenAPI YAML 可能独立漂移的问题。

当前 generated chi server wrapper 尚未接入运行时 router，因此 `make openapi-check` 能证明 YAML 合法、生成物未漂移，但不能证明真实 router 注册了同一批 API。新测试用 `chi.Walk` 枚举完整运行时 router，并与 OpenAPI `paths`/`methods` 对比，让新增、删除或改名 API 时的同步遗漏能在 `go test ./...` 中暴露。

## 2. 改动内容
- 新增了什么
  - `internal/http/router_contract_test.go`：
    - 新增 `TestRouterContractRoutesMatchOpenAPISpec`。
    - 使用现有 `testAuthRouter(t)` 构造完整运行时 router，覆盖 system、auth、user、admin、actuator 路由。
    - 使用 `chi.Walk` 枚举 router method/path。
    - 使用 `kin-openapi` 解析 `api/openapi` 包嵌入的 `eventhub.yaml`，并执行 OpenAPI validate。
    - 只对比 `/api/` 与 `/actuator/` 路径。
    - 排除 `/openapi.yaml`、`/swagger`、`/swagger/*` 文档路由，以及 `OPTIONS`。
    - 对 chi path 参数正则形式做归一化，例如 `{userId:[0-9]+}` 归一为 `{userId}`。
    - 失败时分组输出 `router 有但 spec 没有` 与 `spec 有但 router 没有`。
  - `docs/ai/design/023-router-openapi-contract-test.md`。
  - `docs/ai/implementation/023-router-openapi-contract-test.md`。
- 修改了什么
  - `docs/ai/parity/java-go-parity-matrix.md`：
    - OpenAPI / Swagger 行补充 router/spec path/method 双向一致性测试索引。
- 删除了什么
  - 未删除文件。
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次触及 Go spec-first OpenAPI、手写 router 与 Java controller/Springdoc 模式之间的工程差异和测试策略。

## 3. 为什么这样设计
- 关键设计原因
  - `chi.Walk` 直接读取真实运行时路由树，比手工维护 expected route list 更接近当前服务暴露面。
  - 复用 `testAuthRouter(t)` 构造完整 router，避免只注册 system 路由导致 auth/user/admin 路由被误判缺失。
  - OpenAPI 侧使用 `kin-openapi` 语义解析而不是字符串匹配，能稳定读取 `paths` 和 method。
  - 测试过滤范围只保留 `/api/` 与 `/actuator/`，符合业务 API 和 actuator 契约覆盖边界。
  - 文档路由受 `OPENAPI_ENABLED` 控制，不属于业务 API 契约，所以明确排除。
  - path 参数归一化放在 router 与 spec 共同入口，后续 chi 参数正则不会造成无意义差异。
  - 差异输出使用双向集合差，能区分“router 多注册”与“spec 多声明”。
- 与 Go 项目当前阶段的匹配点
  - 不改变 `internal/http/router.go` 业务注册方式，不提前接入 generated server wrapper。
  - 不新增依赖；`kin-openapi` 和 `chi` 已是当前仓库依赖。
  - 测试文件位于 `internal/http`，因为被验证对象是运行时 router。
- 与 Java 版业务语义的对齐方式
  - Java controller/Springdoc 的契约来源与运行时 mapping 更紧密；Go 当前手写 router 和 YAML 分离，所以用 Go test 固化两者一致。
  - Actuator 路径继续纳入 OpenAPI 和运行时 router 双向覆盖。

## 4. 替代方案
- 方案 A：接入 generated chi server wrapper。
  - 没有采用。该方案会改变运行时 router 装配方式，涉及 handler 适配和中间件边界，超出本次“新增漂移检测测试”的范围。
- 方案 B：只依赖 `make openapi-check`。
  - 没有采用。`make openapi-check` 验证 spec 与生成物，不枚举运行时 router，无法发现手写 router 漏同步。
- 方案 C：手写 expected route list。
  - 没有采用。第三份列表本身也会漂移，且维护成本高于直接比较 router 与 spec 两个真实来源。
- 方案 D：把测试放在 `api/openapi` 包。
  - 没有采用。`api/openapi` 包适合测试 spec policy；本次需要构造并遍历运行时 router，放在 `internal/http` 更贴近职责边界。

## 5. 测试与验证
- 跑了哪些测试
  - RED：先让 `collectOpenAPIContractRoutes` 返回空集合，运行 `go test ./internal/http -run TestRouterContractRoutesMatchOpenAPISpec -count=1`，测试失败并按 `router 有但 spec 没有` 输出 13 条当前运行时路由。
  - GREEN：接入真实 OpenAPI 解析后，运行 `go test ./internal/http -run TestRouterContractRoutesMatchOpenAPISpec -count=1`，测试通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w internal/http/router_contract_test.go`：已运行。
  - `go test ./internal/http -run TestRouterContractRoutesMatchOpenAPISpec -count=1`：已运行，通过。
  - `go test ./...`：已运行，通过。
  - `go vet ./...`：已运行，通过。
  - `make openapi-check`：已运行，通过，包含 validate、generate 和 generated file diff 检查。
  - `make lint`：已运行，通过，输出 `0 issues.`。
  - `git diff --check`：已运行，通过。
  - `sqlc generate`：不运行；本次没有 SQL、schema 或 sqlc 配置变化。
  - migration 测试：不运行；本次没有 migration 变化。
- 手工验证了哪些场景
  - 检查 `internal/http/router.go`，确认文档路由独立受 `deps.OpenAPI` 控制，测试构造的完整 router 不注册文档路由。
  - 检查 `api/openapi/eventhub.yaml`，确认 actuator `GET/HEAD` 和业务 API 当前均有对应 path/method 声明。
- Java-Go parity 如何验证
  - 已更新 parity matrix 的 OpenAPI / Swagger 行，记录 Go 版新增 router/spec 双向一致性门禁。
- 结果如何
  - 当前未发现真实 router/spec method/path 差异。
  - 后续如果新增业务路由但忘记更新 `eventhub.yaml`，或 spec 声明了 router 未注册的 path/method，该测试会失败并输出差异方向。

## 6. 已知限制
- 当前版本还缺什么
  - 该测试只比较 method/path，不校验 request body、query、response schema、错误码和 security；这些仍由 OpenAPI policy test 与业务集成测试覆盖。
  - 如果未来业务 API 不再使用 `/api/` 或 `/actuator/` 前缀，需要同步更新过滤规则和设计说明。
- 哪些地方后面需要继续演进
  - 后续接入 generated chi server wrapper 后，可以保留该测试作为端到端契约回归。
  - 未来若需要校验 router middleware 与 OpenAPI security，可引入明确的路由元数据或扩展字段策略。
- 与 Java 版仍有哪些差距
  - Java 版依赖 controller 注解和 Springdoc/Spring MVC mapping；Go 版当前仍是 spec-first YAML 与手写 chi router 并存。
  - 本次补的是 Go 当前架构下的质量门禁，不改变 Java-Go API 业务语义。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 增强 spec-first 工程治理能力，能展示 OpenAPI 契约与真实运行时暴露面之间的自动化防漂移测试。
- 对微服务 / 云原生演进的影响
  - 后续服务拆分、网关配置或 SDK 生成依赖 OpenAPI 时，method/path 的基础契约更可靠。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续新增 API 时，需要同步修改 `internal/http/router.go` 和 `api/openapi/eventhub.yaml`，否则 `go test ./...` 会失败。
  - 不影响 migration、sqlc 或 repository 分层。
