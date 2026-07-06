# OpenAPI Generated 文件拆分实现说明

## 1. 本次改动解决了什么问题

本次改动解决了 `api/openapi/gen/eventhub.gen.go` 单文件过长、阅读时难以区分 schema model 与 generated server wrapper 的问题。

改动前，`oapi-codegen` 使用单个 `api/openapi/oapi-codegen.yaml` 同时生成 `models`、`chi-server` 和 `strict-server`，所有内容输出到 `eventhub.gen.go`。strict-server 接入后，该文件同时包含 OpenAPI schema model、request/response object、chi wrapper、strict server interface 和 response writer，阅读成本偏高。

改动后，OpenAPI generated code 仍位于同一个 Go package `gen`，但物理文件按职责拆为 `models.gen.go` 和 `server.gen.go`。业务代码继续 import `eventhub-go/api/openapi/gen`，不感知文件名变化。

## 2. 改动内容
- 新增了什么
  - `api/openapi/oapi-codegen.models.yaml`
    - 只生成 `models`。
    - 输出 `api/openapi/gen/models.gen.go`。
  - `api/openapi/oapi-codegen.server.yaml`
    - 生成 `chi-server` 和 `strict-server`。
    - 输出 `api/openapi/gen/server.gen.go`。
  - `api/openapi/gen/models.gen.go`
    - 承载 OpenAPI schema、request body、response data、enum 和参数 model。
  - `api/openapi/gen/server.gen.go`
    - 承载 `ServerInterface`、chi wrapper、strict server interface、strict request/response object 和 generated response writer。
  - `TestOpenAPIGeneratedFilesAreSplit`
    - 固化 generated 文件布局，要求两个新文件存在，旧 `eventhub.gen.go` 不存在。
  - `docs/ai/design/034-openapi-generated-file-split.md`
  - `docs/ai/implementation/034-openapi-generated-file-split.md`
  - `docs/ai/adr/0026-openapi-generated-file-split.md`
- 修改了什么
  - `Makefile`
    - 将 `OAPI_CODEGEN_CONFIG` 拆为 `OAPI_CODEGEN_MODELS_CONFIG` 和 `OAPI_CODEGEN_SERVER_CONFIG`。
    - 将 `OPENAPI_GEN` 改为两个 generated 文件。
    - `openapi-generate` 改为串行执行两次 oapi-codegen。
    - `openapi-check` 继续 validate、generate 和 generated diff，diff 范围只覆盖两个当前 generated 文件。
  - `README.md`
    - 说明 OpenAPI generated code 已按职责拆为 `models.gen.go` 和 `server.gen.go`。
  - `docs/ai/adr/0025-openapi-strict-server-runtime-router.md`
    - 补充 ADR-0026 只调整 generated 物理文件布局，不改变 strict-server runtime router 决策。
  - `docs/ai/parity/java-go-parity-matrix.md`
    - 将当前 OpenAPI generated code 索引从旧单文件更新为双文件。
  - `docs/ai/parity/java-auth-api-contract.md`
    - 同步生成代码路径说明。
- 删除了什么
  - `api/openapi/oapi-codegen.yaml`
  - `api/openapi/gen/eventhub.gen.go`
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次不改变 Java-facing API 契约，但改变 Go-only generated code 组织方式和 OpenAPI 生成治理约定，需要在 parity matrix 中索引。

## 3. 为什么这样设计
- 关键设计原因
  - 按 `models` / `server` 拆分是 `oapi-codegen v2.5.0` 已支持的能力，不需要自定义模板。
  - 两个 generated 文件保持同 package `gen`，所以现有 handler、response、router 仍通过相同 import path 使用 generated 类型。
  - `make openapi-generate` 仍是唯一生成入口，减少手工命令分叉。
  - 旧 `eventhub.gen.go` 已从仓库删除；Makefile 不再保留旧单文件路径配置，避免当前生成链路读起来像仍兼容旧输出。
  - policy test 把文件布局变成可执行约束，后续误合并为单文件或手工残留旧文件会被本地测试捕获。
- 与 Go 项目当前阶段的匹配点
  - 不调整 `internal/http`、service、domain、repository 或 sqlc。
  - 不改变 OpenAPI spec，不影响 Redocly lint、kin-openapi validate 或 oasdiff breaking check 的契约输入。
  - 不引入额外依赖。
- 与 Java 版业务语义的对齐方式
  - Java 版仍以 controller / DTO / Springdoc 表达 HTTP 契约。
  - Go 版继续以 `eventhub.yaml` 作为唯一契约源，generated 文件拆分只改善 Go 仓库可读性，不改变对外业务语义。

## 4. 替代方案
- 方案 A：继续保留单个 `eventhub.gen.go`。
  - 没有采用。它不能解决生成文件阅读成本高的问题。
- 方案 B：按 OpenAPI tag / operationId 拆多个 generated server 文件或 package。
  - 没有采用。`oapi-codegen v2.5.0` 虽然支持 include/exclude tags，但不会自动生成可组合的模块化 strict server interface；当前引入多配置过滤会增加维护风险。
