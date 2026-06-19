// Package config 负责从环境变量加载服务运行配置。
package config

import (
	"os"
	"strconv"
	"time"
)

const (
	defaultXrayAPIAddr   = "67.209.178.119:80"
	defaultWebListen     = "127.0.0.1:8090"
	defaultHourlyDataFil = "./data/hourly.json"
	defaultSampleSeconds = 60
	defaultRetentionDays = 30
)

// Config 保存服务运行所需的配置项。
type Config struct {
	// XrayAPIAddr 是 Xray gRPC API 服务端地址。
	XrayAPIAddr string
	// ListenAddr 是本服务 HTTP 监听地址。
	ListenAddr string
	// AdminToken 是写操作（增删用户等）所需的鉴权 token，为空时写操作一律拒绝。
	AdminToken string
	// HourlyDataFile 是每小时流量持久化 JSON 文件路径。
	HourlyDataFile string
	// SampleInterval 是每小时流量后台采样间隔。
	SampleInterval time.Duration
	// Retention 是每小时流量历史保留时长，超期桶会被剪枝。
	Retention time.Duration
}

// Load 从环境变量读取配置，缺省时回退到默认值。
func Load() Config {
	return Config{
		XrayAPIAddr:    envOr("XRAY_API_ADDR", defaultXrayAPIAddr),
		ListenAddr:     envOr("WEB_LISTEN", defaultWebListen),
		AdminToken:     os.Getenv("ADMIN_TOKEN"),
		HourlyDataFile: envOr("HOURLY_DATA_FILE", defaultHourlyDataFil),
		SampleInterval: time.Duration(envInt("SAMPLE_INTERVAL", defaultSampleSeconds)) * time.Second,
		Retention:      time.Duration(envInt("RETENTION_DAYS", defaultRetentionDays)) * 24 * time.Hour,
	}
}

// envOr 返回环境变量 key 的值，为空时返回 fallback。
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt 返回环境变量 key 解析后的正整数，缺失或非法（含非正数）时返回 fallback。
func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
