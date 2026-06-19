package hourly

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"vps/internal/humanize"
	"vps/internal/stats"
)

// 分类顺序与中文标签，与 stats.buildTraffic 保持一致。
var (
	categoryOrder  = []string{"user", "inbound", "outbound"}
	categoryLabels = map[string]string{
		"user":     "用户 (user)",
		"inbound":  "入站 (inbound)",
		"outbound": "出站 (outbound)",
	}
)

// entry 累计单个对象在某小时内的上下行净流量（增量累加）。
type entry struct {
	uplink, downlink int64
}

// bucket 是一个整点小时的全部流量：category -> objname -> *entry。
type bucket struct {
	hourUnix int64
	cats     map[string]map[string]*entry
}

func newBucket(hourUnix int64) *bucket {
	return &bucket{hourUnix: hourUnix, cats: map[string]map[string]*entry{}}
}

// entryOf 返回（必要时创建）指定 category/name 的累加项。
func (b *bucket) entryOf(category, name string) *entry {
	byName, ok := b.cats[category]
	if !ok {
		byName = map[string]*entry{}
		b.cats[category] = byName
	}
	e, ok := byName[name]
	if !ok {
		e = &entry{}
		byName[name] = e
	}
	return e
}

// Store 保存全部小时桶及上次采样的累计值基线，并负责 JSON 持久化与过期剪枝。
type Store struct {
	mu         sync.Mutex
	path       string            // JSON 文件路径
	retention  time.Duration     // 历史保留时长
	buckets    map[int64]*bucket // hourUnix -> bucket
	lastValues map[string]int64  // 计数器全名 -> 上次累计值（不持久化）
}

// NewStore 构造空 Store（尚未加载文件）。retention<=0 表示不剪枝。
func NewStore(path string, retention time.Duration) *Store {
	return &Store{
		path:       path,
		retention:  retention,
		buckets:    map[int64]*bucket{},
		lastValues: map[string]int64{},
	}
}

// Load 从 JSON 文件载入历史桶；文件不存在时视为空。载入后立即按保留期剪枝。
// lastValues 不持久化——重启后首次采样仅重建基线。
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var pf persistFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return err
	}
	for _, pb := range pf.Buckets {
		b := newBucket(pb.HourUnix)
		for _, pc := range pb.Categories {
			for _, pe := range pc.Entries {
				e := b.entryOf(pc.Category, pe.Name)
				e.uplink = pe.Uplink
				e.downlink = pe.Downlink
			}
		}
		s.buckets[pb.HourUnix] = b
	}
	s.prune(time.Now())
	return nil
}

// Record 执行一次采样的差分累加：对每个计数器算增量并累加进当前整点桶，
// 随后剪枝并落盘。now 通常为采样时刻。
func (s *Store) Record(samples []stats.StatJSON, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hk := hourStartUnix(now)
	b := s.buckets[hk]
	if b == nil {
		b = newBucket(hk)
		s.buckets[hk] = b
	}

	for _, st := range samples {
		parts := strings.Split(st.Name, ">>>")
		if len(parts) != 4 {
			continue // 跳过非预期格式的计数器
		}
		category, name, direction := parts[0], parts[1], parts[3]
		if direction != "uplink" && direction != "downlink" {
			continue
		}

		last, seen := s.lastValues[st.Name]
		s.lastValues[st.Name] = st.Value
		if !seen {
			continue // 首次见到，仅建基线，不计入流量
		}

		var delta int64
		if st.Value >= last {
			delta = st.Value - last
		} else {
			delta = st.Value // 计数器回退视为 Xray 重启，增量取新值
		}
		if delta == 0 {
			continue
		}

		e := b.entryOf(category, name)
		if direction == "uplink" {
			e.uplink += delta
		} else {
			e.downlink += delta
		}
	}

	s.prune(now)
	return s.flush()
}

