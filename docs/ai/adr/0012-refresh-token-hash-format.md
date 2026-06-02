# ADR：refresh token hash 格式

## 标题
refresh token 以 `sha256:<hex>` 保存哈希

## 状态
- accepted

## 背景
登录成功需要返回 opaque refresh token，并创建服务端 `ACTIVE` auth session。refresh token 是长期凭证，后续会用于换取新的 access token 和 refresh token。如果数据库保存明文 refresh token，一旦数据库泄漏，攻击者可以直接复用长期凭证。

Java 版 `RefreshTokenHasher` 当前使用 SHA-256，并给落库值添加 `sha256:` 前缀。Go 版需要明确是否沿用该格式，还是改用 HMAC、BCrypt、Argon2 或其他方案。

## 决策
Go 版选择：

- refresh token 明文由 32 字节密码学安全随机数组成。
- 明文使用 Base64 URL-safe 无 padding 编码，长度固定 43。
- 明文只在登录响应中返回一次。
- DB 只保存 `sha256:<hex>`。
- `hex` 为 SHA-256 digest 的小写十六进制。
- refresh endpoint 下一阶段实现时，使用客户端提交的明文重新计算 hash 后查询 `auth_sessions.refresh_token_hash`。

## 备选方案
- 方案 1：DB 保存 refresh token 明文。
- 方案 2：DB 保存 `sha256:<hex>`。
- 方案 3：DB 保存 HMAC-SHA256 或带 pepper 的 hash。
- 方案 4：使用 BCrypt/Argon2 保存 refresh token hash。

## 决策理由
选择方案 2，原因是：

- 与 Java 当前实现和数据库字段长度保持一致。
- refresh token 本身由高熵随机数生成，不是用户可记忆密码；SHA-256 对高熵随机 token 的离线猜测风险可接受。
- `sha256:` 前缀为未来升级到 HMAC 或带 pepper 方案保留数据格式边界。
- BCrypt/Argon2 更适合低熵密码，对高熵随机 token 会增加不必要的计算成本和查询复杂度。
- 保存明文不可接受，会扩大数据库泄漏后的凭证复用风险。

## 影响
- 好处
  - 数据库不保存可直接使用的长期凭证明文。
  - 格式与 Java 版完全对齐，便于迁移和测试。
  - 前缀化格式支持后续平滑升级 hash 算法。
- 代价
  - 如果 refresh token 生成器出现熵不足，普通 SHA-256 无法像带 pepper 的 HMAC 那样额外保护。
  - 需要保证 refresh token 明文只返回一次，日志和错误信息都不能输出它。
- 后续可能需要调整的地方
  - 生产安全要求提高时，可升级为 `hmac-sha256:<hex>` 或引入 pepper。
  - 实现 refresh token 轮换时，需要结合 old hash、version、status 和过期时间做条件更新。
  - 需要周期性清理过期 auth sessions。
