package server

import (
	"context"
	"net/http"
)

// withTimeout 为请求派生一个带超时的 context。
func (s *Server) withTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), s.timeout)
}

// handleSysStats 对应 gRPC GetSysStats。
func (s *Server) handleSysStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.svc.SysStats(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// handleStat 对应 gRPC GetStats，查询单个计数器取值。
func (s *Server) handleStat(w http.ResponseWriter, r *http.Request) {
	name, ok := requireParam(w, r, "name")
	if !ok {
		return
	}
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.svc.GetStat(ctx, name, queryBool(r, "reset"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// handleStatOnline 对应 gRPC GetStatsOnline，查询单个计数器在线数。
func (s *Server) handleStatOnline(w http.ResponseWriter, r *http.Request) {
	name, ok := requireParam(w, r, "name")
	if !ok {
		return
	}
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.svc.GetStatOnline(ctx, name, queryBool(r, "reset"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// handleStatOnlineIPList 对应 gRPC GetStatsOnlineIpList，查询单个用户在线 IP 列表。
func (s *Server) handleStatOnlineIPList(w http.ResponseWriter, r *http.Request) {
	name, ok := requireParam(w, r, "name")
	if !ok {
		return
	}
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.svc.OnlineIPList(ctx, name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// handleQuery 对应 gRPC QueryStats，返回原始（未聚合）计数器列表。
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.svc.QueryRaw(ctx, r.URL.Query().Get("pattern"), queryBool(r, "reset"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// handleOnlineUsers 对应 gRPC GetAllOnlineUsers，返回在线用户名列表。
func (s *Server) handleOnlineUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.svc.AllOnlineUsers(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// handleTraffic 聚合端点：QueryStats 结果按 category/name 聚合上下行。
func (s *Server) handleTraffic(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.svc.Traffic(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// handleOnline 聚合端点：在线用户及其来源 IP。
func (s *Server) handleOnline(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.svc.OnlineUsers(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// handleAll 聚合端点：sysstats + traffic + online 三合一。
func (s *Server) handleAll(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.svc.All(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}
