# EventHub Go

EventHub Go 是 Java 版 EventHub 的 Go port，用于用 Go 生态自然写法复刻 Java 版的业务语义、API 契约、错误码、数据库模型、测试策略和文档沉淀方式。

Java 版参考项目：

```text
/Users/xinnz/Library/Mobile Documents/com~apple~CloudDocs/Code/Java/eventhub
```

## 当前阶段

- 已完成 HTTP foundation：应用入口、router、server、requestId、recover、统一响应、错误码、分页、system ping/echo/health/info。
- 已完成项目结构规范化：应用装配进入 `internal/app`，system HTTP DTO 进入 `internal/http/dto`，system service 进入 `internal/service/system`，request id 进入 `internal/platform/idgen`。
- 业务模块、数据库、migration、sqlc、OpenAPI、Docker、认证授权、活动、订单、支付等仍待迁移。

## 本地运行

```bash
go run ./cmd/eventhub
```

默认监听 `:8080`。可通过环境变量覆盖：

- `EVENTHUB_APP_NAME`
- `EVENTHUB_ENV`
- `EVENTHUB_HTTP_PORT`
- `EVENTHUB_VERSION`
- `EVENTHUB_LOG_LEVEL`

## 验证

```bash
gofmt -w .
go test ./...
go vet ./...
```

也可以使用 Makefile：

```bash
make fmt
make test
make vet
```

## 目录结构

```text
cmd/eventhub/              可执行入口，保持极薄
internal/app/              应用装配与生命周期
internal/config/           环境变量配置、profile 和对外配置结构
internal/http/             router、server、middleware、handler、dto、response、validation
internal/service/system/   当前 system 基础能力服务
internal/apperror/         错误码、应用错误和错误映射
internal/page/             分页请求与响应模型
internal/platform/         clock、idgen、log 等跨业务基础设施
docs/ai/                   设计、实现说明、ADR、Java-Go parity matrix
api/openapi/               OpenAPI 契约未来落点
migrations/                数据库 migration 未来落点
configs/                   环境变量示例
```

## 文档纪律

非微小修改必须先更新 `docs/ai/design/`，实现后更新 `docs/ai/implementation/`，关键取舍写入 `docs/ai/adr/`，语义、契约、模型、测试或 Go-only 结构差异写入 `docs/ai/parity/java-go-parity-matrix.md`。
