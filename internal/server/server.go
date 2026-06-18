// Package server 实现 HTTP 层：路由注册与请求处理。
package server

import (
	"log"
	"net/http"
	"time"

	"vps/internal/stats"
)

// requestTimeout 单个 HTTP 请求查询 Xray 的超时时间。
const requestTimeout = 5 * time.Second

// Server 持有业务层依赖并对外提供 HTTP 路由。
type Server struct {
	svc     *stats.Service
	timeout time.Duration
}

// New 用给定的 stats.Service 构造 Server。
func New(svc *stats.Service) *Server {
	return &Server{svc: svc, timeout: requestTimeout}
}

// Routes 注册并返回全部 HTTP 路由。
func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	// gRPC 方法 1:1 映射端点。
	mux.HandleFunc("/api/sysstats", s.handleSysStats)
	mux.HandleFunc("/api/stats", s.handleStat)
	mux.HandleFunc("/api/stats/online", s.handleStatOnline)
	mux.HandleFunc("/api/stats/online/iplist", s.handleStatOnlineIPList)
	mux.HandleFunc("/api/query", s.handleQuery)
	mux.HandleFunc("/api/online/users", s.handleOnlineUsers)

	// 业务聚合端点。
	mux.HandleFunc("/api/traffic", s.handleTraffic)
	mux.HandleFunc("/api/online", s.handleOnline)
	mux.HandleFunc("/api/all", s.handleAll)

	return mux
}

// Run 在 addr 上启动 HTTP 服务（阻塞）。
func (s *Server) Run(addr string) error {
	log.Printf("HTTP 服务启动，监听 %s", addr)
	log.Printf("端点: /api/sysstats /api/stats /api/stats/online /api/stats/online/iplist /api/query /api/online/users /api/traffic /api/online /api/all")
	return http.ListenAndServe(addr, s.Routes())
}
