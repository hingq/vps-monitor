package hourly

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"vps/internal/stats"
)

// sample 构造一条计数器记录。
func sample(name string, value int64) stats.StatJSON {
	return stats.StatJSON{Name: name, Value: value}
}

// findEntry 在 Query 结果里定位某小时某 category/name 的条目。
func findEntry(t *testing.T, hours []HourlyJSON, hourUnix int64, category, name string) (stats.TrafficEntryJSON, bool) {
	t.Helper()
	for _, h := range hours {
		if h.HourUnix != hourUnix {
			continue
		}
		for _, c := range h.Categories {
			if c.Category != category {
				continue
			}
			for _, e := range c.Entries {
				if e.Name == name {
					return e, true
				}
			}
		}
	}
	return stats.TrafficEntryJSON{}, false
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "hourly.json"), 30*24*time.Hour)
}

// 首次采样仅建基线，不产生流量。
func TestRecord_FirstSampleBaselineOnly(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2026, 6, 19, 12, 30, 0, 0, time.Local)

	if err := s.Record([]stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", 1000)}, now); err != nil {
		t.Fatalf("Record: %v", err)
	}

	hk := hourStartUnix(now)
	if _, ok := findEntry(t, s.Query(now, now), hk, "user", "alice"); ok {
		t.Fatal("首次采样不应产生流量条目")
	}
}

// 正常差分：第二次采样产生增量。
func TestRecord_Delta(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2026, 6, 19, 12, 30, 0, 0, time.Local)

	_ = s.Record([]stats.StatJSON{
		sample("user>>>alice>>>traffic>>>uplink", 1000),
		sample("user>>>alice>>>traffic>>>downlink", 2000),
	}, now)
	_ = s.Record([]stats.StatJSON{
		sample("user>>>alice>>>traffic>>>uplink", 1500),
		sample("user>>>alice>>>traffic>>>downlink", 5000),
	}, now)

	e, ok := findEntry(t, s.Query(now, now), hourStartUnix(now), "user", "alice")
	if !ok {
		t.Fatal("应有 alice 条目")
	}
	if e.Uplink != 500 || e.Downlink != 3000 {
		t.Fatalf("增量错误: up=%d down=%d，期望 up=500 down=3000", e.Uplink, e.Downlink)
	}
	if e.Total != 3500 {
		t.Fatalf("total=%d，期望 3500", e.Total)
	}
}

