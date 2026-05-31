# Go 项目目录结构规范设计

## 1. 背景
- 当前 Go 版 EventHub 已完成 HTTP 工程底座，但后续 auth、user、event、order、payment、数据库、OpenAPI 等业务与工程模块尚未迁移。
- Java 版对应语义来自 Controller、Service、Mapper、Entity、Config、Security 等分层，以及 Java 版协作规则中对设计、实现说明、ADR 和 parity 的要求。
- Go 版目标是用 Go 生态自然写法复刻 Java 版业务语义和工程质量，而不是逐行复制 Spring Boot 的目录结构。
- 现有 `AGENTS.md` 已约束 `handler -> service -> repository -> sqlc/database`，但还缺少更细的长期 package layout、阶段化落地原则和生成代码前的结构检查清单。

## 2. 目标
- 在 `AGENTS.md` 中新增“Go 项目目录结构规范”，固化长期目标目录、分层职责、阶段化落地原则、生成代码前检查清单和禁止偏离规则。
- 在 `.agents/skills/backend-design-first/SKILL.md` 中加入结构规范检查步骤，使每次设计和实现前先判断涉及哪些目录。
- 在 `docs/ai/README.md` 中说明目录结构变化与设计文档、implementation note、ADR、parity matrix 的联动。
- 新增 `docs/ai/adr/0005-go-project-package-layout.md`，记录 Go package layout 的长期决策。
- 更新 `docs/ai/parity/java-go-parity-matrix.md`，新增 Go package layout / 项目目录结构对齐记录。
- 成功标准：
  - 后续 Codex 生成代码时能按 `cmd`、`internal/app`、`internal/http`、`internal/domain`、`internal/service`、`internal/repository`、`internal/platform`、`internal/security` 等边界落目录。
  - 明确不要为了“看起来完整”创建空 Go package。
  - 本次不新增业务代码、不移动现有 package。

## 3. 非目标
- 本次不实现任何业务代码。
- 本次不重构现有 Go 代码目录。
- 本次不创建空 `domain`、`service`、`repository`、`security`、`platform/db` 等 Go package。
- 本次不引入 sqlc、migration、OpenAPI 生成器或新的第三方依赖。
- 本次不直接照搬 Java/Spring 目录结构。

## 4. 影响范围
- 本次实际修改目录：
  - `.agents/skills/backend-design-first/`
  - `docs/ai/`
  - 仓库根目录下的 `AGENTS.md`
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
  - `cmd/eventhub`：规则中明确只承载可执行入口。
  - `internal/app`：长期目标，当前不创建。
  - `internal/config`：已有配置目录，纳入规范。
  - `internal/platform`：已有 `log`，长期容纳 db、redis、clock、idgen、crypto 等跨业务基础设施。
  - `internal/http`：已有 router、server、middleware、handler、response、validation，规则补充 dto 边界。
  - `internal/apperror`：已有错误码与错误映射基础。
  - `internal/page`：已有分页基础。
  - `internal/domain`、`internal/service`、`internal/repository`、`internal/security`：长期目标，当前不创建空 Go package。
  - `api/openapi`、`migrations`、`configs`：长期目标，当前不创建。
  - `docs/ai`：本次实际更新设计、implementation、ADR、parity 和 README。
