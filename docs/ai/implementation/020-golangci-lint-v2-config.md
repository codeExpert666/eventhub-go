# golangci-lint v2 配置迁移实现说明

## 1. 本次改动解决了什么问题

本次将仓库 `.golangci.yml` 从 golangci-lint v1 配置风格迁移到 v2 配置风格，并同步升级 Makefile、README 和 ADR 中的固定执行版本。随后补充 Makefile 本机版本探测：只有本机 `golangci-lint` 版本匹配固定版本时才使用本机工具，否则自动回退 Docker fallback。

改动前，配置仍使用 `linters.disable-all: true`，并把 `gofmt` 放在 `linters.enable` 中；这在 v2 中已经不是推荐结构。若只迁移配置而继续用 Makefile 固定的 `v1.64.8` runner，会导致 `make lint` 入口和配置格式不匹配。本次同时升级 runner 到 `v2.12.2`，并让 Makefile 对本机工具做固定版本校验，避免旧版或非固定版本工具带来配置兼容问题和 lint 结果漂移。

## 2. 改动内容
- 新增了什么
  - 新增 `docs/ai/design/020-golangci-lint-v2-config.md`。
  - 新增 `docs/ai/implementation/020-golangci-lint-v2-config.md`。
- 修改了什么
  - `.golangci.yml`
    - 新增 `version: "2"`。
    - 将 `linters.disable-all: true` 改为 `linters.default: none`。
    - 将 `gofmt` 从 `linters.enable` 移到 `formatters.enable`。
    - 删除 v1 字段 `issues.exclude-use-default: false`；v2 默认不启用预置排除规则，语义保持一致。
  - `Makefile`
    - 将 `GOLANGCI_LINT_VERSION` 从 `v1.64.8` 升级到 `v2.12.2`。
    - 新增 `GOLANGCI_LINT_EXPECTED_VERSION`，从固定版本号中去掉前缀 `v`，用于匹配 `golangci-lint version` 输出。
    - `lint` 目标在发现本机工具时先读取版本；版本匹配才执行本机 `golangci-lint run ./...`，版本不匹配则打印提示并回退 Docker 镜像。
  - `README.md`
    - 更新本机安装命令为 v2 module path。
    - 更新 Docker fallback 镜像版本。
    - 补充 `.golangci.yml` 使用 v2 配置格式，且本机安装也应使用固定版本。
  - `docs/ai/adr/0022-golangci-lint-quality-gate.md`
    - 更新固定版本和配置结构说明。
    - 保留本机优先、Docker fallback 和低噪音规则集的原决策。
  - `docs/ai/parity/java-go-parity-matrix.md`
    - 在既有“容器化、部署配置与质量门禁”行补充固定 golangci-lint v2 版本、v2 配置格式和 020 文档索引。
- 删除了什么
  - 未删除文件。
- 是否更新 Java-Go parity 记录
  - 已轻量更新既有质量门禁索引行。
  - 本次不改变 Java-Go 业务契约、API、错误码、数据库模型或测试覆盖意图。

## 3. 为什么这样设计
- 关键设计原因
  - golangci-lint v2 将 `disable-all` 改为 `default: none`，将 `gofmt` 等格式检查迁到 `formatters`；按 v2 schema 表达可以减少后续升级成本。
  - 配置格式和 runner 固定版本必须一起迁移，否则 Makefile / CI 入口容易失败或产生不同机器的 lint 结果漂移。
  - 保持原规则集不变，避免把“配置格式迁移”和“规则策略调整”混在一起。
- 与 Go 项目当前阶段的匹配点
  - 当前 Go 版仍处于业务 parity 持续迁移阶段，低噪音 lint 规则更适合保护基础质量而不引入风格噪音。
  - Makefile 继续使用本机优先、Docker fallback，符合 ADR-0022 的开发体验取舍。
- 与 Java 版业务语义的对齐方式
  - Java 版没有对应 golangci-lint 配置。
  - 本次仅维护 Go-only 工程质量门禁，不影响 Java-Go 业务语义对齐。

