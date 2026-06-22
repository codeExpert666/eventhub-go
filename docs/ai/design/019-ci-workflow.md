# GitHub Actions CI Workflow 设计

## 1. 背景
- 当前 Go 版 EventHub 已具备 `Makefile` 质量门禁、固定版本 golangci-lint、sqlc 生成、OpenAPI 契约生成校验、Dockerfile 和 Docker Compose，但仓库还没有 `.github/workflows`，无法在 pull request 或 `main` push 时自动验证这些工程能力。
- 2026-06-20 更新：CI 初版中 `fmt` 和 `sqlc` 漂移检查拆成“先运行维护命令，再在 workflow 里直接执行 `git diff`”，而 OpenAPI 已由 `make openapi-check` 封装 validate/generate/diff。三类检查语义相同却分布在不同位置，容易让本地与 CI 入口再次漂移，因此需要把 check 规则统一沉回 `Makefile`。
- 2026-06-23 更新：GitHub-hosted runner 开始默认将 JavaScript action 迁移到 Node 24 运行时，仓库现有 `actions/checkout@v4` 和 `actions/setup-go@v5` 仍声明 Node 20，导致三个 CI job 都出现 Node 20 deprecation warning。需要升级到声明 Node 24 的官方 action 大版本。
- Java 版参考仓库当前未发现 `.github/workflows`，因此本次不迁移 Java CI 文件；语义来源主要是 Java 版已有容器化 / profile / OpenAPI hardening 文档，以及 Go 版已沉淀的工程质量设计：
  - Java `backend/Dockerfile`、`docker-compose.yml`、prod OpenAPI hardening ADR。
  - Go `docs/ai/design/016-docker-and-dev-workflow.md`、`docs/ai/implementation/016-docker-and-dev-workflow.md`。
  - Go ADR-0020、ADR-0021、ADR-0022。
- 业务上下文是活动预约与票务平台的工程底座：后续 event/order/payment/inventory 模块会增加更多 migration、sqlc query、OpenAPI 契约和并发测试，CI 需要尽早把本地质量门禁固化为远端可重复执行的流水线。

## 2. 目标
- 新增 `.github/workflows/ci.yml`，触发：
  - `pull_request`
  - `push` 到 `main`
- workflow 使用最小权限：
  - `permissions: contents: read`
- 增加 `concurrency`，同一 ref 上新运行会取消旧运行，避免重复消耗 CI 资源。
- 复用仓库已有 `Makefile` 目标和固定工具版本，避免 GitHub Actions 与本地门禁漂移。
- 新增 `Makefile` check 入口，统一表达“先格式化 / 生成，再检查是否有未提交漂移”的规则：
  - `make fmt-check`
  - `make sqlc-check`
  - `make generated-check`
  - `make quality-check`
- CI 分为三个 job：
  - `quality`：Go 格式、vet、测试、lint。
  - `generated-contract`：sqlc 生成代码漂移、OpenAPI validate/generate/check。
  - `docker`：Compose 配置静态校验和 Docker image build。
- GitHub 官方 JavaScript action 使用 Node 24 兼容版本：
  - `actions/checkout@v6`
  - `actions/setup-go@v6`
- 成功标准：
  - Go 代码格式漂移能通过 `make fmt-check` 暴露。
  - sqlc generated code 漂移能通过 `make sqlc-check` 暴露。
  - OpenAPI generated code 漂移能通过 `make openapi-check` 暴露。
  - CI 中生成物漂移统一通过 `make generated-check` 覆盖 sqlc 和 OpenAPI。
  - Dockerfile 能完成 `make docker-build`。
  - Compose 文件能通过 `docker compose config --quiet` 静态解析。
  - GitHub Actions 运行记录不再出现 `actions/checkout@v4`、`actions/setup-go@v5` 触发的 Node 20 deprecation warning。

