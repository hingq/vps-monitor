// Package xrayclient 封装与 Xray gRPC API 的连接建立。
package xrayclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial 建立到 Xray gRPC API 的连接并返回底层 *grpc.ClientConn。
// 本地 API 通常无需 TLS，使用 insecure 凭证；WithBlock 阻塞直到连接成功或 ctx 超时。
// 调用方在该连接上自行创建所需的各 Service 客户端（StatsService / HandlerService /
// RoutingService 等），并负责在使用完毕后 Close 连接。
func Dial(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("连接 Xray API 服务端 %s 失败: %w", addr, err)
	}
	return conn, nil
}
