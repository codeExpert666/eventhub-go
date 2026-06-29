# ADR：Docker 运行镜像不携带 Go 编译工具链

## 标题
Go 应用使用多阶段 Dockerfile，并以 Alpine 作为不含 Go 工具链的运行镜像

## 状态
- accepted

## 背景
Java 版 `backend/Dockerfile` 已采用多阶段构建：构建阶段使用 Maven/JDK，最终运行阶段只保留 JRE 和应用 Jar，不携带 Maven、源码和编译缓存。

Go 版同样需要容器化运行，但不能简单把 `golang` builder 镜像作为最终镜像。活动预约与票务平台后续会进入更完整的本地演示、CI 和部署阶段，运行镜像应尽早收敛到更小攻击面、更少构建期依赖的形态。

本次还要求 Compose healthcheck 直接调用 `/actuator/health`。如果最终镜像完全没有 shell 或 HTTP 探针工具，Compose 需要额外依赖平台侧探针或应用内 probe 子命令；当前阶段还没有这类能力。

## 决策
Go 版 `Dockerfile` 使用多阶段构建：

- build stage 使用固定 Go 官方镜像 `golang:1.24.0-alpine3.21`。
- build stage 编译 `./cmd/eventhub`，输出静态二进制。
- runtime stage 使用固定 Alpine 镜像 `alpine:3.21`。
- runtime stage 只安装 `ca-certificates` 和 `tzdata`，并使用 Alpine 自带的轻量 `wget` 作为 healthcheck 工具。
- runtime stage 不包含 Go 编译器、源码、module cache、测试工具或 sqlc/openapi/migrate 生成工具。
- runtime 容器默认 `EVENTHUB_ENV=prod`、`OPENAPI_ENABLED=false` 且 `OPENAPI_ASSET_ROOT=/app/api/openapi`。
- runtime 容器使用非 root 用户运行应用。

## 备选方案
- 方案 1：最终镜像继续使用 `golang`。
- 方案 2：最终镜像使用 `scratch`。
- 方案 3：最终镜像使用 distroless static。
- 方案 4：最终镜像使用 Alpine。

## 决策理由
选择方案 4：

- 与 Java 多阶段 Dockerfile 的核心目标一致：构建工具只留在 build stage，运行镜像只承载运行职责。
- Alpine 不含 Go 工具链，满足最终运行镜像不带编译工具链的要求。
- Alpine 仍提供 shell / wget，能低成本实现 `GET /actuator/health` 容器 healthcheck。
- `ca-certificates` 和 `tzdata` 适合后续需要 TLS 客户端访问或稳定时区行为的后端进程。
- 非 root 用户运行符合容器默认最小权限原则。

没有选择其他方案：

- 不选 `golang`：会把 Go 编译器、源码构建语境和更多攻击面带到 runtime。
- 不选 `scratch`：镜像最小，但缺少 CA、时区和 healthcheck 工具，当前阶段排障和探针成本偏高。
- 不选 distroless：安全边界好，但缺少 shell/wget；当前还没有应用内 probe 子命令或平台侧探针配置来替代容器内 healthcheck。

## 影响
- 好处
  - 镜像职责更清晰，构建期和运行期依赖分离。
  - 运行镜像不携带 Go 编译工具链。
  - Compose healthcheck 可以直接使用现有 `/actuator/health`。
  - prod 默认不暴露 Swagger/OpenAPI；显式开启时静态资源目录也有确定的容器内路径。
- 代价
  - Alpine 比 `scratch` / distroless 多一些运行时组件。
  - 需要维护 Alpine 与 Go builder 镜像版本。
  - healthcheck 依赖容器内 `wget`，未来若切换 distroless 需要替代方案。
- 后续可能需要调整的地方
  - CI 可补镜像扫描、SBOM 和签名。
  - 如果生产平台提供 HTTP probe，可评估切换到 distroless。
  - 如果应用增加 `eventhub healthcheck` 子命令，也可移除 runtime 镜像内 HTTP 工具。
