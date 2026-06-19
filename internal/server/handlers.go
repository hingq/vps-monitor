package server

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"vps/internal/hourly"
)

// defaultHourlyWindow 是 /api/traffic/hourly 未指定时间范围时默认回溯的小时数。
const defaultHourlyWindow = 24

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

// handleTrafficHourly 返回每小时流量序列。
// 时间范围：优先用 from/to（RFC3339 或 Unix 秒）；否则回溯最近 hours 小时（默认 24）。
// 可选 category、name 过滤。数据来自后台采样器，纯读内存。
func (s *Server) handleTrafficHourly(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	now := time.Now()

	var from, to time.Time
	if q.Get("from") != "" || q.Get("to") != "" {
		var ok bool
		if from, ok = parseTimeParam(w, q.Get("from"), "from"); !ok {
			return
		}
		if to, ok = parseTimeParam(w, q.Get("to"), "to"); !ok {
			return
		}
		if q.Get("to") == "" {
			to = now
		}
	} else {
		hours := defaultHourlyWindow
		if v := q.Get("hours"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hours 必须为正整数"})
				return
			}
			hours = n
		}
		to = now
		from = now.Add(-time.Duration(hours) * time.Hour)
	}

	data := s.hourly.Query(from, to)
	if cat := q.Get("category"); cat != "" {
		data = filterHourly(data, cat, q.Get("name"))
	}
	writeJSON(w, http.StatusOK, data)
}

// filterHourly 按 category（及可选 name）过滤每小时数据，剔除过滤后为空的小时。
func filterHourly(hours []hourly.HourlyJSON, category, name string) []hourly.HourlyJSON {
	out := make([]hourly.HourlyJSON, 0, len(hours))
	for _, h := range hours {
		cats := make([]hourly.CategoryTrafficJSON, 0, len(h.Categories))
		for _, c := range h.Categories {
			if c.Category != category {
				continue
			}
			if name != "" {
				entries := c.Entries[:0:0]
				for _, e := range c.Entries {
					if e.Name == name {
						entries = append(entries, e)
					}
				}
				c.Entries = entries
			}
			if len(c.Entries) > 0 {
				cats = append(cats, c)
			}
		}
		if len(cats) > 0 {
			h.Categories = cats
			out = append(out, h)
		}
	}
	return out
}

// parseTimeParam 解析时间参数，支持 RFC3339 与 Unix 秒；空值返回零值且 ok=true（由调用方决定默认）。
func parseTimeParam(w http.ResponseWriter, v, key string) (time.Time, bool) {
	if v == "" {
		return time.Time{}, true
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, true
	}
	if sec, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Unix(sec, 0), true
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": key + " 时间格式无效（需 RFC3339 或 Unix 秒）"})
	return time.Time{}, false
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
