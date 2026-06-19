# GitHub Actions CI Workflow 实现说明

## 1. 本次改动解决了什么问题

本次为 Go 版 EventHub 新增 GitHub Actions CI workflow，让 pull request 和 `main` 分支 push 能自动执行核心质量门禁、生成代码漂移检查、OpenAPI 契约检查和 Docker 构建校验。

改动前，仓库已经有 `Makefile`、固定版本 golangci-lint、sqlc/OpenAPI 生成命令、Dockerfile 和 Docker Compose，但这些能力只存在于本地手工执行流程里。新增 CI 后，远端仓库可以用同一组 Makefile 目标验证 Go 代码、生成物和容器化基线，降低本地与 CI 门禁漂移风险。

## 2. 改动内容
- 新增了什么
  - 新增 `.github/workflows/ci.yml`。
    - `pull_request` 触发。
    - `push` 到 `main` 触发。
    - `permissions: contents: read`。
    - `concurrency` 使用 workflow 名称和 `github.ref`，同一 ref 新运行取消旧运行。
    - `quality` job 运行 `go mod download`、`make fmt`、`git diff --exit-code`、`make vet`、`make test`、`make lint`。
    - `generated-contract` job 运行 `make sqlc`、`git diff --exit-code internal/repository/mysql/sqlc`、`make openapi-check`。
    - `docker` job 运行 `docker compose config --quiet`、`make docker-build`。
  - 新增 `docs/ai/design/019-ci-workflow.md`。
  - 新增 `docs/ai/implementation/019-ci-workflow.md`。
- 修改了什么
  - 更新 `docs/ai/parity/java-go-parity-matrix.md`：
    - 将 `.github/workflows/ci.yml` 纳入“容器化、部署配置与质量门禁”Go 目标。
    - 增加本次 design / implementation note 索引。
- 删除了什么
  - 未删除文件。
- 是否更新 Java-Go parity 记录
  - 已更新。
  - 本次不改变 Java-Go 业务契约，但 CI 属于 Go-only 工程质量门禁能力，按 AGENTS.md 要求需要纳入 parity matrix。

## 3. 为什么这样设计
- 关键设计原因
  - workflow 复用现有 Makefile 目标，避免把工具版本、命令参数和生成逻辑复制到 GitHub Actions 中。
  - `make fmt` 会原地修改 Go 文件，因此 CI 紧跟 `git diff --exit-code`，显式暴露格式漂移。
  - sqlc 与 OpenAPI 生成物分别做 drift 检查，避免 query、schema 或契约修改后漏提交 generated code。
  - Docker job 不启动完整 Compose stack，只做 `docker compose config --quiet` 和 `make docker-build`，能覆盖容器配置和镜像构建，同时避免 PR 校验被长时间服务启动、端口占用和清理流程拖重。
  - workflow 权限只给 `contents: read`，因为 CI 不需要写仓库、发包、推镜像或访问生产资源。
- 与 Go 项目当前阶段的匹配点
  - 不改业务分层，不新增 handler/service/repository/domain 代码。
  - 继承 ADR-0020 的 Docker runtime 策略、ADR-0021 的显式 migration 策略、ADR-0022 的固定 golangci-lint 质量门禁策略。
  - `actions/setup-go` 使用 `go-version-file: go.mod`，让 CI 跟随仓库声明的 Go 版本。
- 与 Java 版业务语义的对齐方式
  - Java 参考仓库当前未发现 `.github/workflows`，本次不逐行迁移 Java CI。
  - 本次对齐的是工程质量意图：容器化、OpenAPI hardening、migration/sqlc 生成物和质量门禁都必须在 Go 端可重复验证。

## 4. 替代方案
- 方案 A
  - CI 只运行 `make quality`。
  - 未采用原因：`make quality` 不覆盖 sqlc generated drift、OpenAPI generated drift、Compose 配置解析和 Docker build；同时 `make fmt` 原地修改文件后还需要 workflow 额外 diff 检查。
- 方案 B
  - 新增 Makefile `ci` 或 `check` target，把所有 CI 命令集中进 Makefile。
  - 未采用原因：现有 Makefile 目标已经覆盖本次检查，workflow 只需编排；新增 target 会增加一个需要维护的入口，当前收益不明显。
- 方案 C
  - CI 中运行 `make compose-up` 并做 HTTP smoke。
  - 未采用原因：完整 Compose 会拉取 MySQL/Redis/migrate/app，涉及端口、健康等待和清理，当前先用 compose config + Docker build 覆盖容器化基线；完整 smoke 后续可作为独立设计演进。
