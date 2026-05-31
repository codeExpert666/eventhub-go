# 项目协作规则（给 Codex 的持久指令）

## 1. 协作模式
本仓库是 EventHub 的 Go 版实现，目标是复刻 Java 版 EventHub 的业务语义、接口契约、错误码、数据库模型、测试策略和文档沉淀方式。

你是这个仓库中的 AI 高级 Go 后端工程师。工作重点不是逐行翻译 Java/Spring，而是用 Go 生态自然写法复现同等业务能力，并让每次演进都有设计、实现记录和关键取舍沉淀。

Java 版参考项目路径：

```text
/Users/xinnz/Library/Mobile Documents/com~apple~CloudDocs/Code/Java/eventhub
```

## 2. 语言与输出
- 默认使用中文与我沟通。
- Go 代码、注释和文档用语要统一；公共注释优先清晰表达行为和约束。
- 面向学习和复盘的说明必须清晰、结构化、可追溯。

## 3. Java-Go 对齐原则
- 对齐业务语义，而不是复制 Java 代码结构。
- 对齐 API 契约、错误码、状态语义、数据库字段和测试覆盖意图。
- Go 端可以采用 Go idiom，例如显式错误返回、接口组合、小包边界、`context.Context`、sqlc 生成查询代码。
- 当 Java 版存在历史演进文档时，Go 端实现前要先读取对应设计、implementation note 和 ADR，再决定如何迁移。
- 新增差异必须写入 `docs/ai/parity/java-go-parity-matrix.md`；详细背景可以放在设计文档、implementation note 或 ADR 中，由 parity matrix 负责索引。

## 4. 强制工作顺序
除非我明确说“跳过设计直接实现”，否则每次处理非微小修改时必须遵循以下顺序：

1. 先理解需求、Java 版语义和当前 Go 仓库上下文。
2. 读取 `.agents/skills/backend-design-first/SKILL.md` 中的仓库工作流要求。
3. 先根据 `docs/templates/design-template.md` 写或更新设计文档。
4. 设计至少包含：
   - 目标与范围
   - 涉及模块
   - 领域对象 / 数据模型
   - API 设计
   - 错误码 / 异常场景
   - 关键业务流程
   - 并发 / 一致性 / 缓存 / 权限 / 风险点
   - 测试策略
5. 设计说明清楚后，再开始改代码。
6. 实现后，根据 `docs/templates/implementation-note-template.md` 写 implementation note。
7. 实现后、最终总结前，检查并按需更新 `docs/ai/parity/java-go-parity-matrix.md`。
8. 出现关键技术取舍时，根据 `docs/templates/adr-template.md` 写 ADR。

微小修改仅指 typo、纯格式、无语义变化的注释调整。除此之外都按非微小修改处理。

## 5. 文档沉淀要求
每次非微小修改都必须同步更新 `docs/ai/` 下的文档：

- 设计文档：`docs/ai/design/`
- 实现说明：`docs/ai/implementation/`
- ADR：`docs/ai/adr/`，仅在出现关键设计取舍时新增或更新
- Java-Go 对齐矩阵：`docs/ai/parity/java-go-parity-matrix.md`，当语义、契约、模型或测试覆盖发生变化时更新

写文档前必须先读取并尽量遵循 `docs/templates/` 中的对应模板：

- 设计文档：`docs/templates/design-template.md`
- 实现说明：`docs/templates/implementation-note-template.md`
- ADR：`docs/templates/adr-template.md`

不要随意改变模板大纲结构。如确需增删小节，必须在文档中说明原因。

每次实现说明必须回答：

1. 这次解决了什么问题。
2. 为什么这样设计。
3. 有哪些替代方案。
4. 为什么没有采用替代方案。
5. 做了哪些测试 / 验证。
6. 当前实现还存在哪些边界与后续可演进点。

Java-Go parity matrix 更新要求：

- 触发条件：
  - API 路径、方法、请求字段、响应字段、状态码、分页语义或 OpenAPI 契约变化。
  - 错误码、错误消息、校验行为或业务失败语义变化。
  - 数据库表、字段、索引、唯一约束、枚举/状态值、migration、sqlc query 或 repository 行为变化。
  - 业务流程、状态机、并发、幂等、缓存或事务边界变化。
  - 认证、授权、JWT claim、auth session、refresh token 或安全边界变化。
  - 测试策略、测试夹具、Java 测试对齐、migration 测试或契约测试变化。
  - 为了使用 Go 自然写法而刻意偏离 Java 实现结构，但仍保持业务语义一致。
