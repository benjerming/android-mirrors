// Package cleanup 提供周期任务调度骨架。
//
// 当前阶段（M13 stub）只暴露 Start / Stop 接口与一个 ticker；
// 真实清理逻辑（过期 session 回收、ephemeral 实例延迟删除）会在
// session LastActiveAt 持久化后接入。
package cleanup

import (
	"context"
	"log/slog"
	"time"
)

// Scheduler 表示周期清理任务。
type Scheduler struct {
	interval time.Duration
	logger   *slog.Logger
	cancel   context.CancelFunc
	done     chan struct{}
}

// New 用来构造 Scheduler。interval <= 0 时退化为 30s。
func New(interval time.Duration, logger *slog.Logger) *Scheduler {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Scheduler{interval: interval, logger: logger}
}

// Start 启动后台 ticker；返回 nil 表示成功，已经启动则返回 ErrAlreadyRunning。
func (s *Scheduler) Start(ctx context.Context) {
	cctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	go s.loop(cctx)
}

// Stop 停止后台 ticker 并等待退出。
func (s *Scheduler) Stop() {
	if s.cancel == nil {
		return
	}
	s.cancel()
	<-s.done
}

func (s *Scheduler) loop(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.logger != nil {
				s.logger.DebugContext(ctx, "cleanup tick (no-op until B-22..B-25 lands)")
			}
		}
	}
}
