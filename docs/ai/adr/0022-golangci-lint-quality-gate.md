# ADR：golangci-lint 质量门禁固定 v2 版本并提供 Docker fallback

## 标题
使用固定 v2 版本 golangci-lint，并在本机未安装时通过 Docker 镜像执行 lint

## 状态
- accepted

## 背景
Go 版仓库早期已经新增 `.golangci.yml`，但历史 implementation note 多次记录当前机器未安装 `golangci-lint`，导致 lint 目标无法稳定执行。

本次需要补齐本地开发命令和质量门禁。`quality` 目标必须能串联核心检查，而 lint 又不能依赖“每台机器都提前装好同一个版本”的隐含前提。另一方面，golangci-lint 规则如果一次性打开过多，会在当前业务迁移阶段产生大量风格噪音，干扰 Java-Go parity 的主要目标。

2026-06-09 更新：`.golangci.yml` 已迁移到 golangci-lint v2 配置格式，因此固定执行版本同步升级到 v2，避免 v2 配置被 v1 runner 读取失败。

## 决策
Go 版采用以下 lint 策略：

- 固定 golangci-lint 版本为 `v2.12.2`。
- `.golangci.yml` 使用低噪音规则集：
  - `gofmt`
  - `govet`
  - `ineffassign`
  - `staticcheck`
  - `unused`
- `.golangci.yml` 使用 v2 配置格式：
  - `linters.default: none`，只显式开启当前阶段需要的 lint 规则。
  - `formatters.enable: [gofmt]`，保留 gofmt 格式检查。
- Makefile `lint` 优先执行本机 `golangci-lint run ./...`。
- 如果本机没有 `golangci-lint`，或本机版本不匹配固定版本，Makefile 使用固定 Docker 镜像 `golangci/golangci-lint:v2.12.2`。
- Makefile `quality` 串联 `fmt -> vet -> test -> lint`。
- README 说明本机安装命令，也说明不安装时 `make lint` 会自动走 Docker fallback。

## 备选方案
- 方案 1：不引入 golangci-lint，只使用 `go vet`。
- 方案 2：要求所有开发者必须本机安装 golangci-lint。
- 方案 3：Makefile 总是使用 Docker 执行 lint。
- 方案 4：使用固定版本，本机优先，Docker fallback。
- 方案 5：一次性启用大量规则，如 revive、gocyclo、forbidigo、exhaustive 等。
- 方案 6：迁移 v2 配置但保持 Makefile 固定 v1 runner。
- 方案 7：只要本机工具是 v2 就直接使用，不要求匹配固定版本。

## 决策理由
选择方案 4：

- 固定版本避免不同机器 lint 结果漂移。
- 本机优先能利用开发者已有工具，速度更快。
- Docker fallback 让未安装工具的环境仍可执行质量门禁。
- 低噪音规则覆盖格式、明显静态错误、无效赋值和未使用代码，适合当前 Go 后端迁移阶段。
- `quality` 串联核心门禁后，README、CI 和人工验证可以使用同一入口。

没有选择其他方案：

- 不选只用 `go vet`：覆盖不足，不能替代 staticcheck/unused。
- 不选强制本机安装：会重现历史“工具未安装，lint 未运行”的问题。
- 不选总是 Docker：对已安装工具的开发者更慢，也不利于 IDE/本机集成。
- 不选大量规则：当前阶段业务 parity 和分层边界更重要，高噪音风格规则后续可逐步引入。
- 不选 v2 配置搭配 v1 runner：会让本地和 CI lint 入口因配置格式不兼容而不稳定。
- 不选任意 v2 版本：仓库质量门禁采用固定版本策略，只检查主版本仍可能造成不同机器 lint 结果漂移。

## 影响
- 好处
  - lint 可重复执行，版本稳定。
  - 新环境不安装 golangci-lint 也能跑 `make lint`。
  - `make quality` 成为本地核心质量入口。
  - 规则集聚焦低噪音问题。
- 代价
  - Docker fallback 需要 Docker 可用，首次拉取镜像较慢。
  - 低噪音规则不覆盖所有风格、复杂度或安全检查。
  - Docker 镜像版本后续需要手动升级。
  - 本机如果安装的不是固定版本，Makefile 会回退 Docker，首次运行可能比直接本机执行更慢。
- 后续可能需要调整的地方
  - CI 可固定执行 `make quality`。
  - 当业务模块稳定后，可评估逐步开启 `errcheck`、`revive`、`gocritic` 或复杂度规则。
  - 升级 Go toolchain 时同步评估 golangci-lint 版本兼容性。
