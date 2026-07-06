# OpenAPI Generated 文件拆分

## 1. 背景
- 当前 Go 版已经通过 `oapi-codegen v2.5.0` 从 `api/openapi/eventhub.yaml` 生成 OpenAPI transport code。
- strict-server 迁移后，生成内容同时包含 schema/request/response model、chi server wrapper、strict server request/response object 和 strict handler glue。
- 这些内容当前全部位于 `api/openapi/gen/eventhub.gen.go`，文件长度接近两千行，阅读时很难区分“数据模型”和“运行时 wrapper”。
- Java 版对应语义仍是 Spring MVC controller mapping 与 Springdoc 生成文档；Go 版不复刻注解扫描，而是继续以 spec-first OpenAPI 和 generated strict server 作为 HTTP 契约来源。

## 2. 目标
- 将 OpenAPI generated output 按生成职责拆为两个文件：
  - `api/openapi/gen/models.gen.go`：OpenAPI schemas、request body、response data 和 enum model。
  - `api/openapi/gen/server.gen.go`：`ServerInterface`、chi wrapper、strict server interface、strict request/response object 和 generated response writer。
- 保持 Go package 仍为 `gen`，调用方继续使用 `eventhub-go/api/openapi/gen`，不改变业务 import path。
- 保持 `make openapi-generate` 和 `make openapi-check` 仍是生成和漂移检查入口。
- 生成链路只维护 `models.gen.go` 和 `server.gen.go` 两个当前输出文件；旧 `api/openapi/gen/eventhub.gen.go` 从仓库中删除，不再作为 Makefile 配置项保留。
- 用 OpenAPI policy test 固化生成文件布局，防止后续误把生成物合回单文件。

## 3. 非目标
- 不拆分 `api/openapi/eventhub.yaml`。
- 不按 OpenAPI tag、operationId 或业务模块拆多个 generated package / interface。
- 不修改 API path、method、请求字段、响应字段、错误码或 OpenAPI schema。
- 不修改 `internal/http/router.go`、`internal/http/openapi_routes.go`、`internal/http/openapi_adapter.go` 或业务 handler。
- 不调整 service、domain、repository、sqlc、migration 或 Java-Go 业务 parity。
- 不引入自定义 oapi-codegen template。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `api/openapi`
  - `api/openapi/gen`
  - `Makefile`
  - `README.md`
  - `docs/ai/design`
  - `docs/ai/implementation`
  - `docs/ai/adr`
  - `docs/ai/parity/java-go-parity-matrix.md`
- 涉及 API / 表 / 缓存 / 外部接口：
  - API 契约不变。
  - 数据库表、migration、sqlc query、缓存和外部接口不变。
- Java-Go parity matrix 需要更新，因为 Go-only generated code 组织方式变化，且 OpenAPI / Swagger 行原本明确索引了 `eventhub.gen.go`。

## 5. 领域建模
- 本次不新增业务领域对象。
- generated model 仍是 HTTP transport model，不是 domain model。
- `models.gen.go` 中的类型继续承载 OpenAPI 请求体、响应体、enum 和 schema contract。
- `server.gen.go` 中的类型继续承载 generated server interface、request/response object 和 strict wrapper。
- 与 Java 版关系：
  - Java 版由 Springdoc 从 controller 注解推导文档和 DTO schema。
  - Go 版继续由 `eventhub.yaml` 作为唯一契约源，生成文件拆分只影响 Go 仓库可读性，不影响对外业务语义。

## 6. API 设计
- 接口列表：不变。
- 请求参数：不变。
- 响应结构：不变。
- 错误码 / 异常场景：不变。
- 与 Java 版 OpenAPI / controller 契约差异：无新的对外契约差异。
- 生成链路设计：
  - `api/openapi/oapi-codegen.models.yaml` 只启用 `generate.models`，输出 `models.gen.go`。
  - `api/openapi/oapi-codegen.server.yaml` 启用 `generate.chi-server` 和 `generate.strict-server`，输出 `server.gen.go`。
  - 两个配置使用相同 `package: gen` 和相同 OpenAPI spec。
  - `Makefile` 的 `openapi-generate` 串行执行两次 `oapi-codegen`。

## 7. 数据设计
- 不调整表结构、索引、唯一约束或 migration。
- 不新增或修改 sqlc query。
- 不影响 repository/mysql 与 sqlc generated model。
- OpenAPI generated model 物理文件拆分后，Go 编译仍以同 package 视角解析类型。