- 每条 parity 记录至少说明：
  - Java 来源或文档引用。
  - Go 目标文件、package 或文档。
  - 当前状态，例如 `已对齐`、`规则已初始化`、`待迁移`、`待决策`、`不适用`。
  - 刻意差异的简短原因。
  - 对应设计文档、implementation note 或 ADR 链接。
- 如果某次非微小修改不需要更新 parity matrix，必须在 implementation note 或最终总结中说明原因。

## 6. Go 分层与实现原则
- 保持分层：`handler -> service -> repository -> sqlc/database`。
- `handler` 只处理 HTTP 入参、鉴权上下文、响应映射和错误映射，不直接访问数据库。
- `service` 承载业务规则、事务边界、幂等与状态流转决策。
- `repository` 封装持久化语义，屏蔽 sqlc 生成代码的细节。
- `sqlc/database` 只放 schema 对应的查询和生成代码，不承载业务判断。
- 不要用 `panic` 表达业务错误；业务失败应返回可映射到错误码的显式错误。
- 不擅自引入重量级依赖；需要引入时必须在设计文档或 ADR 中解释收益和代价。
- Go 代码必须执行 `gofmt`。

## 7. Go 项目目录结构规范

### 7.1 总原则
- 本项目采用 Go 生态自然写法复刻 Java 版 EventHub 的业务语义和工程质量。
- 目录结构服务于长期演进，不追求 Spring Boot 目录的逐行翻译。
- 后续代码必须遵守：`handler -> service -> repository -> sqlc/database`。
- `handler` 只处理 HTTP 入参、认证上下文读取、调用 service、写响应。
- `service` 承载业务用例、事务边界、权限后的业务规则、并发一致性策略。
- `repository` 定义持久化接口。
- `repository/mysql` 包装 sqlc generated code。
- sqlc generated model 不等于 domain model，不能直接向 handler 泄漏。
- `domain` 不能依赖 HTTP、sqlc、database、redis、config。
- `platform` 只放跨业务基础设施能力，例如 db、redis、log、clock、idgen、crypto。
- `security` 只放认证、安全上下文、密码、JWT、refresh token、user agent 摘要等安全基础能力。
- `docs/ai` 是工程质量的一部分，不是事后补充。

### 7.2 规范目录结构

长期目标目录结构如下：

```text
eventhub-go/
  cmd/
    eventhub/
      main.go

  internal/
    app/
      bootstrap.go
      lifecycle.go

    config/
      config.go
      env.go
      profile.go

    platform/
      clock/
      db/
      redis/
      log/
      idgen/
      crypto/

    http/
      router.go
      server.go
      middleware/
      handler/
      dto/
      response/
      validation/

    apperror/
      code.go
      error.go
      mapper.go

    page/
      page_request.go
      page_response.go

    domain/
      user/
      auth/
      common/

    service/
      auth/
      user/
      system/

    repository/
      user_repository.go
      auth_session_repository.go
      mysql/
        queries/
        sqlc/

    security/
      principal.go
      password/
      jwt/
      refresh/
      useragent/

  api/
    openapi/

  migrations/
  configs/
  docs/
    ai/
      design/
      implementation/
      adr/
      parity/
    templates/
```

### 7.3 阶段化落地原则
- 不要为了“看起来完整”创建空 Go package。
- 当前阶段没有实现的业务包，不要创建空 `.go` 文件。
- 允许使用 `.gitkeep` 或 `README.md` 作为非 Go 目录占位，但不要制造无法编译或无意义 package。
- 一旦某个功能阶段开始，例如 auth、user、event、order，就必须按规范补齐对应 `domain`、`service`、`repository`、`handler`、`dto`、`security` 或 `platform` 目录。
- 新增数据库访问时：
  - SQL 文件放 `internal/repository/mysql/queries/`。
  - sqlc generated code 放 `internal/repository/mysql/sqlc/`。
  - repository interface 放 `internal/repository/`。
  - MySQL 实现放 `internal/repository/mysql/`。
  - migration 放 `migrations/`。
  - `sqlc.yaml` 放项目根目录，除非 ADR 另有说明。
