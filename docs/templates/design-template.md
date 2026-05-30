# 设计文档模板

> 模板说明：本大纲沿用 Java 版 EventHub 的设计文档结构。Go 版文档应保留小节顺序，只在小节内容中替换为 Go、sqlc、migration、OpenAPI 和 Go 测试工具链语境。如确需调整大纲，请在具体设计文档中说明原因。

## 1. 背景
- 当前要解决什么问题
- Java 版对应语义 / 文档 / 代码在哪里
- 业务上下文是什么

## 2. 目标
- 本次要完成什么
- Go 版需要对齐哪些 Java 契约
- 成功标准是什么

## 3. 非目标
- 这次明确不做什么
- 哪些 Java 实现细节不逐行迁移

## 4. 影响范围
- 涉及哪些 Go package / 模块
- 涉及哪些 API / 表 / 缓存 / 外部接口
- 是否影响 `docs/ai/parity/java-go-parity-matrix.md`

## 5. 领域建模
- 核心实体
- 实体关系
- 关键状态
- 与 Java 版领域对象的对应关系

## 6. API 设计
- 接口列表
- 请求参数
- 响应结构
- 错误码 / 异常场景
- 与 Java 版 OpenAPI / controller 契约的差异

## 7. 数据设计
- 表结构调整
- 索引设计
- 唯一约束
- migration 计划
- sqlc query / generated model 影响
- 数据一致性考虑

## 8. 关键流程
- 正常流程
- 异常流程
- 状态流转
- handler / service / repository / sqlc/database 分工

## 9. 并发 / 幂等 / 缓存
- 是否有超卖风险
- 如何防重复提交
- 事务边界在哪里
- 缓存放在哪里，为什么

## 10. 权限与安全
- 哪些角色能访问
- 鉴权与鉴别约束
- JWT claim 边界
- 是否涉及敏感信息、审计或操作日志

## 11. 测试策略
- 单元测试
- service / repository 测试
- migration / sqlc 验证
- 接口验证
- OpenAPI validate
- 异常场景验证
- Java-Go parity 验证
- 需要运行的命令，例如 `gofmt`、`go test ./...`、`go vet ./...`

## 12. 风险与替代方案
- 当前方案的风险
- 备选方案
- 为什么不选备选方案
- 后续可演进点
