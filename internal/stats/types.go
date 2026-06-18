// Package stats 在 Xray gRPC StatsService 之上提供面向 HTTP 的业务封装与 JSON 结构。
package stats

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

// StatJSON 单个计数器的取值（对应 gRPC GetStats / QueryStats 的 Stat）。
type StatJSON struct {
	Name       string `json:"name"`
	Value      int64  `json:"value"`
	ValueHuman string `json:"value_human"`
}

// OnlineCountJSON 单个计数器的在线数（对应 gRPC GetStatsOnline）。
type OnlineCountJSON struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
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
