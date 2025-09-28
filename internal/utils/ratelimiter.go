package utils

import (
	"sync"
	"time"
)

type rateEntry struct {
	count int
	reset time.Time
}

// RateLimiter 提供基于固定时间窗口的简单限流。
type RateLimiter struct {
	limit  int
	window time.Duration
	mu     sync.Mutex
	store  map[string]*rateEntry
}

// NewRateLimiter 创建一个新的限流器。当 limit <= 0 或 window <= 0 时返回 nil，表示不启用限流。
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 || window <= 0 {
		return nil
	}
	return &RateLimiter{
		limit:  limit,
		window: window,
		store:  make(map[string]*rateEntry),
	}
}

// Allow 根据 key 判断是否允许继续请求。
func (r *RateLimiter) Allow(key string) bool {
	if r == nil {
		return true
	}
	if key == "" {
		key = "anonymous"
	}

	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.store[key]
	if !ok || now.After(entry.reset) {
		entry = &rateEntry{count: 0, reset: now.Add(r.window)}
		r.store[key] = entry
	}

	if entry.count >= r.limit {
		return false
	}

	entry.count++
	return true
}
