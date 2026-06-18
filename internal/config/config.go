// Package config 负责从环境变量加载服务运行配置。
package config

import "os"

const (
	defaultXrayAPIAddr = "67.209.178.119:80"
	defaultWebListen   = ":8090"
)

// Config 保存服务运行所需的配置项。
type Config struct {
	// XrayAPIAddr 是 Xray gRPC API 服务端地址。
	XrayAPIAddr string
	// ListenAddr 是本服务 HTTP 监听地址。
	ListenAddr string
}

// Load 从环境变量读取配置，缺省时回退到默认值。
func Load() Config {
	return Config{
		XrayAPIAddr: envOr("XRAY_API_ADDR", defaultXrayAPIAddr),
		ListenAddr:  envOr("WEB_LISTEN", defaultWebListen),
	}
}

// envOr 返回环境变量 key 的值，为空时返回 fallback。
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
