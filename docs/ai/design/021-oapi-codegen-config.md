# oapi-codegen 配置文件化设计

## 1. 背景
- 当前 `Makefile` 在 `openapi-generate` 中直接拼接 `oapi-codegen` 参数：`-generate types,chi-server -package gen -o $(OPENAPI_GEN) $(OPENAPI_SPEC)`。
- 这种写法在参数较少时可读，但后续如果要增加兼容选项、类型映射、导入映射或生成过滤规则，Makefile 会逐步承担工具配置细节，维护成本上升。
- Java 版没有对应的 `oapi-codegen` 配置文件；本次迁移属于 Go 版 spec-first OpenAPI 工程链路维护，业务契约仍以 `api/openapi/eventhub.yaml` 为源。

## 2. 目标
- 新增 `api/openapi/oapi-codegen.yaml`，显式描述当前等价生成配置。
- 保留当前生成能力：
  - Go package 为 `gen`。
  - 输出文件为 `api/openapi/gen/eventhub.gen.go`。
  - 生成 models/types 与 chi-server。
- 修改 `Makefile` 的 `openapi-generate`，改用 `-config api/openapi/oapi-codegen.yaml api/openapi/eventhub.yaml`。
- 保留 `OAPI_CODEGEN_VERSION` 版本变量，继续通过 `go run module@version` 固定生成器版本。
- 生成后确认 `api/openapi/gen/eventhub.gen.go` 没有非预期大规模变化。

## 3. 非目标
- 不引入 `strict-server`。
- 不修改运行时 router、handler、middleware、service 或 repository。
- 不调整 `api/openapi/eventhub.yaml` 的 API 契约、路径、请求字段、响应字段或错误码。
- 不改变 OpenAPI 文档入口启用/禁用行为。
- 不调整 `oapi-codegen` 版本。

## 4. 影响范围
- 涉及 Go package / 模块：
  - `api/openapi/oapi-codegen.yaml`：新增生成器配置文件。
  - `Makefile`：`openapi-generate` 从命令行参数迁移到配置文件。
  - `api/openapi/gen/eventhub.gen.go`：重新生成并检查漂移。
  - `docs/ai/implementation/021-oapi-codegen-config.md`：记录实现和验证。
  - `docs/ai/parity/java-go-parity-matrix.md`：补充 Go 版 OpenAPI 生成链路由配置文件驱动。
- 涉及 API / 表 / 缓存 / 外部接口：
  - 不涉及运行时 API 行为。
  - 不涉及数据库、migration、sqlc、Redis 或外部接口。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 影响 OpenAPI / Swagger 工程链路记录；业务语义不变，但生成策略属于 Go 与 Java 的刻意差异索引。

## 5. 领域建模
- `OpenAPISpec`
  - 物理文件：`api/openapi/eventhub.yaml`。
  - 继续作为 API 契约源。
- `OAPICodegenConfig`
  - 物理文件：`api/openapi/oapi-codegen.yaml`。
  - 表达构建期生成策略：package、output、generate 选项。
  - 当前仅开启 `models` 与 `chi-server`，其中 `models` 对应旧命令行的 `types` 生成能力。
- `GeneratedOpenAPI`
  - 物理文件：`api/openapi/gen/eventhub.gen.go`。
  - 继续由 `oapi-codegen v2.5.0` 生成。
  - 当前仍不接入运行时 router。

## 6. API 设计
- 本次不新增或修改运行时 HTTP API。
- OpenAPI 契约文件 `api/openapi/eventhub.yaml` 不变。
- `oapi-codegen` 配置只影响构建期生成命令：
  - `package: gen`
  - `output: api/openapi/gen/eventhub.gen.go`
  - `generate.models: true`
  - `generate.chi-server: true`
- 错误码 / 异常场景：
  - 运行时错误码不变。
  - 生成失败仍由 `make openapi-generate` 直接返回非零退出码。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Java 继续通过 Springdoc 注解扫描生成文档。
  - Go 继续采用 spec-first YAML，并将生成器参数显式配置化。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：无。
