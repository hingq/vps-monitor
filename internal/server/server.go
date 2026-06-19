// Package server 实现 HTTP 层：路由注册与请求处理。
package server

import (
	"log"
	"net/http"
	"time"

	"vps/internal/hourly"
	"vps/internal/proxyman"
	"vps/internal/routing"
	"vps/internal/stats"
)

// requestTimeout 单个 HTTP 请求查询 Xray 的超时时间。
const requestTimeout = 5 * time.Second

// Server 持有业务层依赖并对外提供 HTTP 路由。
type Server struct {
	svc        *stats.Service
	pm         *proxyman.Service
	routing    *routing.Service
	hourly     *hourly.Store
	adminToken string
	timeout    time.Duration
}

// New 用给定的各业务 Service 与写操作鉴权 token 构造 Server。
// adminToken 为空时，所有写操作端点一律拒绝（403）。
func New(svc *stats.Service, pm *proxyman.Service, routing *routing.Service, hourlyStore *hourly.Store, adminToken string) *Server {
	return &Server{svc: svc, pm: pm, routing: routing, hourly: hourlyStore, adminToken: adminToken, timeout: requestTimeout}
}

// Routes 注册并返回全部 HTTP 路由。
func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	// StatsService：gRPC 方法 1:1 映射端点。
	mux.HandleFunc("/api/sysstats", s.handleSysStats)
	mux.HandleFunc("/api/stats", s.handleStat)
	mux.HandleFunc("/api/stats/online", s.handleStatOnline)
	mux.HandleFunc("/api/stats/online/iplist", s.handleStatOnlineIPList)
	mux.HandleFunc("/api/query", s.handleQuery)
	mux.HandleFunc("/api/online/users", s.handleOnlineUsers)

	// StatsService：业务聚合端点。
	mux.HandleFunc("/api/traffic", s.handleTraffic)
	mux.HandleFunc("/api/traffic/hourly", s.handleTrafficHourly)
	mux.HandleFunc("/api/online", s.handleOnline)
	mux.HandleFunc("/api/all", s.handleAll)

	// HandlerService：inbound 用户管理（写操作需鉴权）。
	mux.HandleFunc("GET /api/inbounds/users", s.handleListUsers)
	mux.HandleFunc("GET /api/inbounds/users/count", s.handleCountUsers)
	mux.HandleFunc("POST /api/inbounds/users", s.requireToken(s.handleAddUser))
	mux.HandleFunc("DELETE /api/inbounds/users", s.requireToken(s.handleRemoveUser))

	// RoutingService：只读路由规则。
	mux.HandleFunc("GET /api/routing/rules", s.handleListRules)

	return mux
}

// Run 在 addr 上启动 HTTP 服务（阻塞）。
func (s *Server) Run(addr string) error {
	log.Printf("HTTP 服务启动，监听 %s", addr)
	log.Printf("只读端点: /api/sysstats /api/stats /api/stats/online /api/stats/online/iplist /api/query /api/online/users /api/traffic /api/traffic/hourly /api/online /api/all /api/inbounds/users /api/inbounds/users/count /api/routing/rules")
	log.Printf("写端点(需 token): POST/DELETE /api/inbounds/users")
	return http.ListenAndServe(addr, s.Routes())
}
