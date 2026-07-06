# 管理员用户列表查询参数规范化实现说明

## 1. 本次改动解决了什么问题

本次改动解决了 `parseAdminUserListQuery` 中“校验使用 trim 后的值，但返回给 service 的 `AdminUserListQuery` 仍保留原始空白字符”的边界不一致问题。

改动前，`username`、`email`、`status` 会在 `validateAdminUserListQuery` 中通过 `strings.TrimSpace` 校验，但 query 字段本身仍是原始值。service 层后续会再次 normalize，所以实际数据库筛选有兜底；但 handler 到 service 的读操作输入没有表达“已经完成 HTTP 参数规范化”的语义。

改动后，`parseAdminUserListQuery` 在同一流程中完成 trim、校验和赋值，返回给 service 的 `username`、`email`、`status` 均为去除首尾空白后的值。

## 2. 改动内容
- 新增了什么
  - `internal/http/handler/user/admin_validation_test.go`
    - 新增 `TestParseAdminUserListQueryNormalizesTextFilters`，覆盖 `username`、`email`、`status` 带首尾空白时，最终 service query 使用 trim 后的值。
  - `docs/ai/design/032-admin-user-query-normalization.md`。
  - `docs/ai/implementation/032-admin-user-query-normalization.md`。
- 修改了什么
  - `internal/http/handler/user/admin_validation.go`
    - `parseAdminUserListQuery` 直接对 `username`、`email`、`status` 做 trim 后赋值。
    - 移除独立 `validateAdminUserListQuery`，将分页、长度、状态和时间范围校验合并回 `parseAdminUserListQuery`。
    - 保留 `parseTimeParam` 作为字段级时间解析 helper。
  - `docs/ai/parity/java-go-parity-matrix.md`
    - 在 Auth、当前用户与管理员用户 API 行记录管理员用户列表 query 规范化边界收敛，并索引 design/implementation 032。
- 删除了什么
  - 删除了独立的 `validateAdminUserListQuery` 私有函数。
- 是否更新 Java-Go parity 记录
  - 已更新。原因是本次不改变外部 API 契约，但让 Go strict handler 的 query 映射更明确对齐 Java `AdminUserQueryRequest` / `UserQueryCriteria` 中的 trim / normalize 语义。

## 3. 为什么这样设计
- 关键设计原因
  - query 字段的校验值和赋值值应来自同一份规范化结果，避免后续维护时再次出现分叉。
  - `parseAdminUserListQuery` 本来就是 generated query params 到 service query 的边界方法，在这里完成默认值、trim、校验和映射更直接。
  - `parseTimeParam` 仍保留为 helper，因为它是单字段解析逻辑，复用后不会造成校验值与赋值值不一致。
- 与 Go 项目当前阶段的匹配点
  - 不新增 HTTP DTO，不改变 service Command / Query / Result 文件边界。
  - handler 继续只处理 HTTP 入参、响应映射和错误映射；service 继续承载业务查询规则和防御性 normalize。
  - 不引入共享 validator 抽象，保持当前 strict handler 小范围显式逻辑。
- 与 Java 版业务语义的对齐方式
  - Java 管理员用户分页设计要求 `username` / `email` trim 后为空则忽略，`email` 最终按小写化条件查询。
  - Go handler 先 trim 进入 service query，service 再保留 `normalizeFilter` / `normalizeEmailFilter` 作为最终 repository criteria 的兜底与小写化边界。

## 4. 替代方案
- 方案 A：保持 `parseAdminUserListQuery` 和 `validateAdminUserListQuery` 分离，只在 parse 阶段 trim 后赋值。
  - 没有采用。虽然可以修复当前 bug，但仍会保留同一组 query 字段在两个函数中来回传递的分叉风险。
- 方案 B：只依赖 service 层 normalize，不改 handler。
  - 没有采用。service 的兜底能保证查询行为，但 handler 输出的 service query 仍不是规范化后的边界对象。
- 方案 C：抽取通用字符串筛选 normalizer。
  - 没有采用。当前只有管理员用户列表有这个局部需求，抽通用能力会扩大改动范围。
- 方案 D：把 email 小写化也提前到 handler。
  - 没有采用。当前 service 的 `normalizeEmailFilter` 已经承担 repository criteria 的小写化，handler 只负责 HTTP 空白规范化即可，避免把数据库查询条件细节前移。

## 5. 测试与验证
- 跑了哪些测试
  - RED：`go test ./internal/http/handler/user`
    - 失败原因符合预期：`username = "  alice  ", want "alice"`。
  - GREEN：`go test ./internal/http/handler/user`
    - 通过。
- 跑了哪些质量门禁，例如 `gofmt`、`go test ./...`、`go vet ./...`、`sqlc generate`
  - `gofmt -w internal/http/handler/user/admin_validation.go internal/http/handler/user/admin_validation_test.go`：已执行。
  - `gofmt -l internal/http/handler/user/admin_validation.go internal/http/handler/user/admin_validation_test.go`：无输出。
  - `go test ./internal/http/handler/user`：通过。
  - `go test ./...`：通过。
  - `go vet ./...`：通过。
  - `golangci-lint run`：通过，输出 `0 issues.`。
  - `git diff --check -- internal/http/handler/user/admin_validation.go internal/http/handler/user/admin_validation_test.go docs/ai/design/032-admin-user-query-normalization.md docs/ai/implementation/032-admin-user-query-normalization.md docs/ai/parity/java-go-parity-matrix.md`：通过。
  - `sqlc generate`：未运行，本次不涉及 SQL、schema 或 sqlc 配置。
  - OpenAPI validate / generate：未运行，本次不涉及 OpenAPI 契约或 generated code。
- 手工验证了哪些场景
  - 对比 Java `AdminUserQueryRequest` 与 Java 管理员用户分页设计，确认 `username` / `email` / `status` 的 trim 语义保持一致。
  - 检查 Go service 仍保留 normalize 兜底，handler 改动不会让 service 直接依赖 HTTP generated model。
- Java-Go parity 如何验证
  - 更新 `docs/ai/parity/java-go-parity-matrix.md` 的 Auth、当前用户与管理员用户 API 行。
  - 确认 `GET /api/v1/admin/users` 的路径、请求字段、响应字段、错误码和分页语义不变。
- 结果如何
  - 定向 RED/GREEN、全量 Go 测试、vet、lint、格式化和 whitespace 检查均通过。

## 6. 已知限制
- service 层仍保留 trim / email lower-case normalize，因此 handler 和 service 对空白处理存在防御性重复。
- 本次不改变邮箱小写化位置；如果未来希望 service query 本身也表达小写化后的 email，需要单独评估 service contract 语义。
- 本次不补 HTTP 黑盒集成测试，因为目标边界是私有 parse 方法输出的 service query，包内单元测试覆盖更直接。

## 7. 对后续版本的影响
- 对简历可用版的价值
  - 管理员用户查询边界更清晰，strict handler 可读性更贴近 generated params 到 service query 的映射职责。
- 对微服务 / 云原生演进的影响
  - 无直接影响；但保留了 service 防御性 normalize，后续如果存在非 HTTP 调用源仍能得到一致查询条件。
- 对后续 Go package、migration、sqlc、OpenAPI 或测试策略的影响
  - 后续新增列表查询时，优先让 parse 方法直接产出规范化后的 service Query。
  - 不影响 migration、sqlc 或 OpenAPI 生成策略。