## 3. 非目标
- 不新增业务代码。
- 不修改 handler/service/repository/domain 分层。
- 不修改 API 路径、请求字段、响应字段、错误码、JWT claim、数据库 schema、migration 或 sqlc query。
- 不做发布、部署、镜像推送、PR 自动评论、制品上传、镜像扫描、SBOM 或签名。
- 默认不在 CI 中运行 `make compose-up`，避免 PR 校验引入长时间运行服务、端口占用、健康等待和清理复杂度。
- 默认不在本次 CI 中运行 `make test-race`，先把核心门禁固化；race 检查作为后续成本评估后的增强项。
- 不新增顶层 `make ci` 或发布流水线；本次只补齐质量门禁和生成物漂移 check 入口。
- 不改变 `make quality` 的本地维护语义；它仍会执行可修复格式化的 `make fmt`。CI 和本地只读式验证使用新增的 `make quality-check`。
- 不修改 Go toolchain 版本；`setup-go` 仍从 `go.mod` 读取 Go 版本。
- 不使用 `ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION` 临时回退 Node 20，也不使用 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24` 掩盖旧 action 版本。

## 4. 影响范围
- 涉及 Go package / 模块：
  - 不触碰 Go package。
  - 不新增、移动或删除业务 Go 文件。
- 涉及文件：
  - 新增 / 更新 `.github/workflows/ci.yml`。
  - 更新 `Makefile`。
  - 更新 `README.md` 的质量门禁说明。
  - 新增 / 更新 `docs/ai/design/019-ci-workflow.md`。
  - 新增 / 更新 `docs/ai/implementation/019-ci-workflow.md`。
  - 更新 `docs/ai/parity/java-go-parity-matrix.md` 的“容器化、部署配置与质量门禁”行。
  - 2026-06-23 Node 24 action runtime 维护只更新 `.github/workflows/ci.yml` 和上述 `docs/ai` 记录，不修改 `Makefile` 或 `README.md`。
- 涉及 API / 表 / 缓存 / 外部接口：
  - 不涉及生产 API、数据库表、Redis 缓存或外部业务接口。
  - 涉及外部 CI 平台 GitHub Actions。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 是。CI 属于 Go-only 工程质量门禁能力，需要纳入容器化、部署配置与质量门禁索引。

## 5. 领域建模
- `CIWorkflow`
  - GitHub Actions workflow 文件，负责在 PR 和 `main` push 时编排质量检查。
  - 不承载业务规则。
- `QualityJob`
  - 运行 Go toolchain 与 Makefile 门禁。
  - 依赖 `actions/checkout@v6`、`actions/setup-go@v6` 和 Go module cache。
- `GeneratedContractJob`
  - 验证 sqlc generated code 与 OpenAPI generated code 没有漂移。
  - sqlc 以 `sqlc.yaml`、`migrations/`、`internal/repository/mysql/queries/` 为输入。
  - OpenAPI 以 `api/openapi/eventhub.yaml` 为契约源。
- `DockerJob`
  - 验证 `docker-compose.yml` 可解析，`Dockerfile` 可构建。
  - 不启动完整 Compose stack。
- `MakefileCheckTarget`
  - `fmt-check`：执行 `make fmt` 后检查 Go 文件是否产生 diff。
  - `sqlc-check`：执行 `make sqlc` 后检查 `internal/repository/mysql/sqlc` 是否产生 diff。
  - `openapi-check`：执行 OpenAPI validate/generate 后检查 `api/openapi/gen/eventhub.gen.go` 是否产生 diff。
  - `generated-check`：聚合 sqlc 与 OpenAPI 生成物漂移检查。
  - `quality-check`：聚合 `fmt-check -> vet -> test -> lint`，作为 CI 的质量 job 入口。
- 与 Java 版领域对象的对应关系：
  - Java Maven/Gradle CI 概念不逐行迁移。
  - Go 版以 Makefile 和 Go toolchain 表达同等工程质量闭环。

## 6. API 设计
- 本次不新增或修改 HTTP API。
- CI workflow 触发契约：
  - `on.pull_request`：所有 PR 触发。
  - `on.push.branches: [main]`：推送到 `main` 触发。
- workflow 权限：
  - `contents: read`，checkout 仓库内容即可。
- workflow 并发：
  - group 使用 workflow 名称和 `github.ref`。
  - `cancel-in-progress: true`，同一 PR ref 或 `main` ref 的旧运行会被取消。
- workflow action runtime：
  - `Checkout` 使用 `actions/checkout@v6`，避免继续引用 Node 20 版本的官方 checkout action。
  - `Setup Go` 使用 `actions/setup-go@v6`，保持 `go-version-file: go.mod` 和 Go cache，同时避免继续引用 Node 20 版本的 setup-go action。
- 错误码 / 异常场景：
  - 不新增应用错误码。
  - CI 失败由 GitHub Actions job exit code 表达。
- 与 Java 版 OpenAPI / controller 契约差异：
  - 不涉及业务接口契约。

## 7. 数据设计
- 表结构调整：无。
- 索引设计：无。
- 唯一约束：无。
- migration 计划：无。
- sqlc query / generated model 影响：
  - 不修改 sqlc query 或 generated code。
  - `make sqlc-check` 会执行 `make sqlc` 并检查 `internal/repository/mysql/sqlc` 是否有 diff，确保生成结果被提交。
- 数据一致性考虑：
  - 不涉及业务数据读写。
  - repository integration tests 中 Testcontainers MySQL 仍由 `make test` 触发；Docker 不可用时测试按当前逻辑 skip。

## 8. 关键流程
- `quality` 正常流程：
  1. 通过 `actions/checkout@v6` checkout 仓库。
  2. 通过 `actions/setup-go@v6` setup Go，使用 `go-version-file: go.mod`，开启 Go cache。
  3. `go mod download`。
  4. `make quality-check`：
     - `fmt-check` 执行 `gofmt -w .` 并用 `git diff --exit-code -- '*.go'` 捕获 Go 格式漂移。
     - `vet` 执行 `go vet ./...`。
     - `test` 执行 `go test ./...`。
     - `lint` 复用 Makefile 固定 golangci-lint 版本和 Docker fallback。
- `generated-contract` 正常流程：
  1. 通过 `actions/checkout@v6` checkout 仓库。
  2. 通过 `actions/setup-go@v6` setup Go，使用 `go-version-file: go.mod`，开启 Go cache。
  3. `go mod download`。
  4. `make generated-check`：
     - `sqlc-check` 执行 `sqlc generate` 并检查 `internal/repository/mysql/sqlc` 漂移。
     - `openapi-check` 执行 OpenAPI validate、generate 和 generated file diff。
- `docker` 正常流程：
  1. 通过 `actions/checkout@v6` checkout 仓库。
  2. `docker compose config --quiet`。
  3. `make docker-build`。
- 异常流程：
  - `make fmt` 修改文件后，`git diff --exit-code` 失败，提示开发者提交格式化结果。
  - `make sqlc` 修改 generated code 后，sqlc diff 检查失败。
  - `make openapi-check` 修改 `api/openapi/gen/eventhub.gen.go` 后失败。
  - Dockerfile 或 compose 配置不可解析时，docker job 失败。
- 状态流转：
  - 不涉及业务状态机。
  - 只涉及 CI job queued -> in_progress -> success/failure/cancelled。
- handler / service / repository / sqlc/database 分工：
  - 不改变分工。
  - CI 只调用现有命令验证这些分层已有测试和生成物。

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险：无，本次不涉及库存或订单。
- 如何防重复提交：
  - GitHub Actions 使用 `concurrency` 取消同一 ref 的旧运行。
  - 不涉及业务接口幂等。
- 事务边界在哪里：
  - 不改变 service 或 repository 事务边界。
- 缓存放在哪里，为什么：
  - 使用 `actions/setup-go@v6` 的 Go module/build cache，降低 CI 重复下载成本。
  - 不新增业务缓存。

## 10. 权限与安全
- 哪些角色能访问：
  - GitHub Actions 运行由 GitHub 仓库事件触发；本次不设计应用内角色访问。
- 鉴权与鉴别约束：
  - workflow 不使用 GitHub token 写权限，不推送、不发布、不创建 PR。
- JWT claim 边界：
  - 不修改 JWT。
  - 不把角色、邮箱、用户名、用户状态写入 JWT。
- 是否涉及敏感信息、审计或操作日志：
  - 不新增 secrets。
  - `permissions: contents: read`，最小化 workflow token 权限。
  - 不启用 `ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION`，避免在 Node 20 EOL 后继续依赖不安全运行时。
  - 不执行部署、镜像推送或生产资源访问。
  - CI 日志会包含 Makefile 命令输出，但不应包含业务密钥；Docker build 使用仓库默认本地配置，不注入生产 secret。

## 11. 测试策略
- 单元测试：
  - CI 中由 `make test` 覆盖现有 Go 单元测试和可运行的集成测试。
- service / repository 测试：
  - CI 中由 `make test` 覆盖；Testcontainers MySQL 依赖 Docker，Docker 可用时运行，Docker 不可用时按测试逻辑 skip。
- migration / sqlc 验证：
  - CI 中执行 `make sqlc-check`，由 Makefile 负责运行 sqlc 并检查 `internal/repository/mysql/sqlc` diff。
  - 本次不新增 migration up/down CI job；后续可单独评估真实 MySQL migration 验证成本。
- 接口验证：
  - CI 中由 `make test` 覆盖现有 handler / HTTP integration 测试。
- OpenAPI validate：
  - CI 中执行 `make openapi-check`。
- 异常场景验证：
  - `make fmt-check` 覆盖 Go 格式漂移。
  - `make sqlc-check` 覆盖 sqlc generated code 漂移。
  - `make openapi-check` 覆盖 OpenAPI generated code 漂移。
  - `docker compose config --quiet` 覆盖 compose YAML 和服务依赖结构可解析。
- Java-Go parity 验证：
  - 本次不改变 Java-Go 业务契约。
  - parity matrix 记录 Go CI workflow 作为质量门禁工程能力。
- 需要运行的命令：
  - `make quality-check`
  - `make generated-check`
  - `make openapi-check`
  - `make sqlc-check`
  - `docker compose config --quiet`
  - `make docker-build`
  - `git diff --check`

## 12. 风险与替代方案
- 当前方案的风险：
  - `actions/checkout@v6` 和 `actions/setup-go@v6` 都要求 runner 支持 Node 24；GitHub-hosted `ubuntu-latest` 满足该前提，自托管 runner 需要先升级到兼容版本。
  - `actions/checkout@v6` 调整了凭据存放方式；当前 workflow 只读 checkout，不在后续步骤执行 authenticated git 写操作，因此影响很小。
  - `make lint` 的 Docker fallback 首次可能拉取 golangci-lint 镜像，耗时较长。
  - `make test` 中的 Testcontainers 集成测试依赖 Docker；GitHub-hosted runner 通常可用，但若 Docker provider 不可用，测试会按当前设计 skip，覆盖深度会下降。
  - `make sqlc` 和 `make openapi-check` 都通过 `go run` 固定版本工具，首次运行需要下载工具依赖。
  - `docker-build` 作为 PR 阻塞 job 会增加 CI 时间，但能及早发现 Dockerfile 与应用编译问题。
- 备选方案：
  - 方案 A：只运行 `make quality`。
  - 方案 B：在 CI 中运行 `make compose-up` 并做 HTTP smoke。
  - 方案 C：把 `test-race` 纳入每次 PR。
  - 方案 D：继续在 GitHub Actions YAML 中分散写 `git diff`，不新增 `fmt-check`、`sqlc-check` 等 Makefile 入口。
  - 方案 E：Docker build 只在 `push main` 跑，不阻塞 PR。
  - 方案 F：新增顶层 `make ci`，把 quality、generated 和 docker 全部收进一个入口。
  - 方案 G：保留 `checkout@v4` / `setup-go@v5`，通过环境变量临时允许 Node 20。
  - 方案 H：只升级到 `checkout@v5`，暂不使用最新 `checkout@v6`。
- 为什么不选备选方案：
  - 不选方案 A：`make quality` 不覆盖 sqlc/OpenAPI generated drift 和 Dockerfile/Compose 校验；且 `make fmt` 会原地修改文件，需要 workflow 额外 `git diff --exit-code`。
  - 不选方案 B：完整 Compose 启动会拉取 MySQL/Redis/migrate，涉及端口、健康等待和清理，当前更适合后续独立 smoke target。
  - 不选方案 C：race 检查成本更高，当前先建立核心 CI；后续可按耗时和稳定性评估加入 nightly 或独立 job。
  - 不选方案 D：fmt/sqlc/OpenAPI 的漂移检查语义一致，继续把其中两类检查留在 workflow 中会让本地和 CI 使用不同入口。
  - 不选方案 E：Dockerfile 是当前工程能力的一部分，PR 阶段阻塞能更早暴露运行镜像编译失败。
  - 不选方案 F：Docker build 与 Go 质量 / 生成物检查属于不同耗时和依赖层级；当前保留 job 级拆分，让 CI 日志和失败定位更清楚。
  - 不选方案 G：Node 20 已进入 EOL 和移除流程，临时回退只能延后问题，后续 runner 移除 Node 20 后仍会失败。
  - 不选方案 H：`checkout@v6` 是当前官方 checkout 主版本；当前 workflow 没有依赖旧版凭据写入行为，直接跟进最新主版本更简单。
- 后续可演进点：
  - 增加 `make ci`，当 CI 命令组合稳定后让本地和远端入口进一步集中。
  - 增加 `test-race` 的定期 workflow 或按标签触发的 job。
  - 增加 migration up/down CI 验证或 Compose smoke target。
  - 增加 Docker image 扫描、SBOM、签名和部署流水线，但应作为后续发布设计处理。