// Query 返回 [from, to]（含端点，按整点对齐）区间内的小时桶，按时间升序。
func (s *Store) Query(from, to time.Time) []HourlyJSON {
	s.mu.Lock()
	defer s.mu.Unlock()

	fromU := hourStartUnix(from)
	toU := hourStartUnix(to)

	keys := make([]int64, 0, len(s.buckets))
	for hk := range s.buckets {
		if hk >= fromU && hk <= toU {
			keys = append(keys, hk)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	out := make([]HourlyJSON, 0, len(keys))
	for _, hk := range keys {
		out = append(out, bucketToJSON(s.buckets[hk]))
	}
	return out
}

// Flush 立即落盘（用于优雅退出时的最后一次写盘）。
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flush()
}

// ---------- 内部辅助（调用方需持有 s.mu） ----------

// prune 删除早于 now-retention 的桶；retention<=0 时不剪枝。
func (s *Store) prune(now time.Time) {
	if s.retention <= 0 {
		return
	}
	cutoff := now.Add(-s.retention).Unix()
	for hk := range s.buckets {
		if hk < cutoff {
			delete(s.buckets, hk)
		}
	}
}

// flush 将全部桶序列化为 JSON 文件（临时文件 + 原子 rename）。
func (s *Store) flush() error {
	pf := persistFile{Buckets: make([]persistBucket, 0, len(s.buckets))}

	keys := make([]int64, 0, len(s.buckets))
	for hk := range s.buckets {
		keys = append(keys, hk)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	for _, hk := range keys {
		pf.Buckets = append(pf.Buckets, bucketToPersist(s.buckets[hk]))
	}

	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return err
	}

	if dir := filepath.Dir(s.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// hourStartUnix 返回 t 所在整点（服务器本地时区）的 Unix 秒。
func hourStartUnix(t time.Time) int64 {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location()).Unix()
}

// bucketToPersist 把内存桶转为持久化结构（分类与对象名均排序，输出稳定）。
func bucketToPersist(b *bucket) persistBucket {
	pb := persistBucket{HourUnix: b.hourUnix}
	for _, category := range categoryOrder {
		byName, ok := b.cats[category]
		if !ok {
			continue
		}
		pb.Categories = append(pb.Categories, persistCategory{
			Category: category,
			Entries:  sortedEntries(byName),
		})
	}
	return pb
}

// bucketToJSON 把内存桶转为对外 JSON（附人类可读字段）。
func bucketToJSON(b *bucket) HourlyJSON {
	hj := HourlyJSON{
		HourUnix: b.hourUnix,
		Hour:     time.Unix(b.hourUnix, 0).Format(time.RFC3339),
	}
	for _, category := range categoryOrder {
		byName, ok := b.cats[category]
		if !ok {
			continue
		}
		label := categoryLabels[category]
		if label == "" {
			label = category
		}
		pes := sortedEntries(byName)
		entries := make([]stats.TrafficEntryJSON, 0, len(pes))
		for _, pe := range pes {
			total := pe.Uplink + pe.Downlink
			entries = append(entries, stats.TrafficEntryJSON{
				Name:          pe.Name,
				Uplink:        pe.Uplink,
				UplinkHuman:   humanize.FormatBytes(pe.Uplink),
				Downlink:      pe.Downlink,
				DownlinkHuman: humanize.FormatBytes(pe.Downlink),
				Total:         total,
				TotalHuman:    humanize.FormatBytes(total),
			})
		}
		hj.Categories = append(hj.Categories, CategoryTrafficJSON{
			Category: category,
			Label:    label,
			Entries:  entries,
		})
	}
	return hj
}

// sortedEntries 把某类的对象映射转为按对象名排序的持久化条目切片。
func sortedEntries(byName map[string]*entry) []persistEntry {
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]persistEntry, 0, len(names))
	for _, name := range names {
		e := byName[name]
		out = append(out, persistEntry{Name: name, Uplink: e.uplink, Downlink: e.downlink})
	}
	return out
}
