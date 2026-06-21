# syntax=docker/dockerfile:1

# ---- build 阶段：静态编译 Go 二进制 ----
# --platform=$BUILDPLATFORM：编译器跑在宿主原生架构上（避免 QEMU 模拟编译，极大提速），
# 再靠下方 GOARCH 交叉编译产出目标架构二进制。
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

# 由 buildkit 注入的目标平台，决定交叉编译产物架构。
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# 先拉依赖，利用层缓存（go.mod/go.sum 不变时跳过重新下载）。
COPY go.mod go.sum ./
RUN go mod download

# 再编译。xray-core 为纯 Go，关闭 CGO 产出静态可执行文件；-o /vps 固定产物路径，与运行阶段 COPY 对齐。
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /vps ./cmd/vps

# ---- 运行阶段：最小镜像 ----
FROM alpine:latest

# gRPC 走公网若启用 TLS 需根证书；当前用 insecure，加上无害且更稳妥。
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 appuser

COPY --from=build /vps /vps

# 每小时流量持久化目录，预建并归属 appuser，命名卷首次挂载会继承该属主。
RUN mkdir -p /data && chown appuser:appuser /data
VOLUME ["/data"]

USER appuser
EXPOSE 8080
ENTRYPOINT ["/vps"]
