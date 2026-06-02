# Persistence Foundation 实现说明

## 1. 本次改动解决了什么问题

本次为 Go 版 EventHub 建立了 MySQL、Redis、migration、sqlc 和 repository 底座，使后续 auth 注册、登录、refresh token、RBAC 和管理员用户列表可以在稳定的持久化边界上继续迁移。

本次对齐 Java 版 Flyway V1-V3 与 MyBatis `UserMapper`、`RoleMapper`、`AuthSessionMapper` 的数据库语义，但没有逐行迁移 Java/Spring 结构。Go 版采用 golang-migrate、sqlc、`database/sql`、repository interface 和 Testcontainers MySQL。

后续 review 中补充修复了两个持久化契约细节：MySQL DSN 在连接层强制 `parseTime=true`，以及单用户角色编码查询在无结果时返回空 slice，对齐 Java/MyBatis 空 List 语义。

## 2. 改动内容
- 新增了什么
  - `migrations/000001_system_bootstrap.*.sql`：对齐 Java V1 的 `system_bootstrap_record`。
  - `migrations/000002_auth_schema.*.sql`：对齐 Java V2/V3 的 `users`、`roles`、`user_roles`、`auth_sessions`，并 seed `USER`、`ADMIN` 和本地 demo admin。
  - `sqlc.yaml`：配置 MySQL engine、schema 文件、query 目录和 `internal/repository/mysql/sqlc` 输出目录。
  - `internal/platform/db`：MySQL 连接、唯一约束识别和 service 可用的事务控制器。
  - `internal/platform/redis`：Redis client、默认地址和 ping 底座；当前不参与认证强一致。
  - `internal/repository`：user、role、auth session repository interface 与持久化语义结构体。
  - `internal/repository/mysql/queries/*.sql`：对齐 Java MyBatis XML 的显式 SQL。
  - `internal/repository/mysql/sqlc/*`：由 sqlc 生成的 MySQL 查询代码。
  - `internal/repository/mysql`：包装 sqlc generated code，不向外暴露 generated model。
  - `internal/repository/mysql/mysql_repository_integration_test.go`：Testcontainers MySQL 集成测试，覆盖 migration up/down、seed、repository、唯一约束、事务上下文和 session 条件更新。
  - `internal/platform/db/errors_test.go`：MySQL 1062 duplicate entry 识别单元测试。
  - `Makefile` 增加 `make sqlc`。
  - ADR-0007 到 ADR-0010。
- 修改了什么
  - `internal/platform/db.OpenMySQL` 现在会解析并规范化 MySQL DSN，强制 `parseTime=true`，避免 sqlc 生成代码扫描 `TIMESTAMP` 到 `time.Time` 时依赖调用方手动配置。
  - `internal/repository/mysql.RoleRepository.FindRoleCodesByUserID` 在无角色记录时返回 `[]string{}`，不再透出 sqlc 的 nil slice。
  - `internal/platform/db/mysql_test.go` 增加 DSN 规范化单元测试。
  - `internal/repository/mysql/mysql_repository_integration_test.go` 增加无角色查询空 slice 断言。
  - `go.mod` / `go.sum` 新增 MySQL driver、golang-migrate、Redis client、Testcontainers 相关依赖。
  - `go` directive 由 `1.24` 规范化为 `1.24.0`，仍保持 Go 1.24 线；这是 Go module tooling 与 Testcontainers `go 1.24.0` 声明对齐后的结果。
- 删除了什么
  - 本次没有删除运行时代码。
- 是否更新 Java-Go parity 记录
  - 已更新 `docs/ai/parity/java-go-parity-matrix.md`，记录数据库迁移与持久化边界、数据库测试策略、auth session 持久化底座和 Redis 认证一致性边界。
- 文件移动和 package 边界变化
  - 本次没有移动既有文件。
  - 新增 `internal/repository`、`internal/repository/mysql`、`internal/repository/mysql/queries`、`internal/repository/mysql/sqlc`、`internal/platform/db`、`internal/platform/redis`。
  - 没有新增 HTTP handler / DTO package，也没有新增 service Command / Query / Result。
  - DTO、service contract、domain model 映射不适用；repository model 当前只表达持久化语义，后续 auth service 再映射到 service result 或 domain model。