- 数据一致性考虑：
  - 本次仅涉及 OpenAPI 生成产物一致性。
  - `make openapi-check` 继续承担契约校验、重新生成和 generated file 漂移检查。

## 8. 关键流程
- 正常流程：
  1. 开发者执行 `make openapi-generate`。
  2. Makefile 创建 `api/openapi/gen` 目录。
  3. Makefile 通过 `go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)` 调用固定版本生成器。
  4. 生成器读取 `api/openapi/oapi-codegen.yaml`。
  5. 生成器读取 `api/openapi/eventhub.yaml`。
  6. 生成 `api/openapi/gen/eventhub.gen.go`。
- 异常流程：
  - 配置文件语法错误、OpenAPI 契约错误或生成器版本不兼容时，`make openapi-generate` 返回失败。
  - `make openapi-check` 在重新生成后通过 `git diff --exit-code $(OPENAPI_GEN)` 捕获 generated 文件漂移。
- 状态流转：
  - 不涉及业务状态机。
- handler / service / repository / sqlc/database 分工：
  - 不触碰 handler、service、repository 或 sqlc/database。
  - 本次保持运行时 router 与 generated chi server interface 解耦。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无。
- 如何防重复提交：无运行时写入操作。
- 事务边界在哪里：无数据库事务。
- 缓存放在哪里，为什么：不涉及缓存。

## 10. 权限与安全
- 哪些角色能访问：不涉及运行时访问控制变化。
- 鉴权与鉴别约束：不修改 auth middleware 或 OpenAPI 文档入口注册条件。
- JWT claim 边界：不修改 JWT，不新增角色、邮箱、用户名或用户状态 claim。
- 是否涉及敏感信息、审计或操作日志：不涉及。

## 11. 测试策略
- 单元测试：
  - 不新增单元测试；本次没有 Go 运行时代码变化。
- service / repository 测试：
  - 不新增；本次不触碰业务服务和持久化。
- migration / sqlc 验证：
  - 不运行 sqlc 或 migration 专项验证；本次没有 SQL、schema 或 migration 变化。
- 接口验证：
  - 通过 `go test ./...` 确认 generated package 仍可编译。
- OpenAPI validate：
  - 运行 `make openapi-check`，其中包含 `make openapi-validate`。
- 异常场景验证：
  - 生成配置文件由 `make openapi-generate` 实际读取，能发现 YAML key 或兼容性问题。
- Java-Go parity 验证：
  - 检查 parity matrix 的 OpenAPI / Swagger 行，确认记录 Go spec-first 与配置文件驱动生成链路。
- 需要运行的命令：
  - `make openapi-generate`
  - `make openapi-check`
  - `go test ./...`

## 12. 风险与替代方案
- 当前方案的风险：
  - `oapi-codegen` 配置文件字段与命令行参数命名不同，`types` 在配置中表达为 `models`；需要用当前固定版本确认。
  - 后续修改配置时，如果绕过 `make openapi-check`，可能留下 generated 文件漂移。
- 备选方案：
  - 方案 A：继续在 Makefile 中拼接所有参数。
  - 方案 B：升级 `oapi-codegen` 或顺手接入 `strict-server`。
  - 方案 C：把 OpenAPI 生成包装成单独脚本。
- 为什么不选备选方案：
  - 不选方案 A：配置扩展性差，Makefile 会承载越来越多生成器细节。
  - 不选方案 B：本次目标是等价迁移生成配置，升级工具或接入 strict-server 会扩大验证面并可能改变 generated API。
  - 不选方案 C：当前仅需一个标准 YAML 配置文件，脚本会增加额外入口和维护成本。
- 后续可演进点：
  - 如果后续接入 strict-server、import-mapping、模板覆盖或 compatibility 选项，应优先扩展 `api/openapi/oapi-codegen.yaml` 并同步记录设计取舍。
  - CI 可继续使用 `make openapi-check` 防止生成产物漂移。