- 涉及 API / 表 / 缓存 / 外部接口：
  - 无。本次只修改协作规则和文档，不改变运行时 API、数据库、缓存或外部接口。
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`：
  - 是。本次记录 Java 分层到 Go package layout 的映射和刻意差异。

## 5. 领域建模
- 核心实体：
  - `Project package layout`：Go 版长期目录结构目标。
  - `Layer responsibility`：handler、service、repository、repository/mysql、sqlc、domain、platform、security、app、config 等边界职责。
  - `Structure conformance check`：每次设计和实现前的目录归属判断步骤。
  - `Structure debt`：实际目录与长期规范暂未完全一致，但因阶段未到而不创建空 package 的记录。
- 实体关系：
  - `AGENTS.md` 定义项目级目录规范。
  - `backend-design-first` skill 把目录规范转成每次任务的执行步骤。
  - `docs/ai/README.md` 说明目录结构变化如何触发设计、实现说明、ADR 和 parity 更新。
  - ADR 记录为什么选择当前混合式 Go package layout。
  - parity matrix 作为 Java-Go 分层映射索引。
- 关键状态：
  - `已决策`：长期目录结构已写入规则和 ADR。
  - `阶段化落地`：当前阶段只保留已有可编译目录，不创建空 Go package。
  - `需 ADR 说明`：未来若偏离目录规范，必须新增或更新 ADR。

## 6. API 设计
- 本次不新增或修改运行时 API。
- 本次新增的规则会约束未来 API 设计：
  - HTTP 传输层放 `internal/http`。
  - 请求/响应 DTO 放 `internal/http/dto`。
  - 业务用例放 `internal/service/<domain>`。
  - domain model 不依赖 HTTP DTO、sqlc generated model、database、redis 或 config。
  - OpenAPI 契约放 `api/openapi/eventhub.yaml`，生成代码放 `api/openapi/gen/`，生成代码不能污染 domain model。
- 与 Java 版 OpenAPI / controller 契约的差异：
  - Go 版用显式 router、handler、DTO 和 service 调用表达 Controller 边界，不复制 Spring MVC 注解模型。

## 7. 数据设计
- 本次不新增数据库表、索引、唯一约束、migration 或 sqlc query。
- 本次新增的规则会约束未来数据设计：
  - SQL 文件放 `internal/repository/mysql/queries/`。
  - sqlc generated code 放 `internal/repository/mysql/sqlc/`。
  - repository interface 放 `internal/repository/`。
  - MySQL 实现放 `internal/repository/mysql/`。
  - migration 放 `migrations/`。
  - `sqlc.yaml` 放项目根目录，除非 ADR 另有说明。
- 数据一致性考虑：
  - `service` 承载事务边界、幂等、并发一致性和权限后的业务规则。
  - `repository/mysql` 只包装 sqlc generated code，不承载 HTTP 响应或业务状态机。

## 8. 关键流程
- 正常流程：
  1. Codex 在非微小修改前读取 `AGENTS.md` 和 `backend-design-first` skill。
  2. 先进行结构规范检查，列出涉及和不涉及的目录。
  3. 在设计文档中写明目录影响、是否新建目录、是否移动 package。
  4. 按规范目录实现最小可行改动。
  5. implementation note 列出文件移动和 package 边界变化。
  6. 如偏离规范，新增或更新 ADR。
  7. 更新 parity matrix，记录 Java 分层到 Go 目录的映射。
- 异常流程：
  - 如果当前阶段没有实现某业务包，不创建空 `.go` 文件。
  - 如果目录规范与实际业务需求冲突，在设计文档和 ADR 中说明原因后再偏离。
- 状态流转：
  - `规则未细化 -> 设计明确 -> AGENTS/skill/README 更新 -> ADR accepted -> parity 已记录 -> 后续任务按结构检查执行`。
- handler / service / repository / sqlc/database 分工：
  - 本次只固化规则，不新增运行时调用链。
  - 后续业务实现必须遵守 `handler -> service -> repository -> sqlc/database`。

## 9. 并发 / 幂等 / 缓存
- 本次不涉及运行时并发、幂等或缓存。
- 规则层面明确：
  - 并发一致性、幂等和事务边界应写在 `service` 设计中。
  - 缓存、db、redis、clock、idgen、crypto 等跨业务基础设施放 `internal/platform`。
  - `domain` 不依赖 redis、database 或 config。

## 10. 权限与安全
- 本次不实现认证或授权代码。
- 规则层面明确：
  - 认证、安全上下文、密码、JWT、refresh token、user agent 摘要等安全基础能力放 `internal/security`。
  - JWT 只能放稳定身份与技术性 claim，不把角色、邮箱、用户名、用户状态写入 JWT。
  - 权限后的业务规则由 `service` 承载。
- 不涉及敏感信息、审计或操作日志实现。

## 11. 测试策略
- 本次需要运行的验证：
  - `grep -R "handler -> service -> repository" AGENTS.md .agents/skills docs/ai`
  - `grep -R "Go 项目目录结构规范" AGENTS.md`
  - `grep -R "Structure conformance check" .agents/skills/backend-design-first/SKILL.md`
  - `go test ./...`
  - `go vet ./...`
- 单元测试：
  - 不新增运行时代码，无新增单元测试。
- service / repository 测试：
  - 不适用，本次不新增 service 或 repository。
- migration / sqlc 验证：
  - 不适用，本次没有 SQL、schema、sqlc 或 migration 变化。
- 接口验证 / OpenAPI validate：
  - 不适用，本次没有 API 契约变化。
- Java-Go parity 验证：
  - 对照 Java Controller / Service / Mapper / Entity / Config / Security 分层，确认 parity matrix 新增 Go package layout 映射。

## 12. 风险与替代方案
- 当前方案的风险：
  - 规则比当前实际目录更完整，短期存在“结构目标已决策、部分目录尚未落地”的结构债务。
  - 如果未来任务忽略阶段化落地原则，可能为了凑目录创建空 Go package。
  - 如果后续引入 sqlc、OpenAPI 或 migration 时没有同步更新 docs/ai，规则会和实际实现脱节。
- 备选方案：
  - 方案 A：完全横向分层，例如 `internal/handler`、`internal/service`、`internal/repository`。
  - 方案 B：完全纵向 modules，例如 `internal/modules/user/{handler,service,repository}`。
  - 方案 C：当前选择的混合式结构，HTTP、service、repository、domain、安全和平台基础设施各自保持清晰边界。
- 为什么不选备选方案：
  - 不选方案 A：纯横向分层在业务增多后容易出现包过宽、上下文不清，但保留横向核心层有利于 Java-Go parity 学习。
  - 不选方案 B：完全纵向 modules 对早期迁移和 Java-Go 对照不够直观，也容易让基础设施和安全能力被重复包装。
  - 选择方案 C：兼顾 Java-Go parity、Go `internal` 约束、handler/service/repository/sqlc 对照学习和长期业务演进。
- 后续可演进点：
  - 引入 auth/user/event/order 时，按阶段补齐对应 domain、service、repository、handler、dto、security 或 platform 目录。
  - 如果业务复杂度要求局部纵向聚合，可在 ADR 中说明后再调整。
