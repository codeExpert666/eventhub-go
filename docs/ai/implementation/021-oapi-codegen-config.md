# oapi-codegen 配置文件化实现说明

## 1. 本次改动解决了什么问题

本次将 `Makefile` 中直接拼接的 `oapi-codegen` 参数迁移到显式配置文件，降低后续维护 OpenAPI 生成链路的成本。

迁移后，生成配置集中在 `api/openapi/oapi-codegen.yaml`，`Makefile` 仍保留 `OAPI_CODEGEN_VERSION`，继续用固定版本的 `go run module@version` 执行生成。

## 2. 改动内容
- 新增了什么
  - `api/openapi/oapi-codegen.yaml`：
    - `package: gen`
    - `output: api/openapi/gen/eventhub.gen.go`
    - `generate.models: true`
    - `generate.chi-server: true`
  - 设计文档：`docs/ai/design/021-oapi-codegen-config.md`。
- 修改了什么
  - `Makefile`：
    - 新增 `OAPI_CODEGEN_CONFIG := api/openapi/oapi-codegen.yaml`。
    - `openapi-generate` 改为 `-config $(OAPI_CODEGEN_CONFIG) $(OPENAPI_SPEC)`。
  - `docs/ai/parity/java-go-parity-matrix.md`：
    - OpenAPI / Swagger 行补充 Go 版生成链路由 `oapi-codegen.yaml` 配置驱动。
- 删除了什么
  - 未删除文件。
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次触及 OpenAPI 生成策略和工程质量门禁索引，但不改变业务 API 契约。

## 3. 为什么这样设计
- 关键设计原因
  - `oapi-codegen` 的生成选项会随后续演进增加，放入 YAML 配置比继续拼接 Makefile 参数更可读。
  - 当前 `oapi-codegen v2.5.0` 的配置文件中，命令行 `-generate types` 等价表达为 `generate.models: true`；本次用工具自身的 `-output-config` 结果和源码确认了该映射。
  - 保留 `OAPI_CODEGEN_VERSION`，避免不同开发机使用不同生成器版本造成 generated 文件漂移。
- 与 Go 项目当前阶段的匹配点
  - 继续保持 spec-first OpenAPI。
  - 继续生成 types/models 与 chi server interface。
  - 不接入 strict-server，不改变 router 行为。
- 与 Java 版业务语义的对齐方式
  - Java 版没有 `oapi-codegen` 配置文件；这是 Go 版 spec-first 工程链路的自然实现方式。
  - `api/openapi/eventhub.yaml` 作为契约源不变，因此 Java-Go API 语义不变。

## 4. 替代方案
- 方案 A：继续在 `Makefile` 中拼接 `-generate types,chi-server -package gen -o ...`。
  - 没有采用。后续新增 import mapping、compatibility 或模板选项时，Makefile 会越来越难读。
- 方案 B：顺手接入 `strict-server`。
  - 没有采用。当前验收明确要求暂不引入 strict-server，也不改变运行时 router。
- 方案 C：升级 `oapi-codegen`。
  - 没有采用。本次目标是等价迁移配置，不扩大生成差异和验证范围。

## 5. 测试与验证
- 跑了哪些测试
  - `make openapi-generate`：通过。
  - `make openapi-check`：通过。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `make lint`：通过，输出 `0 issues.`。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt`：未运行；本次没有修改 Go 源码，且 generated 文件重新生成后无 diff。
  - `sqlc generate`：未运行；本次没有修改 SQL、schema 或 sqlc 配置。
  - migration 测试：未单独运行；本次没有 migration 变化。
- 手工验证了哪些场景
  - 使用 `oapi-codegen v2.5.0 -output-config` 确认当前命令行参数对应配置为 `models: true` 与 `chi-server: true`。
  - `make openapi-generate` 后检查 `api/openapi/gen/eventhub.gen.go` 无 diff，确认生成结果等价。
- Java-Go parity 如何验证
  - 检查 `api/openapi/eventhub.yaml` 未改动，运行时 API 契约不变。
  - 更新 parity matrix 的 OpenAPI / Swagger 行，索引本次 Go-only 生成链路维护。
- 结果如何
  - `api/openapi/oapi-codegen.yaml` 成为生成配置入口。
  - generated 文件没有非预期大规模变化。
  - 未接入 strict-server，未改 router 行为。

## 6. 已知限制
- 当前版本还缺什么
  - 生成接口仍未接入运行时 router；本次保持现状。
- 哪些地方后面需要继续演进
  - 后续如需 import mapping、兼容选项、模板覆盖或 strict-server，应优先扩展 `api/openapi/oapi-codegen.yaml`，并补充设计或 ADR。
- 与 Java 版仍有哪些差距
  - Java 版使用 Springdoc 注解扫描；Go 版继续使用 spec-first YAML 与 oapi-codegen 配置文件。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 生成链路更清晰，便于审查和 CI 维护。
- 对微服务 / 云原生演进的影响
  - 配置文件化便于未来按服务拆分 OpenAPI 生成规则。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续 OpenAPI 生成策略变化应修改 `api/openapi/oapi-codegen.yaml`，并继续通过 `make openapi-check` 防止 generated 文件漂移。
