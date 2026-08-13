package service

import (
	"sync"
	"time"
)

// OperationProgress 是长任务向订阅者（SSE）推送的进度事件。
type OperationProgress struct {
	Type      string `json:"type"`
	Status    string `json:"status"`
	Stage     string `json:"stage"`
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Success   int    `json:"success"`
	Failed    int    `json:"failed"`
	NodeID    uint   `json:"node_id,omitempty"`
	Latency   int64  `json:"latency,omitempty"`
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
	Testing   bool   `json:"testing"`
}

// ProgressBus 是线程安全的进度发布订阅总线。
// 支持多订阅者，缓存最后状态，终端状态关闭 channel。
type ProgressBus struct {
	mu          sync.Mutex
	subscribers map[chan OperationProgress]struct{}
	last        *OperationProgress
	running     bool
	startedAt   time.Time
	finishedAt  time.Time
}

func NewProgressBus() *ProgressBus {
	return &ProgressBus{subscribers: make(map[chan OperationProgress]struct{})}
}

// Subscribe 注册订阅者，返回 channel 和取消订阅函数。
// 如果已有最后状态，会立即发送一次。
func (b *ProgressBus) Subscribe() (<-chan OperationProgress, func()) {
	ch := make(chan OperationProgress, 100)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	last := b.last
	b.mu.Unlock()

	if last != nil {
		ch <- *last
	}

	return ch, func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
	}
}

// Publish 向所有订阅者推送进度。terminal=true 时关闭所有 channel 并清空订阅者。
func (b *ProgressBus) Publish(p OperationProgress, terminal bool) {
	b.mu.Lock()
	copied := p
	b.last = &copied
	if terminal {
		b.running = false
		b.finishedAt = time.Now()
	}

	subscribers := make([]chan OperationProgress, 0, len(b.subscribers))
	for ch := range b.subscribers {
		subscribers = append(subscribers, ch)
	}
	if terminal {
		b.subscribers = make(map[chan OperationProgress]struct{})
	}
	b.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- p:
		default:
		}
		if terminal {
			close(ch)
		}
	}
}

// Start 标记任务开始运行。
func (b *ProgressBus) Start() {
	b.mu.Lock()
	b.running = true
	b.startedAt = time.Now()
	b.mu.Unlock()
}

// IsRunning 返回任务是否正在运行。
func (b *ProgressBus) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

// Last 返回最后推送的进度副本。
func (b *ProgressBus) Last() *OperationProgress {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.last == nil {
		return nil
	}
	copied := *b.last
	return &copied
}

// StartedAt 返回任务开始时间。
func (b *ProgressBus) StartedAt() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.startedAt
}

// FinishedAt 返回任务结束时间。
func (b *ProgressBus) FinishedAt() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.finishedAt
}