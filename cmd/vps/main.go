// Command vps 连接 Xray gRPC StatsService 并将其能力暴露为 HTTP API。
package main

import (
	"context"
	"log"
	"time"

	"vps/internal/config"
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

	client, conn, err := xrayclient.Dial(ctx, cfg.XrayAPIAddr)
	if err != nil {
		log.Fatalf("无法连接到 Xray API 服务端 %s: %v", cfg.XrayAPIAddr, err)
	}
	defer conn.Close()
	log.Printf("成功连接到 Xray API 服务端 %s", cfg.XrayAPIAddr)

	// 复用同一条连接，整个进程生命周期内共享 StatsService 客户端。
	svc := stats.NewService(client)
	if err := server.New(svc).Run(cfg.ListenAddr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}
