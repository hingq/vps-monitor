package stats

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/xtls/xray-core/app/stats/command"

	"vps/internal/humanize"
)

// Service 在 StatsService gRPC 客户端之上提供业务方法，整个进程生命周期内复用同一客户端。
type Service struct {
	client command.StatsServiceClient
}

// NewService 用给定的 StatsService 客户端构造 Service。
func NewService(client command.StatsServiceClient) *Service {
	return &Service{client: client}
}

// ---------- gRPC ----------

// SysStats 调用 GetSysStats 并转换为 JSON 结构体。
func (s *Service) SysStats(ctx context.Context) (*SysStatsJSON, error) {
	resp, err := s.client.GetSysStats(ctx, &command.SysStatsRequest{})
	if err != nil {
		return nil, err
	}
	return &SysStatsJSON{
		UptimeSeconds:   resp.Uptime,
		NumGoroutine:    resp.NumGoroutine,
		NumGC:           resp.NumGC,
		AllocBytes:      resp.Alloc,
		AllocHuman:      humanize.FormatBytes(int64(resp.Alloc)),
		TotalAllocBytes: resp.TotalAlloc,
		TotalAllocHuman: humanize.FormatBytes(int64(resp.TotalAlloc)),
		SysBytes:        resp.Sys,
		SysHuman:        humanize.FormatBytes(int64(resp.Sys)),
	}, nil
}

// GetStat 调用 GetStats 获取单个计数器取值。
func (s *Service) GetStat(ctx context.Context, name string, reset bool) (*StatJSON, error) {
	resp, err := s.client.GetStats(ctx, &command.GetStatsRequest{Name: name, Reset_: reset})
	if err != nil {
		return nil, err
	}
	return statToJSON(resp.Stat), nil
}

// GetStatOnline 调用 GetStatsOnline 获取单个计数器的在线数。
func (s *Service) GetStatOnline(ctx context.Context, name string, reset bool) (*OnlineCountJSON, error) {
	resp, err := s.client.GetStatsOnline(ctx, &command.GetStatsRequest{Name: name, Reset_: reset})
	if err != nil {
		return nil, err
	}
	out := &OnlineCountJSON{Name: name}
	if resp.Stat != nil {
		out.Name = resp.Stat.Name
		out.Value = resp.Stat.Value
	}
	return out, nil
}

// OnlineIPList 调用 GetStatsOnlineIpList 获取单个用户的在线 IP 列表（按 IP 排序）。
func (s *Service) OnlineIPList(ctx context.Context, name string) ([]OnlineIPJSON, error) {
	resp, err := s.client.GetStatsOnlineIpList(ctx, &command.GetStatsRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return ipsToJSON(resp.Ips), nil
}

// QueryRaw 调用 QueryStats 返回原始（未聚合）计数器列表。
func (s *Service) QueryRaw(ctx context.Context, pattern string, reset bool) ([]StatJSON, error) {
	resp, err := s.client.QueryStats(ctx, &command.QueryStatsRequest{Pattern: pattern, Reset_: reset})
	if err != nil {
		return nil, err
	}
	result := make([]StatJSON, 0, len(resp.Stat))
	for _, stat := range resp.Stat {
		result = append(result, *statToJSON(stat))
	}
	return result, nil
}

// AllOnlineUsers 调用 GetAllOnlineUsers 返回排序后的在线用户名列表。
func (s *Service) AllOnlineUsers(ctx context.Context) ([]string, error) {
	resp, err := s.client.GetAllOnlineUsers(ctx, &command.GetAllOnlineUsersRequest{})
	if err != nil {
		return nil, err
	}
	users := append([]string(nil), resp.Users...)
	sort.Strings(users)
	return users, nil
}

// ---------- 业务聚合方法 ----------

// Traffic 调用 QueryStats，并把计数器列表（名称形如 category>>>name>>>traffic>>>direction）
// 按 category -> name 聚合上下行，返回固定顺序（user/inbound/outbound）的分类结果。
func (s *Service) Traffic(ctx context.Context) ([]CategoryJSON, error) {
	resp, err := s.client.QueryStats(ctx, &command.QueryStatsRequest{Pattern: "", Reset_: false})
	if err != nil {
		return nil, err
	}
	return buildTraffic(resp.Stat), nil
}

// OnlineUsers 查询所有在线用户及其来源 IP。
func (s *Service) OnlineUsers(ctx context.Context) ([]OnlineUserJSON, error) {
	users, err := s.AllOnlineUsers(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]OnlineUserJSON, 0, len(users))
	for _, user := range users {
		entry := OnlineUserJSON{User: user, IPs: []OnlineIPJSON{}}

		ips, err := s.OnlineIPList(ctx, user)
		if err != nil {
			// 单个用户查询失败不影响整体，IPs 保持空。
			result = append(result, entry)
			continue
		}
		entry.IPs = ips
		result = append(result, entry)
	}
	return result, nil
}

// All 聚合系统状态、流量与在线用户三类数据。
func (s *Service) All(ctx context.Context) (*AllJSON, error) {
	sys, err := s.SysStats(ctx)
	if err != nil {
		return nil, err
	}
	traffic, err := s.Traffic(ctx)
	if err != nil {
		return nil, err
	}
	online, err := s.OnlineUsers(ctx)
	if err != nil {
		return nil, err
	}
	return &AllJSON{SysStats: sys, Traffic: traffic, Online: online}, nil
}

// ---------- 内部辅助 ----------

func statToJSON(stat *command.Stat) *StatJSON {
	if stat == nil {
		return &StatJSON{ValueHuman: humanize.FormatBytes(0)}
	}
	return &StatJSON{
		Name:       stat.Name,
		Value:      stat.Value,
		ValueHuman: humanize.FormatBytes(stat.Value),
	}
}

func ipsToJSON(raw map[string]int64) []OnlineIPJSON {
	ips := make([]string, 0, len(raw))
	for ip := range raw {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	out := make([]OnlineIPJSON, 0, len(ips))
	for _, ip := range ips {
		ts := raw[ip]
		out = append(out, OnlineIPJSON{
			IP:           ip,
			LastSeenUnix: ts,
			LastSeen:     time.Unix(ts, 0).Format("2006-01-02 15:04:05"),
		})
	}
	return out
}

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
				UplinkHuman:   humanize.FormatBytes(e.uplink),
				Downlink:      e.downlink,
				DownlinkHuman: humanize.FormatBytes(e.downlink),
				Total:         total,
				TotalHuman:    humanize.FormatBytes(total),
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