## 3. 为什么这样设计
- 关键设计原因
  - Java 版 Flyway + MyBatis XML 的核心价值是显式 schema 和显式 SQL。Go 版用 golang-migrate + sqlc 保留这个可审查性。
  - sqlc 生成查询代码，repository/mysql 再包装一层，可以避免 sqlc row 进入 handler/service，同时保留编译期 SQL 参数和结果类型检查。
  - `database/sql` 是 Go 标准库数据库抽象，配合 `go-sql-driver/mysql` 足够覆盖当前 MySQL 需求。
  - MySQL unique constraint 检测放在 `platform/db`，只识别底层数据库错误，不在基础设施层绑定 `AUTH-409`，后续由 service 做业务错误映射。
  - `Transactor` 用 context 传递 `*sql.Tx`，repository/mysql 自动选择当前 tx 或普通 `*sql.DB`。事务边界留给 service 控制，符合 `handler -> service -> repository -> sqlc/database`。
  - Redis 只作为平台连接底座，不作为 auth session 权威来源，避免 refresh/logout/user disabled 这类安全状态出现双写一致性问题。
- 与 Go 项目当前阶段的匹配点
  - 当前还没有 auth service 和 handler，因此不创建空 `internal/service/auth`、`internal/http/handler/auth` 或 DTO 包。
  - repository 已足够支撑后续 auth 最小闭环：创建用户、绑定角色、按用户名/邮箱查用户、角色查询、创建/查询/轮换/吊销 auth session。
- 与 Java 版业务语义的对齐方式
  - 表名、字段名、唯一约束、外键、索引、seed 角色、用户状态 `ENABLED/DISABLED`、会话状态 `ACTIVE/REVOKED` 与 Java 版保持一致。
  - Java V2/V3 在 Go 版合并为 `000002_auth_schema`，因为 Go 版当前没有生产历史版本需要逐号保留；来源和差异已在设计与 parity matrix 说明。
  - Java H2 测试策略在 Go 版刻意改为 Testcontainers MySQL，以真实验证 MySQL migration、约束和条件更新。
  - Java/MyBatis 查询多行无匹配时返回空 List；Go repository 包装层统一把 sqlc nil slice 转为空 slice，避免把生成代码细节泄漏给 service/API。
  - MySQL 时间字段是 schema 契约的一部分，连接层强制 `parseTime=true` 比要求每个调用方记住 DSN 参数更稳。

## 4. 替代方案
- 方案 A：GORM
  - 没有采用。GORM 会把 ORM model、迁移和查询语义包得更隐式，不适合当前阶段逐项对齐 Java MyBatis XML 的 SQL 契约。
- 方案 B：sqlx / 手写 `database/sql`
  - 没有采用。手写 row scan 会重复样板，也缺少 sqlc 提供的编译期查询参数和结果结构生成。
- 方案 C：继续使用 Flyway
  - 没有采用。Flyway 是 Java 生态工具，Go 项目测试和部署继续引入 JVM 迁移器会扩大工具链边界。
- 方案 D：goose
  - 没有采用。goose 同样可行，但本次选择 golang-migrate 是因为其 source/database driver 分离、Testcontainers 集成和纯 SQL migration 方式更直接。
- 方案 E：H2 / SQLite 测试
  - 没有采用。本次要验证 MySQL 真实唯一约束、外键、时间戳、`LAST_INSERT_ID()` 和条件 update，替代数据库容易产生假阳性。

