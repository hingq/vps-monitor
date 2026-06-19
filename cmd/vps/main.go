// Command vps 连接 Xray gRPC StatsService 并将其能力暴露为 HTTP API。
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	proxymancmd "github.com/xtls/xray-core/app/proxyman/command"
	routercmd "github.com/xtls/xray-core/app/router/command"
	statscmd "github.com/xtls/xray-core/app/stats/command"

	"vps/internal/config"
	"vps/internal/hourly"
	"vps/internal/proxyman"
	"vps/internal/routing"
	"vps/internal/server"
	"vps/internal/stats"
	"vps/internal/xrayclient"
)

// dialTimeout 建立 gRPC 连接的超时时间。
const dialTimeout = 5 * time.Second

func main() {
	cfg := config.Load()

	// 建立与 Xray gRPC API 的连接（本地 API 通常无需 TLS）。
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	conn, err := xrayclient.Dial(ctx, cfg.XrayAPIAddr)
	if err != nil {
		log.Fatalf("无法连接到 Xray API 服务端 %s: %v", cfg.XrayAPIAddr, err)
	}
	defer conn.Close()
	log.Printf("成功连接到 Xray API 服务端 %s", cfg.XrayAPIAddr)

	// 复用同一条连接，整个进程生命周期内共享各 Service 客户端。
	statsSvc := stats.NewService(statscmd.NewStatsServiceClient(conn))
	pmSvc := proxyman.NewService(proxymancmd.NewHandlerServiceClient(conn))
	routingSvc := routing.NewService(routercmd.NewRoutingServiceClient(conn))

	// 每小时流量：加载历史，启动后台采样器（绑定信号 context 以便优雅退出落盘）。
	hourlyStore := hourly.NewStore(cfg.HourlyDataFile, cfg.Retention)
	if err := hourlyStore.Load(); err != nil {
		log.Printf("加载每小时流量历史失败（将以空数据启动）: %v", err)
	}
	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go hourly.NewCollector(statsSvc, hourlyStore, cfg.SampleInterval).Run(runCtx)

	if err := server.New(statsSvc, pmSvc, routingSvc, hourlyStore, cfg.AdminToken).Run(cfg.ListenAddr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}