- 方案 D
  - 每次 PR 都运行 `make test-race`。
  - 未采用原因：race 检查成本更高，本次先建立核心 CI；后续可按耗时和稳定性评估加到定期 workflow 或独立 job。
- 方案 E
  - Docker build 只在 `push main` 运行，不阻塞 PR。
  - 未采用原因：Dockerfile 是当前工程基线的一部分，PR 阶段阻塞能更早发现运行镜像构建问题。

本次未新增 ADR。原因是 CI 分层、Docker build 阻塞 PR、工具安装策略都沿用既有 ADR-0020、ADR-0021、ADR-0022 和 `docs/ai/design/016-docker-and-dev-workflow.md` 的决策，本次只是把这些已决策能力编排进 GitHub Actions，没有引入新的关键架构取舍。

## 5. 测试与验证
- 跑了哪些测试
  - `make fmt`
  - `git diff --exit-code`
  - `make vet`
  - `make test`
  - `make lint`
  - `make sqlc`
  - `git diff --exit-code internal/repository/mysql/sqlc`
  - `make openapi-check`
  - `docker compose config --quiet`
  - `make docker-build`
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `make fmt` 执行 `gofmt -w .`，通过；随后 `git diff --exit-code` 通过，说明没有已跟踪 Go 文件格式漂移。
  - `make vet` 执行 `go vet ./...`，通过。
  - `make test` 执行 `go test ./...`，通过。
  - `make lint` 通过。
  - `make sqlc` 执行 `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate`，通过；随后 `git diff --exit-code internal/repository/mysql/sqlc` 通过，说明 sqlc generated code 无漂移。
  - `make openapi-check` 执行 OpenAPI validate、oapi-codegen generate 和 `git diff --exit-code api/openapi/gen/eventhub.gen.go`，通过。
  - `docker compose config --quiet` 通过。
  - `make docker-build` 通过，成功构建 `eventhub-go:local`。
- 手工验证了哪些场景
  - 读取 `.github/workflows/ci.yml`，确认 workflow 触发、最小权限、concurrency、三个 job 和 Makefile 命令符合设计。
  - 确认 `quality` 没有直接用 `make quality` 代替分步检查，保留 `make fmt` 后的 `git diff --exit-code`。
  - 确认 `generated-contract` 对 sqlc 和 OpenAPI generated code 都有 drift 检查。
  - 确认 `docker` 不运行 `make compose-up`，只做静态 compose config 和 image build。
- Java-Go parity 如何验证
  - Java 参考仓库当前未发现 `.github/workflows`。
  - 对照 Go 版 016 设计/实现和 ADR-0020/0021/0022，确认 CI 只是把已决策的工程质量门禁纳入远端自动化。
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`。
- 结果如何
  - 所有要求的验证命令均通过。

## 6. 已知限制
- 当前版本还缺什么
  - CI 尚未运行 `make test-race`。
  - CI 尚未启动完整 Docker Compose stack，也没有外部 HTTP smoke。
  - CI 尚未做 migration up/down 的真实数据库验证、镜像扫描、SBOM、签名或部署。
- 哪些地方后面需要继续演进
  - 当 CI 命令组合稳定后，可考虑新增 `make ci`，让本地和远端入口进一步集中。
  - 可按耗时和稳定性把 `make test-race` 加到定期 workflow 或独立 job。
  - 可新增 Compose smoke target，在 MySQL/Redis/migrate/app 全部启动后跑 HTTP smoke。
  - 发布阶段可单独设计镜像扫描、SBOM、签名、推送和部署。
- 与 Java 版仍有哪些差距
  - Java 参考仓库当前没有可迁移的 GitHub Actions workflow。
  - Go CI 选择 GitHub Actions + Makefile 编排，是 Go 端工程自动化能力，不对应 Java/Spring 运行时代码。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 项目具备远端可见的 CI 质量门禁，能展示 Go 后端工程化闭环：格式、vet、测试、lint、sqlc、OpenAPI、Docker。
- 对微服务 / 云原生演进的影响
  - CI 已为后续 migration、OpenAPI、容器镜像和发布流水线打下自动化入口。
  - 显式区分质量校验、生成契约校验和 Docker 校验，后续可以独立扩展对应 job。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 新增 SQL/query 后，CI 会通过 `make sqlc` 和 generated code diff 暴露漏提交。
  - 新增或修改 OpenAPI 契约后，CI 会通过 `make openapi-check` 暴露契约或生成代码问题。
  - Dockerfile、docker-compose.yml 或 Go 编译问题会在 PR 阶段暴露。
