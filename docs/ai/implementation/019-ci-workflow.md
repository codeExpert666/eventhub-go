# GitHub Actions CI Workflow 实现说明

## 1. 本次改动解决了什么问题

本次继续演进 Go 版 EventHub 的 GitHub Actions CI workflow，把 `fmt`、`sqlc` 和 OpenAPI 三类“先格式化 / 生成，再检查未提交漂移”的规则统一沉淀到 `Makefile`。

改动前，OpenAPI 已由 `make openapi-check` 封装 validate、generate 和 generated file diff；但 `fmt` 和 `sqlc` 在 CI 里分别拆成 `make fmt` / `make sqlc` 加 workflow 内部 `git diff --exit-code`。这会让本地验证入口和远端 CI 编排不一致，也让后续调整 diff 范围或生成物检查策略时需要同时修改 Makefile 和 GitHub Actions YAML。

本次选择更新已有 `docs/ai/design/019-ci-workflow.md` 和本实现说明，而不是新增 021 文档。原因是这次不是新的业务能力或新的 CI 子系统，而是 019 CI workflow 设计的同主题修正：旧设计曾判断“不新增 Makefile target”，本次基于实际不一致反向修正该判断。

## 2. 改动内容
- 新增了什么
  - `Makefile` 新增 `fmt-check`：
    - 执行 `make fmt`。
    - 使用 `git diff --exit-code -- '*.go'` 检查 Go 文件格式漂移。
  - `Makefile` 新增 `quality-check`：
    - 顺序执行 `fmt-check`、`vet`、`test`、`lint`。
    - 作为 CI `quality` job 的统一入口。
  - `Makefile` 新增 `sqlc-check`：
    - 执行 `make sqlc`。
    - 使用 `git diff --exit-code internal/repository/mysql/sqlc` 检查 sqlc generated code 漂移。
  - `Makefile` 新增 `generated-check`：
    - 顺序执行 `sqlc-check` 和 `openapi-check`。
    - 作为 CI `generated-contract` job 的统一入口。
- 修改了什么
  - `Makefile` 将 `openapi-check` 从 prerequisite 形式改成显式顺序执行：
    - `make openapi-validate`
    - `make openapi-generate`
    - `git diff --exit-code api/openapi/gen/eventhub.gen.go`
  - `.github/workflows/ci.yml`：
    - `quality` job 改为调用 `make quality-check`。
    - `generated-contract` job 改为调用 `make generated-check`。
    - workflow 不再直接写 `git diff` 漂移检查细节。
  - `README.md`：
    - 补充 `fmt-check`、`quality-check`、`sqlc-check`、`generated-check`。
    - 说明 `*-check` 目标会先执行对应格式化或生成命令，再通过 `git diff --exit-code` 暴露未提交漂移。
  - `docs/ai/design/019-ci-workflow.md`：
    - 更新背景、目标、非目标、影响范围、领域建模、关键流程、测试策略和替代方案。
    - 明确本次从“CI 自己分散 diff”转向“Makefile 统一 check 入口”。
  - `docs/ai/parity/java-go-parity-matrix.md`：
    - 更新“容器化、部署配置与质量门禁”行，加入 `quality-check`、`generated-check` 和统一 check 入口说明。
- 删除了什么
  - 未删除文件。
  - CI 删除了分散的 `Format Go files`、`Check formatting drift`、`Generate sqlc code`、`Check sqlc generated code drift` 等步骤，改由 Makefile check 目标承载。
- 是否更新 Java-Go parity 记录
  - 已更新。
  - 本次不改变 Java-Go 业务契约，但质量门禁属于 Go-only 工程能力，需要继续在 parity matrix 中索引。