- 方案 C：拆分 OpenAPI spec，再通过 import mapping / self mapping 组合生成。
  - 没有采用。当前 spec 规模还不需要拆源 YAML；过早拆 spec 会增加 lint、validate、breaking check 和文档治理复杂度。
- 方案 D：使用自定义 oapi-codegen templates。
  - 没有采用。自定义模板会绑定生成器内部结构，升级成本和隐性兼容风险高于当前收益。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./api/openapi -run TestOpenAPIGeneratedFilesAreSplit -count=1`
    - 失败原因符合预期：`models.gen.go` 尚不存在。
  - GREEN：`make openapi-generate`
    - 通过，生成 `models.gen.go` 和 `server.gen.go`，并删除旧 `eventhub.gen.go`。
  - GREEN：`go test ./api/openapi -run TestOpenAPIGeneratedFilesAreSplit -count=1`
    - 通过。
  - `go test ./api/openapi -count=1`
    - 通过。
  - `go test ./...`
    - 通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w api/openapi/openapi_policy_test.go`
    - 已执行。
  - `gofmt -l api/openapi/openapi_policy_test.go api/openapi/gen/models.gen.go api/openapi/gen/server.gen.go`
    - 无输出。
  - `go vet ./...`
    - 通过。
  - `make openapi-lint`
    - 通过，Redocly 输出 API description valid。
  - `make lint`
    - 通过，输出 `0 issues.`。
  - `git diff --check`
    - 通过。
  - `make openapi-check`
    - `openapi-validate` 阶段通过。
    - `openapi-generate` 阶段通过。
    - `git diff --exit-code -- api/openapi/gen/models.gen.go api/openapi/gen/server.gen.go` 阶段通过。
  - 代码评审反馈后清理迁移期 legacy 配置：
    - 删除 `OPENAPI_LEGACY_GEN`。
    - 删除 `openapi-generate` 中的旧文件清理命令。
    - `openapi-check` 只检查当前两个 generated 输出文件。
  - 生成幂等性检查：
    - 重新运行 `make openapi-generate` 后，`models.gen.go` SHA-256 仍为 `8ebdaf1a20cbc115d14bff211f1d099a28964e8be8920078c0c4597d638cab8c`。
    - 重新运行 `make openapi-generate` 后，`server.gen.go` SHA-256 仍为 `57262798f8d734c0a44d266a475a645dbd21052374850db46481ef8c3713cb8f`。
  - `sqlc generate`：未运行，本次不涉及 SQL、schema 或 sqlc 配置。
  - migration 测试：未运行，本次不涉及 migration。
  - OpenAPI breaking check：未运行，本次未改变 `api/openapi/eventhub.yaml` 的 API 契约。
- 手工验证了哪些场景
  - `wc -l` 确认 `models.gen.go` 约 455 行，`server.gen.go` 约 1472 行。
  - 确认 `api/openapi/gen/eventhub.gen.go` 不存在。
  - 检查 `models.gen.go` 含 model 类型与 enum，`server.gen.go` 含 server interface 与 strict wrapper。
  - 检查业务 import path 仍为 `eventhub-go/api/openapi/gen`。
- Java-Go parity 如何验证
  - 确认 `api/openapi/eventhub.yaml` 未修改。
  - 确认 API path、method、请求字段、响应字段、状态码和错误码未修改。
  - 更新 `docs/ai/parity/java-go-parity-matrix.md` 与 `docs/ai/parity/java-auth-api-contract.md` 中 generated code 路径。
- 结果如何
  - 生成、编译、测试、vet、lint、OpenAPI lint、格式化和 diff whitespace 检查均通过。
  - `make openapi-check` 的 validate/generate 阶段通过，最终 diff gate 在未提交 generated 文件拆分状态下按预期失败。

## 6. 已知限制
- `server.gen.go` 仍然较长，因为 strict request/response object 和 response writer 都属于 server 侧 generated code。
- 本次只按生成职责拆文件，没有按业务模块拆 generated interface。
  - `make openapi-check` 只检查当前两个 generated 输出文件；旧 `eventhub.gen.go` 是否残留由 OpenAPI policy test 和 Go 编译共同兜底。
- 早期历史设计 / implementation 文档中仍可能提到当时的 `eventhub.gen.go`，本次只更新当前事实索引和后续维护入口。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - OpenAPI 生成链路更容易解释：spec -> models generated file / server generated file -> router strict wrapper -> handler。
  - generated 文件布局更适合 code review 和新同学阅读。
- 对微服务 / 云原生演进的影响
  - 当前拆分不影响部署、容器镜像、OpenAPI docs route 或 API breaking check。
  - 如果后续按服务边界拆 API spec，可以基于本次双文件配置继续演进。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 新增 OpenAPI schema 或 operation 后仍运行 `make openapi-generate` / `make openapi-check`。
  - 若未来 oapi-codegen 支持稳定的按 tag 多文件输出，可以新增设计和 ADR 再迁移。
  - policy test 会持续阻止旧单文件 `eventhub.gen.go` 回归。