## 4. 替代方案
- 方案 A
  - 只更新 `.golangci.yml`，不更新 Makefile 固定版本。
  - 未采用原因：v2 配置可能被 v1 runner 读取失败，`make lint` 不稳定。
- 方案 B
  - 保留 v1 配置，暂不迁移。
  - 未采用原因：不满足本次“新版风格”目标，也会让配置继续停留在旧结构。
- 方案 C
  - 迁移 v2 的同时启用更多规则。
  - 未采用原因：新增规则属于质量策略变化，会带来额外噪音；本次只做等价迁移。
- 方案 D
  - Makefile 总是使用 Docker v2，不再优先本机工具。
  - 未采用原因：会降低已安装 v2 工具开发者的本地体验，且偏离既有 ADR 的本机优先策略。
- 方案 E
  - 只检查本机工具是否为 v2，不要求匹配固定版本。
  - 未采用原因：仓库已采用固定版本策略；只检查主版本仍可能让不同机器使用不同 v2 小版本，带来 lint 结果漂移。

## 5. 测试与验证
- 跑了哪些测试
  - `docker run --rm -v "$(pwd):/app" -w /app golangci/golangci-lint:v2.12.2 golangci-lint config verify`
  - 临时 fake `golangci-lint` 红灯验证：模拟本机 `v1.64.8` 时，旧 Makefile 会错误调用本机工具并以 `Error 42` 失败。
  - 临时 fake `golangci-lint` 绿灯验证：模拟本机 `v1.64.8` 时，新 Makefile 打印版本不匹配提示并调用 Docker fallback。
  - 临时 fake `golangci-lint` 绿灯验证：模拟本机 `v2.12.2` 时，新 Makefile 调用本机 `golangci-lint run ./...`，不调用 Docker。
  - 临时 fake Docker 绿灯验证：模拟没有本机 `golangci-lint` 时，新 Makefile 继续调用 Docker fallback。
  - `make lint`
  - `go vet ./...`
  - `go test ./...`
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - v2 配置校验通过，Docker 首次拉取 `golangci/golangci-lint:v2.12.2` 后命令退出码为 0。
  - `make lint` 通过，输出 `0 issues.`。
  - `go vet ./...` 通过，无输出。
  - `go test ./...` 通过。
  - 未运行 `gofmt`：本次未修改 Go 文件。
  - 未运行 `sqlc generate`：本次未修改 SQL、migration、sqlc query 或 `sqlc.yaml`。
  - 未运行 OpenAPI validate/generate：本次未修改 OpenAPI 契约。
- 手工验证了哪些场景
  - 确认 `.golangci.yml` 使用 v2 schema，且原 lint 规则集保持不变。
  - 确认 Makefile 和 README 固定版本与 v2 配置格式一致。
  - 确认 Makefile 在本机工具版本不匹配时不会继续执行本机旧工具。
  - 确认 ADR-0022 仍表达本机优先、Docker fallback、低噪音规则集，只更新版本和配置结构。
- Java-Go parity 如何验证
  - 确认不涉及 Java 业务契约。
  - 在既有质量门禁 parity 行补充 v2 配置迁移索引。
- 结果如何
  - 所有本次相关验证命令均通过。

## 6. 已知限制
- 当前版本还缺什么
  - Makefile 版本探测依赖 `golangci-lint version` 输出包含 `version <固定版本>`；这是官方当前输出格式，后续若上游输出格式改变，需要同步调整匹配逻辑。
- 哪些地方后面需要继续演进
  - 后续业务模块稳定后，可评估逐步启用 `errcheck`、`revive`、`gocritic` 或 `gosec`。
- 与 Java 版仍有哪些差距
  - 不适用。Java 版没有 golangci-lint 配置。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 质量门禁配置跟上 golangci-lint v2，README 和 Makefile 可直接指导新环境运行。
- 对微服务 / 云原生演进的影响
  - CI 和本地 lint 入口继续保持固定版本和 Docker fallback，便于后续模块拆分前保持一致的 Go 代码质量基线。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 不改变 Go package 边界、migration、sqlc 或 OpenAPI 策略。
  - 后续若调整 lint 规则，应单独记录规则策略变化，而不是混在配置格式迁移中。