## 8. 关键流程
- 正常流程：
  1. 开发者修改 `api/openapi/eventhub.yaml` 或生成配置。
  2. 执行 `make openapi-generate`。
  3. 使用 models 配置生成 `models.gen.go`。
  4. 使用 server 配置生成 `server.gen.go`。
  5. 运行 `make openapi-check` 时，先 validate spec，再 regenerate，并检查两个 generated 文件没有非预期漂移。
- 异常流程：
  - 任一配置文件语法错误、生成器版本不兼容或 OpenAPI spec 不合法时，`make openapi-generate` / `make openapi-check` 返回失败。
  - 如果旧 `eventhub.gen.go` 因手工操作残留，policy test 失败，并且 Go 编译可能出现重复声明；Makefile 不再保留该旧路径配置。
- 分层分工：
  - `api/openapi` 负责契约与生成治理。
  - `api/openapi/gen` 只存放 generated transport code。
  - `internal/http` 继续只依赖 `gen` package，不关心 generated code 的物理文件名。
  - service / domain / repository 继续不依赖 generated model。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及并发写入、幂等、防重复提交、事务、缓存或库存扣减。
- 生成命令串行执行，避免两个 `oapi-codegen` 进程同时写入同一目录带来的竞态。

## 10. 权限与安全
- 鉴权、RBAC、JWT claim 和 refresh token 语义不变。
- `/openapi.yaml`、`/swagger/*` 的 `OPENAPI_ENABLED` 控制不变。
- 生成物拆分不改变生产 HTTP 路由注册、认证错误优先级或安全响应 envelope。

## 11. 测试策略
- 单元测试：
  - 新增 `TestOpenAPIGeneratedFilesAreSplit`，检查 `models.gen.go` / `server.gen.go` 存在，旧 `eventhub.gen.go` 不存在。
- service / repository 测试：
  - 不涉及。
- migration / sqlc 验证：
  - 不涉及。
- 接口验证：
  - `go test ./api/openapi -run TestOpenAPIGeneratedFilesAreSplit -count=1`
  - `go test ./...`
- OpenAPI validate：
  - `make openapi-check`
  - `make openapi-lint`
- 异常场景验证：
  - RED 阶段：旧单文件布局下新增 policy test 应失败。
  - GREEN 阶段：拆分生成配置并重新生成后 policy test 应通过。
- Java-Go parity 验证：
  - 确认 API 契约、错误码、schema、runtime route 和业务 handler 无变化。
  - 更新 parity matrix 中 `eventhub.gen.go` 的生成物索引。
- 需要运行的命令：
  - `gofmt -w api/openapi/openapi_policy_test.go`
  - `go test ./api/openapi -run TestOpenAPIGeneratedFilesAreSplit -count=1`
  - `make openapi-generate`
  - `make openapi-check`
  - `go test ./...`
  - `go vet ./...`
  - `make openapi-lint`
  - `git diff --check`

## 12. 风险与替代方案
- 当前方案风险：
  - `server.gen.go` 仍会包含 strict request/response object，文件仍较长，但已经把 schema model 从运行时 wrapper 中分离。
  - `make openapi-check` 的 diff 变量需要覆盖两个 generated 文件，否则可能漏检其中一个生成物漂移。
  - 旧 `eventhub.gen.go` 如果未删除，会造成重复声明或 policy test 失败。
- 备选方案：
  - 方案 A：继续单文件输出，只靠阅读路径说明缓解可读性。
  - 方案 B：按 tag / operationId 生成多个模块文件。
  - 方案 C：拆分 OpenAPI spec，再使用 import mapping / self mapping 组合生成。
  - 方案 D：使用自定义 templates 强行改变 oapi-codegen 输出布局。
- 为什么不选备选方案：
  - 不选方案 A：不能解决用户指出的生成文件过长问题。
  - 不选方案 B：`oapi-codegen v2.5.0` 的 include/exclude tag 能过滤 operation，但不会自动生成可组合的模块化 strict server interface；在当前阶段引入多配置过滤风险高于收益。
  - 不选方案 C：当前 spec 规模还不需要拆文件源，拆 spec 会影响文档治理和跨分支 breaking check 复杂度。
  - 不选方案 D：自定义模板会增加长期维护成本，并可能在升级 oapi-codegen 时产生隐性兼容负担。
- 后续可演进点：
  - API 规模继续扩大后，可以重新评估按 spec domain 拆分，并用 import mapping 管理 shared schemas。
  - 如果 oapi-codegen 后续原生支持按 tag 多文件输出，可再评估迁移。
