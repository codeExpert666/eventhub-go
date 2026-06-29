# syntax=docker/dockerfile:1.7

# 构建阶段：将 Go 服务编译为体积较小的 Linux 静态二进制文件。
FROM golang:1.24.0-alpine3.21 AS build

WORKDIR /workspace

# 先复制 go.mod / go.sum 并下载依赖，便于 Docker 在仅业务代码变化时复用依赖缓存层。
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/eventhub ./cmd/eventhub

# 运行阶段：最终镜像只保留运行所需内容，不携带 Go 编译工具链。
FROM alpine:3.21

# 安装 CA 根证书和时区数据，并创建非特权用户运行服务进程。
RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S eventhub \
	&& adduser -S -G eventhub eventhub

WORKDIR /app

# 仅从构建阶段复制已编译好的服务二进制文件和可选 OpenAPI 本地静态资源。
COPY --from=build /out/eventhub /app/eventhub
COPY --from=build /workspace/api/openapi/eventhub.yaml /app/api/openapi/eventhub.yaml
COPY --from=build /workspace/api/openapi/swagger /app/api/openapi/swagger

# 默认运行配置；部署环境可通过容器环境变量覆盖这些值。
ENV EVENTHUB_ENV=prod \
	EVENTHUB_HTTP_PORT=8080 \
	OPENAPI_ENABLED=false \
	OPENAPI_ASSET_ROOT=/app/api/openapi

EXPOSE 8080

USER eventhub:eventhub

# 复用 Go 服务暴露的 Spring 兼容健康检查路径。
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
	CMD wget -qO- http://127.0.0.1:8080/actuator/health >/dev/null || exit 1

ENTRYPOINT ["/app/eventhub"]
