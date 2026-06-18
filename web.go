package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/xtls/xray-core/app/stats/command"
)

// requestTimeout 单个 HTTP 请求查询 Xray 的超时时间。
const requestTimeout = 5 * time.Second

// trafficEntry 聚合单个对象的上下行流量（中间结构）。
type trafficEntry struct {
	name             string
	uplink, downlink int64
}

// ---------- JSON 响应结构体 ----------

// SysStatsJSON 对应 Xray 系统状态。
type SysStatsJSON struct {
	UptimeSeconds   uint32 `json:"uptime_seconds"`
	NumGoroutine    uint32 `json:"num_goroutine"`
	NumGC           uint32 `json:"num_gc"`
	AllocBytes      uint64 `json:"alloc_bytes"`
	AllocHuman      string `json:"alloc_human"`
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	TotalAllocHuman string `json:"total_alloc_human"`
	SysBytes        uint64 `json:"sys_bytes"`
	SysHuman        string `json:"sys_human"`
}

// TrafficEntryJSON 单个对象的上下行流量。
type TrafficEntryJSON struct {
	Name          string `json:"name"`
	Uplink        int64  `json:"uplink"`
	UplinkHuman   string `json:"uplink_human"`
	Downlink      int64  `json:"downlink"`
	DownlinkHuman string `json:"downlink_human"`
	Total         int64  `json:"total"`
	TotalHuman    string `json:"total_human"`
}

// CategoryJSON 一类流量（user / inbound / outbound）。
type CategoryJSON struct {
	Category string             `json:"category"`
	Label    string             `json:"label"`
	Entries  []TrafficEntryJSON `json:"entries"`
}

// OnlineIPJSON 在线 IP 及最后活跃时间。
type OnlineIPJSON struct {
	IP           string `json:"ip"`
	LastSeenUnix int64  `json:"last_seen_unix"`
	LastSeen     string `json:"last_seen"`
}

// OnlineUserJSON 在线用户及其来源 IP。
type OnlineUserJSON struct {
	User string         `json:"user"`
	IPs  []OnlineIPJSON `json:"ips"`
}

// AllJSON 三类数据的聚合。
type AllJSON struct {
	SysStats *SysStatsJSON    `json:"sys_stats"`
	Traffic  []CategoryJSON   `json:"traffic"`
	Online   []OnlineUserJSON `json:"online"`
}

// ---------- 数据构造（纯函数 / 查询封装） ----------

// buildSysStats 调用 GetSysStats 并转换为 JSON 结构体。
func buildSysStats(ctx context.Context, client command.StatsServiceClient) (*SysStatsJSON, error) {
	resp, err := client.GetSysStats(ctx, &command.SysStatsRequest{})
	if err != nil {
		return nil, err
	}
	return &SysStatsJSON{
		UptimeSeconds:   resp.Uptime,
		NumGoroutine:    resp.NumGoroutine,
		NumGC:           resp.NumGC,
		AllocBytes:      resp.Alloc,
		AllocHuman:      formatBytes(int64(resp.Alloc)),
		TotalAllocBytes: resp.TotalAlloc,
		TotalAllocHuman: formatBytes(int64(resp.TotalAlloc)),
		SysBytes:        resp.Sys,
		SysHuman:        formatBytes(int64(resp.Sys)),
	}, nil
}