// 计数器回退（Xray 重启）：增量取新值。
func TestRecord_CounterReset(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2026, 6, 19, 12, 30, 0, 0, time.Local)

	_ = s.Record([]stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", 1000)}, now)
	_ = s.Record([]stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", 1300)}, now) // +300
	_ = s.Record([]stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", 200)}, now)  // 回退 -> +200

	e, _ := findEntry(t, s.Query(now, now), hourStartUnix(now), "user", "alice")
	if e.Uplink != 500 {
		t.Fatalf("uplink=%d，期望 500 (300+200)", e.Uplink)
	}
}

// 跨整点：增量分别落入不同小时桶。
func TestRecord_HourRollover(t *testing.T) {
	s := newTestStore(t)
	h1 := time.Date(2026, 6, 19, 12, 30, 0, 0, time.Local)
	h2 := time.Date(2026, 6, 19, 13, 5, 0, 0, time.Local)

	_ = s.Record([]stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", 1000)}, h1)
	_ = s.Record([]stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", 1400)}, h1) // +400 -> 12点桶
	_ = s.Record([]stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", 1900)}, h2) // +500 -> 13点桶

	hours := s.Query(h1, h2)
	if len(hours) != 2 {
		t.Fatalf("应有 2 个小时桶，实得 %d", len(hours))
	}
	if e, _ := findEntry(t, hours, hourStartUnix(h1), "user", "alice"); e.Uplink != 400 {
		t.Fatalf("12点桶 uplink=%d，期望 400", e.Uplink)
	}
	if e, _ := findEntry(t, hours, hourStartUnix(h2), "user", "alice"); e.Uplink != 500 {
		t.Fatalf("13点桶 uplink=%d，期望 500", e.Uplink)
	}
}

// 非法计数器名与非流量方向被忽略。
func TestRecord_SkipsMalformed(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2026, 6, 19, 12, 30, 0, 0, time.Local)

	bad := []stats.StatJSON{
		sample("badname", 100),
		sample("a>>>b>>>c", 100),
		sample("user>>>alice>>>traffic>>>weird", 100),
	}
	_ = s.Record(bad, now)
	_ = s.Record(bad, now)

	if hours := s.Query(now, now); len(hours) != 0 {
		for _, h := range hours {
			if len(h.Categories) != 0 {
				t.Fatalf("非法计数器不应产生条目: %+v", h)
			}
		}
	}
}

// 剪枝：超过保留期的桶被删除。
func TestPrune_DropsOldBuckets(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "hourly.json"), 24*time.Hour)
	old := time.Date(2026, 6, 1, 10, 0, 0, 0, time.Local)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.Local)

	_ = s.Record([]stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", 1000)}, old)
	_ = s.Record([]stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", 1400)}, old)
	// 一次远在未来的采样触发对旧桶的剪枝。
	_ = s.Record([]stats.StatJSON{sample("user>>>bob>>>traffic>>>uplink", 1)}, now)

	if hours := s.Query(old, now); len(hours) != 1 {
		t.Fatalf("旧桶应被剪枝，仅剩当前桶，实得 %d 个桶", len(hours))
	}
}

// JSON 持久化往返：Load 后历史桶可查。
func TestPersistence_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "hourly.json")
	now := time.Date(2026, 6, 19, 12, 30, 0, 0, time.Local)

	s1 := NewStore(path, 30*24*time.Hour)
	_ = s1.Record([]stats.StatJSON{sample("inbound>>>vless>>>traffic>>>downlink", 1000)}, now)
	_ = s1.Record([]stats.StatJSON{sample("inbound>>>vless>>>traffic>>>downlink", 6000)}, now) // +5000

	s2 := NewStore(path, 30*24*time.Hour)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	e, ok := findEntry(t, s2.Query(now, now), hourStartUnix(now), "inbound", "vless")
	if !ok {
		t.Fatal("Load 后应能查到历史桶")
	}
	if e.Downlink != 5000 {
		t.Fatalf("downlink=%d，期望 5000", e.Downlink)
	}
}

// Load 不存在的文件不报错。
func TestLoad_MissingFile(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "nope.json"), time.Hour)
	if err := s.Load(); err != nil {
		t.Fatalf("缺失文件应视为空而非报错: %v", err)
	}
}

// 并发读写：-race 下不应有数据竞争。
func TestConcurrentReadWrite(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2026, 6, 19, 12, 30, 0, 0, time.Local)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(v int64) {
			defer wg.Done()
			_ = s.Record([]stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", v)}, now)
		}(int64(i * 100))
		go func() {
			defer wg.Done()
			_ = s.Query(now, now)
		}()
	}
	wg.Wait()
}

// fakeQuerier 实现 rawQuerier，供 Collector 测试。
type fakeQuerier struct {
	mu     sync.Mutex
	values []int64 // 每次调用返回的 uplink 累计值序列
	calls  int
}

func (f *fakeQuerier) QueryRaw(_ context.Context, _ string, _ bool) ([]stats.StatJSON, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v := f.values[len(f.values)-1]
	if f.calls < len(f.values) {
		v = f.values[f.calls]
	}
	f.calls++
	return []stats.StatJSON{sample("user>>>alice>>>traffic>>>uplink", v)}, nil
}

// Collector 采样若干次后累计正确，ctx 取消后退出并落盘。
func TestCollector_RunSamplesAndStops(t *testing.T) {
	s := newTestStore(t)
	q := &fakeQuerier{values: []int64{1000, 1200, 1500}}
	c := NewCollector(q, s, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	// 等到至少完成 3 次采样。
	deadline := time.After(2 * time.Second)
	for {
		q.mu.Lock()
		n := q.calls
		q.mu.Unlock()
		if n >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("采样未在期限内完成")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-done

	now := time.Now()
	e, ok := findEntry(t, s.Query(now, now), hourStartUnix(now), "user", "alice")
	if !ok {
		t.Fatal("应有 alice 条目")
	}
	// 基线 1000，之后 +200、+300 = 500（之后的采样可能继续累加，至少 >=500）。
	if e.Uplink < 500 {
		t.Fatalf("uplink=%d，至少应为 500", e.Uplink)
	}
}