## 5. 测试与验证
- 跑了哪些测试
  - `make sqlc`
  - `go test ./internal/repository/mysql`
  - `go test ./...`
  - `go vet ./...`
  - `make test`
  - `make vet`
  - `git diff --check`
  - 修复 review 点后追加运行：`gofmt`、`make sqlc`、`go test -count=1 ./internal/platform/db`、`go test -count=1 ./internal/repository/mysql`、`go test ./...`、`go vet ./...`。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - 已对新增/修改 Go 文件执行 `gofmt`。
  - `make sqlc` 已成功执行，等价运行 `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate`。
  - `go test ./...` 通过，其中 `internal/repository/mysql` 非缓存运行时启动 Testcontainers MySQL 并执行 migration/repository 集成测试。
  - `go vet ./...` 通过。
  - `make test`、`make vet` 通过。
  - `golangci-lint run` 未运行：仓库有 `.golangci.yml`，但当前机器未安装 `golangci-lint`。
- 手工验证了哪些场景
  - migration up 后五张表存在。
  - migration down 后 `users` 等业务表不存在。
  - `USER` / `ADMIN` seed 角色存在，demo admin 拥有两个角色。
  - repository transaction context 能把创建用户和绑定角色放入同一事务。
  - MySQL DSN 未显式配置或显式配置 `parseTime=false` 时，`platform/db` 规范化结果仍启用 `parseTime=true`。
  - 不存在用户的角色编码查询返回非 nil 的空 slice。
  - username duplicate 触发 MySQL 1062，并能被 `platform/db.IsUniqueConstraintError` 捕获。
  - auth session 可创建、按 hash 查询、重复 session_id 被唯一约束拒绝。
  - refresh token 条件轮换第一次成功、旧 token/version 第二次失败。
  - ACTIVE session 吊销为 REVOKED，第二次吊销 affected rows 为 0。
- Java-Go parity 如何验证
  - 对照 Java Flyway V1-V3。
  - 对照 Java `UserMapper.xml`、`RoleMapper.xml`、`AuthSessionMapper.xml`。
  - 对照 Java `AuthSessionMapperTest` 和 `AuthSessionConcurrencyTest` 的关键测试意图。
- 结果如何
  - 已通过，除 `golangci-lint` 因本机缺工具未运行外，其余可行验证均通过。

## 6. 已知限制
- 当前版本还缺什么
  - 未实现 auth handler、auth DTO、auth service、JWT、密码哈希、refresh token 生成/解析。
  - `internal/platform/db` 和 `internal/platform/redis` 尚未接入 `internal/config` 和 `internal/app` 启动装配。
  - `OpenMySQL` 当前接受 `go-sql-driver/mysql` DSN 格式；如果后续配置层想支持 URL 风格 DSN，需要在 config 层转换后再传入。
  - Redis 只有连接底座，未实现缓存、denylist、限流或健康检查装配。
  - repository 当前使用持久化语义结构体；后续 auth service 引入后，需要明确 domain/service result 映射。
- 哪些地方后面需要继续演进
  - auth 注册要用 service 事务包住 user insert + role binding，并把 unique constraint 映射到 `AUTH-409`。
  - login/refresh/logout 需要补 refresh token hash、session id、JWT claim 和 user disabled 校验。
  - 活动、订单、库存迁移时要继续使用 Testcontainers MySQL 做并发和事务测试。
- 与 Java 版仍有哪些差距
  - Java 已有 auth HTTP API、security filter、token service、auth integration tests；Go 本次只完成持久化底座。
  - Java test profile 使用 H2；Go 已决策改用 MySQL 容器，属于刻意差异。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 项目从纯 HTTP foundation 进入真实数据库工程阶段，具备 migration、SQL 生成、repository、事务和集成测试能力。
- 对微服务 / 云原生演进的影响
  - golang-migrate 和 Testcontainers 适合 CI、容器化和服务拆分后的独立迁移验证。
  - Redis 暂不承担认证强一致，降低后续拆分 auth/session 服务时的一致性风险。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 新增数据库访问必须继续遵守 `repository interface -> repository/mysql -> sqlc`。
  - 新增 SQL query 后运行 `make sqlc`。
  - 新增 migration 后补 Testcontainers migration up/down 或 repository 集成测试。
  - OpenAPI 暂未受影响，后续 auth API 迁移时再补契约验证。
