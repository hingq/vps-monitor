package hourly

import (
	"context"
	"log"
	"time"

	"vps/internal/stats"
)

// sampleTimeout 单次采样查询 Xray 的超时时间。
const sampleTimeout = 5 * time.Second

// rawQuerier 是 Collector 依赖的最小接口：仅需拉取原始计数器列表。
// stats.Service 已实现该方法，可直接注入。
type rawQuerier interface {
	QueryRaw(ctx context.Context, pattern string, reset bool) ([]stats.StatJSON, error)
}

// Collector 周期性采样累计计数器并交给 Store 差分累加。
type Collector struct {
	q        rawQuerier
	store    *Store
	interval time.Duration
}

// NewCollector 构造采样器。
func NewCollector(q rawQuerier, store *Store, interval time.Duration) *Collector {
	return &Collector{q: q, store: store, interval: interval}
}

// Run 启动采样循环（阻塞），直到 ctx 取消；退出前做最后一次落盘。
// 建议在独立 goroutine 中调用。
func (c *Collector) Run(ctx context.Context) {
	log.Printf("每小时流量采样器启动，采样间隔 %s", c.interval)
	// 立即采样一次以尽快建立基线。
	c.sample(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := c.store.Flush(); err != nil {
				log.Printf("每小时流量退出落盘失败: %v", err)
			}
			log.Printf("每小时流量采样器已停止")
			return
		case <-ticker.C:
			c.sample(ctx)
		}
	}
}

// sample 执行一次采样：拉取原始计数器并交给 Store。失败只记日志不中断循环。
func (c *Collector) sample(ctx context.Context) {
	sctx, cancel := context.WithTimeout(ctx, sampleTimeout)
	defer cancel()

	samples, err := c.q.QueryRaw(sctx, "", false)
	if err != nil {
		log.Printf("每小时流量采样失败: %v", err)
		return
	}
	if err := c.store.Record(samples, time.Now()); err != nil {
		log.Printf("每小时流量写盘失败: %v", err)
	}
}
