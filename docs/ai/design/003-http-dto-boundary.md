# HTTP DTO 与 VO 边界规范设计

## 1. 背景
- 当前 Go 版 EventHub 已建立 HTTP foundation 和项目 package layout 规则，后续 auth、user、event、reservation、order、ticket 等模块会持续新增 HTTP 请求和响应结构体。
- Java 版项目中常见 `VO` 命名可表达 View Object 或响应展示对象，但在 Go 版中如果直接创建 `internal/http/vo`，容易与 DDD Value Object 混淆。
- Go 版已经有 `internal/http/response` 承载统一 `APIResponse` envelope 和响应写出工具；如果再把具体业务响应放入该包，会混淆统一响应外壳与业务 data DTO 的职责。
- Java 版对应语义来源包括 request DTO、response DTO 和 VO 命名习惯；Go 版需要对齐 HTTP 契约语义，而不是复制 Java package 命名。

## 2. 目标
- 固化 Go 版 HTTP DTO / VO / domain value object 的边界规范。
- 明确本项目不设置 `internal/http/vo`。
- 明确 HTTP request body、query 参数对象、path 参数辅助对象、HTTP response data、list item / summary / detail response 对象统一放 `internal/http/dto`。
- 明确 `internal/http/response` 只放统一响应 envelope 和 writer。
- 明确 DDD Value Object 放 `internal/domain/<domain>` 或 `internal/domain/common`，不放 HTTP 层。
- 更新 `AGENTS.md`、backend-design-first skill、`docs/ai/README.md`、ADR 和 parity matrix，让后续 Codex 任务持续遵守。
- 成功标准：
  - 文档中可以清楚检索到 `HTTP DTO`、`Value Object` 和禁止 `internal/http/vo` 的规则。
  - 后续 handler 使用 DTO，service 使用 Command / Query，domain 不承担 HTTP JSON 契约职责。
  - 不新增业务代码、不创建空 Go package。

## 3. 非目标
- 本次不实现任何业务接口。
- 本次不新增或迁移 Go DTO 代码。
- 本次不创建 `internal/http/dto`、`internal/http/vo` 或其他空 Go package。
- 本次不改变现有 HTTP response envelope 的运行时行为。
- 本次不修改 OpenAPI 契约、数据库 schema、sqlc query 或 migration。

## 4. 影响范围
- 本次实际修改目录：
  - 仓库根目录下的 `AGENTS.md`
  - `.agents/skills/backend-design-first/`
  - `docs/ai/`
- 本次不新建、不修改运行时代码目录：
  - `cmd/`
  - `internal/app/`
  - `internal/config/`
  - `internal/platform/`
  - `internal/http/`
  - `internal/apperror/`
  - `internal/page/`
  - `internal/domain/`
  - `internal/service/`
  - `internal/repository/`
  - `internal/security/`
  - `api/openapi/`
  - `migrations/`
  - `configs/`
- 规则涉及的长期 Go package / 模块：
  - `internal/http/dto`：HTTP request/response data 结构体。
  - `internal/http/response`：统一 `APIResponse` envelope 和 writer。
  - `internal/domain/<domain>`、`internal/domain/common`：domain model 和 DDD Value Object。
  - `internal/service/<domain>`：Command / Query 和业务用例。
  - `internal/repository/mysql/sqlc`：sqlc generated model，仅在 repository 边界内被包装。
- 涉及 API / 表 / 缓存 / 外部接口：
  - 无。本次只修改协作规则和文档，不改变运行时 API、数据库、缓存或外部接口。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 是。本次记录 Java VO 命名习惯到 Go HTTP DTO / domain value object 边界的刻意差异。

## 5. 领域建模
- 核心对象：
  - `HTTP DTO`：HTTP 入参和出参的数据契约，包括 request body、query、path 参数辅助对象和 response data。
  - `Response envelope`：统一 API 响应外壳，例如 `APIResponse`。
  - `Response writer`：统一响应写出工具，例如 `WriteSuccess`、`WriteError`。
  - `Domain model`：表达业务实体、聚合状态或业务枚举。
  - `Domain Value Object`：DDD 意义上的值对象，例如 `Email`、`Money`。
  - `Service Command / Query`：service 用例输入，不依赖 HTTP DTO。
- 实体关系：
  - handler 依赖 `internal/http/dto` 和 `internal/http/response`。
  - handler 将 HTTP DTO 映射为 service Command / Query。
  - service 返回业务结果或 domain model，不拼 HTTP JSON。
  - handler 将 service result / domain model 映射为 HTTP response DTO。
  - repository/mysql 将 sqlc row 映射为 domain model，不向 handler 暴露 sqlc generated model。
- 关键状态：
  - `已决策`：不创建 `internal/http/vo`，Java VO 语义在 Go 版按用途归入 HTTP DTO 或 domain Value Object。
  - `需 ADR 说明`：如果未来偏离本规范，必须在设计文档和 ADR 中说明原因。

## 6. API 设计
- 本次不新增或修改运行时 API。
- 本次新增的规则会约束未来 API 设计：
  - HTTP 请求体类型命名推荐 `XxxRequest`。
  - HTTP 响应 data 类型命名推荐 `XxxResponse`。
  - 列表项响应类型命名推荐 `XxxListItemResponse`。
  - 汇总响应类型命名推荐 `XxxSummaryResponse`。
  - 详情响应类型命名推荐 `XxxDetailResponse`。
  - 不推荐 `XxxVO`、`XxxDTO` 和 `XxxResp`，除非外部生成代码或兼容需求明确要求。