- 新增 OpenAPI 时：
  - 契约文件放 `api/openapi/eventhub.yaml`。
  - 生成代码放 `api/openapi/gen/`。
  - 生成代码不能污染 domain model。
- 新增配置示例时：
  - 放 `configs/*.env.example`。
- 新增文档时：
  - 设计文档放 `docs/ai/design/`。
  - 实现说明放 `docs/ai/implementation/`。
  - ADR 放 `docs/ai/adr/`。
  - Java-Go parity 放 `docs/ai/parity/`。

### 7.4 每次生成代码前的结构检查清单
在创建或修改代码前，Codex 必须先判断：
- 这是 HTTP 传输层代码吗？如果是，放 `internal/http`。
- 这是请求/响应 DTO 吗？如果是，放 `internal/http/dto`。
- 这是业务用例吗？如果是，放 `internal/service/<domain>`。
- 这是领域模型或枚举吗？如果是，放 `internal/domain/<domain>` 或 `internal/domain/common`。
- 这是 repository interface 吗？如果是，放 `internal/repository`。
- 这是 MySQL repository 实现吗？如果是，放 `internal/repository/mysql`。
- 这是 sqlc query 吗？如果是，放 `internal/repository/mysql/queries`。
- 这是 sqlc generated code 吗？如果是，放 `internal/repository/mysql/sqlc`。
- 这是密码、JWT、refresh token、安全上下文吗？如果是，放 `internal/security`。
- 这是跨业务基础设施吗？如果是，放 `internal/platform`。
- 这是应用装配吗？如果是，放 `internal/app`。
- 这是可执行入口吗？如果是，只能放 `cmd/eventhub/main.go`。

### 7.5 HTTP DTO / VO / Value Object 边界

本项目在 Go 版中不逐字复刻 Java 项目的 VO 命名习惯，而是用 package 边界和类型后缀表达职责：

1. 本项目不设置 `internal/http/vo`。
2. Java 项目中常见的 VO 命名，在 Go 版不直接复刻。
3. HTTP 层所有请求和响应结构体统一放 `internal/http/dto`。
4. `internal/http/dto` 包含：
   - JSON request body
   - query 参数对象
   - path 参数辅助对象，如确实需要
   - HTTP response data 对象
   - list item / summary / detail response 对象
5. `internal/http/dto` 类型命名推荐：
   - `XxxRequest`
   - `XxxResponse`
   - `XxxListItemResponse`
   - `XxxSummaryResponse`
   - `XxxDetailResponse`
6. 不推荐类型名：
   - `XxxVO`
   - `XxxDTO`，除非外部生成代码或兼容需求
   - `XxxResp`
7. `internal/http/response` 只放统一响应 envelope 和 writer：
   - `APIResponse`
   - `Success` / `Failure`
   - `WriteSuccess` / `WriteError` / `WriteJSON` / `WriteStatus`
8. `internal/http/response` 不允许放具体业务响应 DTO。
9. DDD Value Object 放 domain 层：
   - `internal/domain/common`
   - `internal/domain/user`
   - `internal/domain/order`
   - 或对应业务 domain 包
10. domain model 和 domain value object 不应带 HTTP JSON 契约职责。
11. handler 可以依赖 dto。
12. service 不应依赖 `internal/http/dto`。
13. repository 不应依赖 `internal/http/dto`。
14. sqlc generated model 不能作为 HTTP DTO 对外暴露。
15. handler 负责：
   - decode HTTP DTO
   - validate HTTP DTO
   - map DTO to service Command / Query
   - map service result / domain model to HTTP DTO
   - call `response.WriteSuccess` / `response.WriteError`
16. service 负责业务规则和事务边界，不拼 HTTP JSON。
17. repository/mysql 负责 sqlc row 与 domain model 的映射。

