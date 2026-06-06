# syntax=docker/dockerfile:1.7

FROM golang:1.24.0-alpine3.21 AS build

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/eventhub ./cmd/eventhub

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S eventhub \
	&& adduser -S -G eventhub eventhub

WORKDIR /app

COPY --from=build /out/eventhub /app/eventhub

ENV EVENTHUB_ENV=prod \
	EVENTHUB_HTTP_PORT=8080 \
	OPENAPI_ENABLED=false

EXPOSE 8080

USER eventhub:eventhub

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
	CMD wget -qO- http://127.0.0.1:8080/actuator/health >/dev/null || exit 1

ENTRYPOINT ["/app/eventhub"]