## 3. 为什么这样设计
- 关键设计原因
  - `fmt-check`、`sqlc-check`、`openapi-check` 的行为模型一致：先让工具规范化输出，再通过 `git diff --exit-code` 确认仓库已提交规范化结果。
  - 把 check 语义放在 `Makefile`，可以让本地和 CI 使用同一入口，减少 YAML 与本地命令漂移。
  - 保留 `fmt`、`sqlc`、`openapi-generate` 作为“会修改文件”的维护命令；新增 `*-check` 作为“验证是否有漂移”的入口，职责更清楚。
  - `quality` 保持原本本地维护语义，不改变开发者已有习惯；CI 使用 `quality-check` 获得漂移检查。
  - `generated-check` 只聚合 sqlc 和 OpenAPI，不把 Docker build 混进同一目标，保留 CI job 拆分带来的失败定位清晰度。
- 与 Go 项目当前阶段的匹配点
  - 不修改 Go package、handler/service/repository/domain 分层。
  - 不引入新依赖；继续使用现有 `go run module@version` 工具策略和固定版本 golangci-lint 策略。
  - `Makefile` 仍是工程质量门禁入口，GitHub Actions 只做远端编排。
- 与 Java 版业务语义的对齐方式
  - Java 参考仓库当前没有需要逐行迁移的 GitHub Actions 文件。
  - 本次对齐的是工程质量意图：Go 端的格式、静态检查、测试、lint、sqlc 生成物、OpenAPI 生成物和 Docker 构建都应可重复验证。

本次未新增 ADR。原因是没有引入新的架构决策；工具版本、lint 策略、OpenAPI spec-first、Docker/Compose 和 CI job 拆分都沿用 ADR-0018、ADR-0020、ADR-0021、ADR-0022 及 019 CI workflow 设计。本次只是修正 check 入口的归属位置。

## 4. 替代方案
- 方案 A
  - 维持现状：`fmt` 和 `sqlc` 在 CI YAML 中继续拆成命令加 `git diff`，OpenAPI 继续使用 `make openapi-check`。
  - 未采用原因：三类漂移检查语义一致，却分散在两处维护；后续调整检查范围时容易再次产生本地与 CI 漂移。
- 方案 B
  - 只新增 `make ci`，把所有质量、生成和 Docker 命令塞进一个入口。
  - 未采用原因：Docker build 与 Go 质量 / 生成物检查耗时、依赖和失败定位不同；当前 CI 的三个 job 拆分更清楚。
- 方案 C
  - 修改 `make quality`，让它直接包含 `fmt-check`。
  - 未采用原因：`make quality` 已是本地常用维护入口，保留“会自动格式化”的行为更符合现有习惯；新增 `quality-check` 可以在不破坏旧入口的情况下满足 CI 验证。
- 方案 D
  - 让 `fmt-check` 使用全仓库 `git diff --exit-code`。
  - 未采用原因：`fmt` 只会改 Go 文件，按 `*.go` pathspec 检查更贴近目标，也避免本地已有 README/docs 等无关脏改导致格式检查误失败。CI 中若生成物漂移，仍会由 `generated-check` 专门暴露。

## 5. 测试与验证
- 跑了哪些测试
  - `make quality-check`
  - `make generated-check`
  - `docker compose config --quiet`
  - `make docker-build`
  - `curl -I --max-time 20 https://auth.docker.io/`
  - `git diff --check`
  - `make -n fmt-check quality-check sqlc-check openapi-check generated-check`
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `make quality-check` 已通过：
    - `fmt-check` 执行 `gofmt -w .`，随后 `git diff --exit-code -- '*.go'` 通过。
    - `go vet ./...` 通过。
    - `go test ./...` 通过。
    - `make lint` 通过，输出 `0 issues.`。
  - `make generated-check` 已通过：
    - `sqlc-check` 执行 `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate`，随后 `git diff --exit-code internal/repository/mysql/sqlc` 通过。
    - `openapi-check` 执行 OpenAPI validate、oapi-codegen generate 和 `git diff --exit-code api/openapi/gen/eventhub.gen.go`，通过。
  - `docker compose config --quiet` 已通过。
  - `git diff --check` 已通过。
  - `make -n fmt-check quality-check sqlc-check openapi-check generated-check` 已确认目标展开顺序符合预期。
  - `make docker-build` 已执行但未通过：BuildKit 在解析 `# syntax=docker/dockerfile:1.7` 时访问 Docker Hub auth endpoint 超时，报错为 `failed to fetch anonymous token` / `TLS handshake timeout`。