- 错误码 / 异常场景：
  - 本次不改变错误码。
  - `internal/http/response` 继续负责统一成功 / 失败响应写出。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Go 版对齐请求字段、响应字段和状态语义，但不逐字照搬 Java `VO` 命名。
  - Java 中 View Object 语义在 Go 版归入 HTTP DTO；DDD Value Object 语义在 Go 版归入 domain。

## 7. 数据设计
- 本次不新增数据库表、索引、唯一约束、migration 或 sqlc query。
- 本次新增的规则会约束未来数据边界：
  - sqlc generated model 不能作为 HTTP DTO 对外暴露。
  - `repository/mysql` 负责 sqlc row 与 domain model 的映射。
  - domain model 不应因为 HTTP JSON 输出需要而直接携带传输层职责。
- 数据一致性考虑：
  - 本次不涉及运行时数据一致性。

## 8. 关键流程
- 正常流程：
  1. handler decode 并 validate HTTP DTO。
  2. handler 将 DTO 映射为 service Command / Query。
  3. service 执行业务规则、事务边界和状态决策。
  4. repository/mysql 将 sqlc row 映射为 domain model。
  5. service 返回业务结果或 domain model。
  6. handler 将结果映射为 HTTP response DTO。
  7. handler 调用 `response.WriteSuccess` 或 `response.WriteError`。
- 异常流程：
  - 如果新增具体业务 response，被放入 `internal/http/response`，需要改回 `internal/http/dto`。
  - 如果想创建 `vo`，默认禁止；HTTP 展示对象改为 `XxxResponse`，DDD Value Object 放 domain。
  - 如果 service 直接依赖 `internal/http/dto`，应改为 Command / Query。
  - 如果 domain 带 `json` tag，默认避免；若确有设计理由，需要记录。
  - 如果 sqlc generated model 暴露到 handler 或 DTO，必须改由 repository/mysql 映射。
- 状态流转：
  - `VO 命名未固化 -> 设计明确 -> AGENTS/skill/README 更新 -> ADR accepted -> parity 已记录 -> 后续任务按 DTO boundary check 执行`。
- handler / service / repository / sqlc/database 分工：
  - handler：HTTP DTO、校验、映射、响应写出。
  - service：业务规则、事务边界、Command / Query。
  - repository：持久化语义接口。
  - repository/mysql：sqlc row 与 domain model 映射。
  - sqlc/database：生成查询代码，不承载业务判断。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及运行时并发、幂等或缓存。
- 规则层面明确：
  - DTO 不承载并发、幂等或缓存策略。
  - 幂等、防重复提交、事务边界和缓存策略由 service 设计承载。
  - 缓存返回结构如需对外暴露，也应先映射为 HTTP DTO。

## 10. 权限与安全
- 本次不实现认证或授权代码。
- 规则层面明确：
  - HTTP DTO 可以表达请求中的认证上下文外部字段，但 JWT claim、Principal、安全上下文仍属于 `internal/security`。
  - 不因为 response DTO 需要就把角色、邮箱、用户名、用户状态写入 JWT。
  - domain model 和 domain value object 不承担 HTTP JSON 契约职责，避免敏感字段被误暴露。
- 不涉及敏感信息、审计或操作日志实现。

## 11. 测试策略
- 本次需要运行的验证：
  - `grep -R "internal/http/vo" AGENTS.md .agents docs README.md || true`
  - `grep -R "HTTP DTO" AGENTS.md .agents docs README.md`
  - `grep -R "Value Object" AGENTS.md .agents docs README.md`
  - `go test ./...`
  - `go vet ./...`
  - `git diff --check`
- 单元测试：
  - 不新增运行时代码，无新增单元测试。
- service / repository 测试：
  - 不适用，本次不新增 service 或 repository。
- migration / sqlc 验证：
  - 不适用，本次没有 SQL、schema、sqlc 或 migration 变化。
- 接口验证 / OpenAPI validate：
  - 不适用，本次没有 API 契约变化。
- Java-Go parity 验证：
  - 对照 Java request DTO、response DTO 和 VO 命名习惯，确认 Go 版文档记录为 HTTP DTO / domain Value Object 两类边界。

## 12. 风险与替代方案
- 当前方案的风险：
  - 规则先于大量业务 DTO 落地，短期主要依赖 Codex 遵守文档约束。
  - 如果未来 OpenAPI 生成代码引入 `DTO` 后缀，可能需要在设计文档中说明兼容例外。
  - 如果 domain model 为内部序列化临时携带 `json` tag，需要额外说明避免被误认为 HTTP DTO。
- 备选方案：
  - 方案 A：创建 `internal/http/vo` 放响应展示对象。
  - 方案 B：拆分 `internal/http/request` 和 `internal/http/response` 存业务请求/响应。
  - 方案 C：请求和响应结构体统一放 `internal/http/dto`。
- 为什么不选备选方案：
  - 不选方案 A：`VO` 同时可能表示 View Object 和 Value Object，长期容易混淆。
  - 不选方案 B：`internal/http/response` 已用于统一响应 envelope 和 writer，继续放业务 response 会让职责不清。
  - 选择方案 C：HTTP 传输契约集中管理，命名通过 `Request`、`Response`、`Item`、`Summary`、`Detail` 后缀表达用途，同时保持 domain / service 不依赖 HTTP。
- 后续可演进点：
  - 新增 OpenAPI schema 后，可继续验证 schema 名称与 DTO 命名约定是否一致。
  - 引入具体 auth/user/event/order DTO 时，implementation note 必须写明 DTO 与 service command/domain model 的映射关系。
