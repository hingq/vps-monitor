# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

HTTP → Xray gRPC 桥接服务。连接 Xray 的多个 gRPC service（StatsService 统计、HandlerService inbound 用户管理、RoutingService 路由），包装成 JSON REST 接口对外暴露。模块名为 `vps`（见 `go.mod`，Go 1.26）。

## 架构

入口 `cmd/vps/main.go` 拨号一次得到 `*grpc.ClientConn`，在其上创建各 service 客户端并注入，整个进程复用同一条连接：

- `internal/config` — 从环境变量加载配置，缺省回退默认值
- `internal/xrayclient` — 用 `insecure` 凭证 + `WithBlock` 拨号 Xray gRPC，返回 `*grpc.ClientConn`（本地 API 通常无 TLS）
- `internal/stats` — StatsService 业务层：6 个 gRPC 方法 1:1 封装 + 流量/在线聚合
- `internal/proxyman` — HandlerService 业务层：VLESS 增删用户（走 `AlterInbound` + typed `Add/RemoveUserOperation`）、列举/计数用户
- `internal/routing` — RoutingService 业务层：只读 `ListRule`
- `internal/hourly` — 每小时流量：后台 `Collector` 定时采样累计计数器，`Store` 差分累加进整点小时桶 + JSON 文件持久化 + 30 天剪枝；对外 `/api/traffic/hourly`
- `internal/server` — HTTP 路由与 handler，每个请求 5s context 超时；写操作经 `requireToken` 中间件鉴权
- `internal/humanize` — 字节数格式化（B/KB/MB/GB/TB）

接口详见 `@docs/api.md`。

## 命令

- 运行：`go run ./cmd/vps`
- 构建：`go build -o vps ./cmd/vps`
- 全部测试：`go test ./...`
- 单包测试：`go test ./internal/stats`
- 单个测试：`go test ./internal/stats -run TestName`
- Lint：`golangci-lint run ./...`（配置见 `.golangci.yml`，需 `brew install golangci-lint`）

## 配置（环境变量）

- `XRAY_API_ADDR` — Xray gRPC API 地址，默认 `67.209.178.119:80`
- `WEB_LISTEN` — HTTP 监听地址，默认 `:8090`
- `ADMIN_TOKEN` — 写操作（增删用户）的鉴权 token；**为空时写操作一律返回 403**
- `HOURLY_DATA_FILE` — 每小时流量持久化 JSON 文件路径，默认 `./data/hourly.json`
- `SAMPLE_INTERVAL` — 每小时流量后台采样间隔（秒），默认 `60`
- `RETENTION_DAYS` — 每小时流量历史保留天数，默认 `30`

## 关键约定与 gotcha

- **Xray 计数器命名**：聚合逻辑假设计数器名为 `category>>>name>>>traffic>>>direction` 格式，不符合此格式的计数器会被静默丢弃。改动 `/api/traffic` 等聚合时务必遵守。
- **每小时流量基于差分采样**：Xray 计数器是累计值，`hourly` 包靠后台定时采样做差分（`delta = max(0, 当前-上次)`，回退判定为重启取新值）累加进整点桶。服务重启后首采样仅重建基线（`lastValues` 不持久化），重启到首采样间的流量丢失，属预期。小时边界按服务器本地时区对齐。
- **部分失败容忍**：`/api/online` 聚合中单个用户的 IP 查询失败不会中断整个响应。
- **service 依赖最小接口**：`proxyman`/`routing` 的 `Service` 只依赖本包内声明的最小接口（`handlerClient`/`routingClient`，仅含用到的方法），而非完整的 `command.XxxServiceClient`。新增方法时同步扩这个接口，测试 fake 也只需实现这几个方法。
- **测试**：标准 `testing` 包，表驱动 + 实现 gRPC 接口的 fake（如 `fakeClient`）注入，不依赖真实 Xray。
- 代码注释与日志用中文；JSON 字段用 snake_case（如 `uptime_seconds`、`value_human`）。

## 提交约定

Conventional Commits + 中文描述（如 `feat: 添加在线用户聚合接口`），直接提交到 `main`。
