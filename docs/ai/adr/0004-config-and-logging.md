# 配置与日志 ADR

## 标题
Go 版 EventHub 基础阶段采用环境变量配置和 slog 结构化日志

## 状态
- accepted

## 背景
Java 版 EventHub 使用 Spring Boot `application.yml`、profile 和 Logback/MDC 管理配置、环境差异和 requestId 日志字段。

Go 版当前处于 HTTP 工程底座阶段，需要具备：

- dev/test/prod 环境雏形。
- 应用名、端口、版本、日志级别等基础配置。
- requestId 写入日志字段。
- 与标准库 HTTP、context 和 httptest 兼容。

本次不接数据库、Redis、JWT 或外部配置中心，不需要引入完整配置框架。

## 决策
Go 版 EventHub 基础阶段采用：

- `internal/config` 从环境变量读取配置，并提供保守默认值。
- 环境变量：
  - `EVENTHUB_APP_NAME`
  - `EVENTHUB_ENV`
  - `EVENTHUB_HTTP_PORT`
  - `EVENTHUB_VERSION`
  - `EVENTHUB_LOG_LEVEL`
- 环境取值限制为 `dev/test/prod`，未知值回退到 `dev`。
- `prod` 下日志级别不低于 `INFO`。
- `internal/platform/log` 使用标准库 `log/slog` JSON handler。
- requestId 通过 `context.Context` 传递，并在 middleware 日志中作为 `requestId` 字段输出。

## 备选方案
- 方案 1：引入 Viper 或 Koanf 读取 YAML/TOML/env。
- 方案 2：引入 Zap、Zerolog 或 Logrus。
- 方案 3：只用 `log.Printf` 输出纯文本日志。
- 方案 4：使用标准库环境变量读取和 `slog`。

## 决策理由
选择当前方案的原因：

- 当前配置项很少，标准库足够表达 dev/test/prod 雏形，避免过早引入配置框架。
- `slog` 是 Go 标准库结构化日志方案，能直接输出 JSON，并支持 `InfoContext/ErrorContext`。
- requestId 使用 context 比模拟 Java MDC 更符合 Go HTTP 请求生命周期。
- 测试中可以用 `io.Discard` logger，避免输出噪音。
- 未来接入数据库、Redis、JWT 密钥、配置文件或配置中心时，可以在 `internal/config` 内部演进，不影响 handler/service 外部契约。

## 影响
- 好处：
  - 工程底座依赖少，启动和测试简单。
  - 日志字段稳定包含 `service/env/requestId`，便于后续接入日志平台。
  - 配置读取逻辑集中，后续扩展不会散落在 handler 中。
- 代价：
  - 暂不支持 YAML/TOML 多层配置文件。
  - 暂不支持动态配置刷新。
  - `slog` 的高级采样、异步写入和字段脱敏能力需要后续按需扩展。
- 后续可能需要调整的地方：
  - 数据库、Redis、JWT、支付模拟等配置接入后，需要补充校验和 prod 快速失败规则。
  - 生产可观测性增强时，可以在保持 `slog.Logger` 接口边界的基础上替换 handler 或增加日志采样。
  - 若引入配置文件，需要新增 ADR 或更新本 ADR，说明与环境变量覆盖顺序的关系。