- 手工验证了哪些场景
  - 对照 `.github/workflows/ci.yml`，确认 `quality` job 不再直接写 `make fmt` 和 `git diff`，而是调用 `make quality-check`。
  - 对照 `.github/workflows/ci.yml`，确认 `generated-contract` job 不再直接写 sqlc 生成和 diff，而是调用 `make generated-check`。
  - 对照 `Makefile` 干跑输出，确认 `quality-check` 展开为 `fmt-check -> vet -> test -> lint`，`generated-check` 展开为 `sqlc-check -> openapi-check`。
  - 对照 README 质量门禁段落，确认维护命令和 check 命令都对用户可见。
  - 对 Docker build 失败做了外部网络验证：`curl -I --max-time 20 https://auth.docker.io/` 在代理返回 `HTTP/1.0 200 Connection established` 后出现 `LibreSSL SSL_connect: SSL_ERROR_SYSCALL`，说明当前环境到 Docker Hub 认证端点不可用，失败点不在项目 Dockerfile 或 Go 构建。
- Java-Go parity 如何验证
  - 本次不修改 Java-Go API、错误码、数据库模型、JWT claim 或业务流程。
  - 已更新 parity matrix 的工程质量门禁行，记录 Go 端统一 Makefile check 入口。
- 结果如何
  - Go 质量门禁、sqlc/OpenAPI 生成物检查、Compose 静态解析和空白检查均通过。
  - Docker image build 受当前网络访问 Docker Hub 认证端点失败影响未完成；该问题已通过 `curl` 最小探针复现为外部网络/TLS 问题。

## 6. 已知限制
- 当前版本还缺什么
  - CI 仍未运行 `make test-race`。
  - CI 仍未启动完整 Docker Compose stack，也没有外部 HTTP smoke。
  - CI 仍未做 migration up/down 的真实数据库验证、镜像扫描、SBOM、签名或部署。
- 哪些地方后面需要继续演进
  - 如果后续 CI 命令组合进一步稳定，可以新增顶层 `make ci`，但应保留 job 级失败定位能力。
  - 可按耗时和稳定性把 `make test-race` 加到定期 workflow 或独立 job。
  - 可新增 Compose smoke target，在 MySQL/Redis/migrate/app 全部启动后跑 HTTP smoke。
  - 发布阶段可单独设计镜像扫描、SBOM、签名、推送和部署。
- 与 Java 版仍有哪些差距
  - Java 参考仓库当前没有可迁移的 GitHub Actions workflow。
  - Go CI 选择 GitHub Actions + Makefile 编排，是 Go 端工程自动化能力，不对应 Java/Spring 运行时代码。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 项目质量门禁入口更清晰：维护命令用于生成和格式化，`*-check` 命令用于验证漂移。
  - CI YAML 更薄，核心规则沉淀在 Makefile 中，便于面试或复盘时解释本地与 CI 如何保持一致。
- 对微服务 / 云原生演进的影响
  - 后续新增 event/order/payment/inventory 模块时，新增 SQL、OpenAPI 或 Go 文件格式变化会通过统一 check 入口暴露。
  - CI job 仍保留质量、生成契约、Docker 三类边界，后续可以独立扩展各自的验证深度。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 新增 SQL/query 后，本地和 CI 都可使用 `make sqlc-check` 或 `make generated-check` 检查 generated code 是否同步。
  - 新增或修改 OpenAPI 契约后，本地和 CI 都可使用 `make openapi-check` 或 `make generated-check` 检查契约和生成代码。
  - 新增 Go package 后，本地和 CI 都可通过 `make quality-check` 同步执行格式、vet、测试和 lint。