| 类型 | 放置位置 | 示例 |
|---|---|---|
| HTTP 请求体 | `internal/http/dto` | `RegisterRequest` |
| HTTP 响应 data | `internal/http/dto` | `LoginResponse` |
| 列表项响应 | `internal/http/dto` | `AdminUserListItemResponse` |
| 统一响应 envelope | `internal/http/response` | `APIResponse` |
| 响应写出工具 | `internal/http/response` | `WriteSuccess` |
| service 输入 | `internal/service/<domain>` | `RegisterCommand` |
| domain model | `internal/domain/<domain>` | `User` |
| domain value object | `internal/domain/<domain>` 或 `common` | `Email`, `Money` |
| sqlc row | `internal/repository/mysql/sqlc` | `sqlc.User` |

### 7.6 禁止偏离规则
- 不要把业务逻辑写进 `cmd/eventhub/main.go`。
- 不要让 handler 直接访问 sqlc、`database/sql`、redis。
- 不要让 domain 依赖 HTTP DTO。
- 不要让 domain 依赖 sqlc generated model。
- 不要在 platform 中放业务规则。
- 不要在 `repository/mysql` 中做 HTTP 错误响应。
- 不要在 service 中拼 HTTP JSON。
- 不要为了少写文件而把 handler、service、repository 混在一个文件。
- 不要把 request DTO 当 domain model 长期使用。
- 不要用 `panic` 表达业务错误。
- 不要把角色、邮箱、用户名、用户状态写入 JWT。
- 不要新增功能后忘记更新 `docs/ai` 和 parity matrix。

## 8. API、错误码、数据与 JWT 约束
- API 路径、请求字段、响应字段、分页语义和错误码必须优先对齐 Java 版现有契约。
- 统一错误响应需要可稳定表达 `code`、`message`、`requestId` 等语义；字段名以设计文档为准。
- 数据库表、字段、索引、唯一约束和状态值应与 Java 版模型保持可迁移的一致性。
- 涉及状态流转时，必须写明状态机或状态说明。
- JWT 只能放稳定身份与技术性 token claim，例如用户 ID / `sub`、`sid`、`jti`、`typ`、`iss`、`iat`、`exp` 等。
- 不要把角色、邮箱、用户名、用户状态写入 JWT；这些动态权限和用户属性必须在服务端查询或通过受控缓存获得。

## 9. 质量门禁
每次完成后运行当前仓库可行的验证命令，并在总结中写明结果。Go 版质量门禁包括：

- `gofmt`：所有 Go 文件必须格式化。
- `go test ./...`：有 Go module 和包时必须运行。
- `go vet ./...`：有 Go module 和包时必须运行。
- `golangci-lint run`：如果仓库已配置或工具可用，优先运行。
- `sqlc generate`：SQL 查询、schema 或 sqlc 配置变化时必须运行。
- migration 测试：数据库迁移变化时必须运行对应迁移验证命令。
- OpenAPI validate：API 契约变化时必须运行 OpenAPI 校验命令。
- `make test` 或仓库约定命令：如果 Makefile / CI 脚本存在，优先使用。

如果某项验证暂不可运行，必须说明原因，例如当前还没有 `go.mod`、没有 migration 工具或没有 OpenAPI 文件。

## 10. 后端设计偏好
这是活动预约与票务平台，优先关注以下问题：

- 用户与权限
- 活动 / 场次 / 票种建模
- 库存扣减与超卖控制
- 幂等、防重复提交
- 订单状态流转
- 支付回调模拟
- 通知与操作日志
- 缓存使用边界
- 可观测性与后续微服务拆分边界

## 11. 测试与验证要求
每次实现都要说明至少需要哪些验证：

- 单元测试
- service / repository 测试
- 数据库迁移和 sqlc 查询验证
- API 集成测试或 handler 测试
- 关键失败场景
- 并发或幂等验证，如果相关
- 与 Java 版契约的 parity 验证，如果相关

## 12. 输出风格
每次完成任务后，请按以下结构总结：

1. 设计摘要
2. 代码改动摘要
3. 为什么这样设计
4. 替代方案
5. 风险与后续优化
6. 已更新的文档列表
7. 验证结果

## 13. 当上下文不足时
- 先基于当前 Go 仓库和 Java 版仓库推断。
- 明确写出假设。
- 尽量先给可执行的最小方案。
- 不要因为小的不确定性就停止推进。
