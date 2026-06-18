// Package xrayclient 封装与 Xray gRPC API 的连接建立。
package xrayclient

import (
	"context"
	"fmt"
	"io"

	"github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial 建立到 Xray gRPC API 的连接，返回 StatsService 客户端与底层连接的 Closer。
// 本地 API 通常无需 TLS，使用 insecure 凭证；WithBlock 阻塞直到连接成功或 ctx 超时。
// 调用方负责在使用完毕后 Close 返回的 io.Closer。
func Dial(ctx context.Context, addr string) (command.StatsServiceClient, io.Closer, error) {
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("连接 Xray API 服务端 %s 失败: %w", addr, err)
	}
	return command.NewStatsServiceClient(conn), conn, nil
}