// buildTraffic 把计数器列表（名称形如 category>>>name>>>traffic>>>direction）
// 按 category -> name 聚合上下行，返回固定顺序（user/inbound/outbound）的分类结果。
func buildTraffic(stats []*command.Stat) []CategoryJSON {
	categories := map[string]map[string]*trafficEntry{}
	for _, stat := range stats {
		parts := strings.Split(stat.Name, ">>>")
		if len(parts) != 4 {
			continue // 跳过非预期格式的计数器
		}
		category, name, direction := parts[0], parts[1], parts[3]

		byName, ok := categories[category]
		if !ok {
			byName = map[string]*trafficEntry{}
			categories[category] = byName
		}
		entry, ok := byName[name]
		if !ok {
			entry = &trafficEntry{name: name}
			byName[name] = entry
		}
		switch direction {
		case "uplink":
			entry.uplink = stat.Value
		case "downlink":
			entry.downlink = stat.Value
		}
	}

	labels := map[string]string{
		"user":     "用户 (user)",
		"inbound":  "入站 (inbound)",
		"outbound": "出站 (outbound)",
	}
	order := []string{"user", "inbound", "outbound"}

	result := make([]CategoryJSON, 0, len(order))
	for _, category := range order {
		byName, ok := categories[category]
		if !ok {
			continue
		}
		label := labels[category]
		if label == "" {
			label = category
		}

		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		sort.Strings(names)

		entries := make([]TrafficEntryJSON, 0, len(names))
		for _, name := range names {
			e := byName[name]
			total := e.uplink + e.downlink
			entries = append(entries, TrafficEntryJSON{
				Name:          e.name,
				Uplink:        e.uplink,
				UplinkHuman:   formatBytes(e.uplink),
				Downlink:      e.downlink,
				DownlinkHuman: formatBytes(e.downlink),
				Total:         total,
				TotalHuman:    formatBytes(total),
			})
		}

		result = append(result, CategoryJSON{
			Category: category,
			Label:    label,
			Entries:  entries,
		})
	}
	return result
}

// buildOnlineUsers 查询所有在线用户及其来源 IP。
func buildOnlineUsers(ctx context.Context, client command.StatsServiceClient) ([]OnlineUserJSON, error) {
	usersResp, err := client.GetAllOnlineUsers(ctx, &command.GetAllOnlineUsersRequest{})
	if err != nil {
		return nil, err
	}

	users := append([]string(nil), usersResp.Users...)
	sort.Strings(users)

	result := make([]OnlineUserJSON, 0, len(users))
	for _, user := range users {
		entry := OnlineUserJSON{User: user, IPs: []OnlineIPJSON{}}

		ipResp, err := client.GetStatsOnlineIpList(ctx, &command.GetStatsRequest{Name: user})
		if err != nil {
			// 单个用户查询失败不影响整体，IPs 保持空。
			result = append(result, entry)
			continue
		}

		ips := make([]string, 0, len(ipResp.Ips))
		for ip := range ipResp.Ips {
			ips = append(ips, ip)
		}
		sort.Strings(ips)

		for _, ip := range ips {
			ts := ipResp.Ips[ip]
			entry.IPs = append(entry.IPs, OnlineIPJSON{
				IP:           ip,
				LastSeenUnix: ts,
				LastSeen:     time.Unix(ts, 0).Format("2006-01-02 15:04:05"),
			})
		}
		result = append(result, entry)
	}
	return result, nil
}

// ---------- HTTP 层 ----------

// writeJSON 以美化后的 JSON 写出响应。
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// writeError 返回 500 + 错误信息。
func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func startWebServer(addr string, client command.StatsServiceClient) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/sysstats", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		data, err := buildSysStats(ctx, client)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, data)
	})

	mux.HandleFunc("/api/traffic", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		resp, err := client.QueryStats(ctx, &command.QueryStatsRequest{Pattern: "", Reset_: false})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, buildTraffic(resp.Stat))
	})

	mux.HandleFunc("/api/online", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		data, err := buildOnlineUsers(ctx, client)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, data)
	})

	mux.HandleFunc("/api/all", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()

		sys, err := buildSysStats(ctx, client)
		if err != nil {
			writeError(w, err)
			return
		}
		resp, err := client.QueryStats(ctx, &command.QueryStatsRequest{Pattern: "", Reset_: false})
		if err != nil {
			writeError(w, err)
			return
		}
		online, err := buildOnlineUsers(ctx, client)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, AllJSON{
			SysStats: sys,
			Traffic:  buildTraffic(resp.Stat),
			Online:   online,
		})
	})

	log.Printf("HTTP 服务启动，监听 %s（接口: /api/sysstats /api/traffic /api/online /api/all）", addr)
	return http.ListenAndServe(addr, mux)
}
