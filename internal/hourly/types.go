// Package hourly 在 Xray 累计流量计数器之上提供"每小时流量"统计：
// 后台定时采样累计值并差分，按整点小时桶累加，落盘为 JSON 文件并支持查询。
package hourly

import "vps/internal/stats"

// ---------- JSON 响应结构 ----------

// HourlyJSON 表示某个整点小时的流量切片。
type HourlyJSON struct {
	HourUnix   int64                 `json:"hour_unix"`
	Hour       string                `json:"hour"` // 带时区的 RFC3339（服务器本地时区）
	Categories []CategoryTrafficJSON `json:"categories"`
}

// CategoryTrafficJSON 表示一小时内某一类（user/inbound/outbound）的流量。
type CategoryTrafficJSON struct {
	Category string                   `json:"category"`
	Label    string                   `json:"label"`
	Entries  []stats.TrafficEntryJSON `json:"entries"`
}

// ---------- 持久化结构（JSON 文件） ----------

// persistFile 是落盘 JSON 文件的顶层结构。
type persistFile struct {
	Buckets []persistBucket `json:"buckets"`
}

// persistBucket 是单个整点小时桶的持久化形态。
type persistBucket struct {
	HourUnix   int64             `json:"hour_unix"`
	Categories []persistCategory `json:"categories"`
}

// persistCategory 是桶内某一类的持久化形态。
type persistCategory struct {
	Category string         `json:"category"`
	Entries  []persistEntry `json:"entries"`
}

// persistEntry 是某对象在某小时的上下行净流量。
type persistEntry struct {
	Name     string `json:"name"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}
