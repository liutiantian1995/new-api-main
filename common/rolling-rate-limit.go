package common

import (
	"sync"
	"time"
)

// RollingInMemoryRateLimiter implements a sliding-window log in memory.
// It is intentionally separate from InMemoryRateLimiter so the cleanup
// cadence can be tuned for long rolling windows (hours/days/weeks) instead
// of the existing minute-level limiter.
//
// Timestamps are stored as int64 UnixNano values for sub-second precision
// (important so tests can exercise the expiration path without multi-second
// sleeps, and so rolling-window boundaries are not off-by-one second in
// production).
type RollingInMemoryRateLimiter struct {
	store              map[string]*[]int64
	mutex              sync.Mutex
	expirationDuration time.Duration
}

func NewRollingInMemoryRateLimiter(expiration time.Duration) *RollingInMemoryRateLimiter {
	rl := &RollingInMemoryRateLimiter{
		store:              make(map[string]*[]int64),
		expirationDuration: expiration,
	}
	if expiration > 0 {
		go rl.clearExpiredItems()
	}
	return rl
}

func (l *RollingInMemoryRateLimiter) clearExpiredItems() {
	for {
		time.Sleep(l.expirationDuration)
		l.mutex.Lock()
		cutoff := time.Now().Add(-l.expirationDuration).UnixNano()
		for key, queue := range l.store {
			size := len(*queue)
			if size == 0 || (*queue)[size-1] < cutoff {
				delete(l.store, key)
			}
		}
		l.mutex.Unlock()
	}
}

// Count returns the number of recorded timestamps currently in the window for key.
// Note: this is the raw stored count; callers may also want to trim by duration
// before deciding, but Record already trims on each write.
func (l *RollingInMemoryRateLimiter) Count(key string) int {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	queue, ok := l.store[key]
	if !ok {
		return 0
	}
	return len(*queue)
}

// Record appends a new timestamp and trims the window to:
//   - entries within the last `duration` seconds (rolling window)
//   - at most `max` entries (cap for memory bound)
func (l *RollingInMemoryRateLimiter) Record(key string, max int, duration int64) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	now := time.Now().UnixNano()
	cutoff := now - duration*int64(time.Second)
	queue, ok := l.store[key]
	if !ok {
		s := make([]int64, 0, max)
		l.store[key] = &s
		queue = l.store[key]
	}
	*queue = append(*queue, now)
	// Trim entries older than the rolling window
	start := 0
	for start < len(*queue) && (*queue)[start] < cutoff {
		start++
	}
	if start > 0 {
		*queue = (*queue)[start:]
	}
	// Trim to max entries (keep newest)
	if len(*queue) > max {
		*queue = (*queue)[len(*queue)-max:]
	}
}
