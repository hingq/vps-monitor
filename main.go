package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultXrayAPIAddr = "67.209.178.119:80"
	defaultWebListen   = ":8090"
)

// formatBytes 将字节数格式化为可读单位（B/KB/MB/GB/TB）。
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGT"[exp])
}

// envOr 返回环境变量 key 的值，为空时返回 fallback。
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	xrayAPIAddr := envOr("XRAY_API_ADDR", defaultXrayAPIAddr)
	listenAddr := envOr("WEB_LISTEN", defaultWebListen)

	// 建立与 Xray gRPC API 的连接（本地 API 通常无需 TLS）。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, xrayAPIAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), // 阻塞直到连接成功或超时
	)
	if err != nil {
		log.Fatalf("无法连接到 Xray API 服务端 %s: %v", xrayAPIAddr, err)
	}
	defer conn.Close()

	log.Printf("成功连接到 Xray API 服务端 %s", xrayAPIAddr)

	// 复用同一条连接，整个进程生命周期内共享 StatsService 客户端。
	client := command.NewStatsServiceClient(conn)

	if err := startWebServer(listenAddr, client); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}
