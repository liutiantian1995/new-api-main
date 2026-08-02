package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRollingInMemoryRateLimiter_Count(t *testing.T) {
	rl := NewRollingInMemoryRateLimiter(24 * time.Hour)
	key := "test:count"
	assert.Equal(t, 0, rl.Count(key))
	rl.Record(key, 100, 18000)
	assert.Equal(t, 1, rl.Count(key))
}

func TestRollingInMemoryRateLimiter_Record(t *testing.T) {
	rl := NewRollingInMemoryRateLimiter(24 * time.Hour)
	key := "test:record"
	// Record more than max — should trim
	for i := 0; i < 10; i++ {
		rl.Record(key, 3, 18000)
	}
	assert.Equal(t, 3, rl.Count(key))
}

func TestRollingInMemoryRateLimiter_Expiration(t *testing.T) {
	// Short expiration for testing
	rl := NewRollingInMemoryRateLimiter(100 * time.Millisecond)
	key := "test:expire"
	rl.Record(key, 100, 18000)
	assert.Equal(t, 1, rl.Count(key))
	// After cleanup runs, key should be evicted
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, 0, rl.Count(key))
}

// TestRollingInMemoryRateLimiter_CheckAllowed_UnknownKey verifies that a
// never-recorded key is allowed (treated as zero usage).
func TestRollingInMemoryRateLimiter_CheckAllowed_UnknownKey(t *testing.T) {
	rl := NewRollingInMemoryRateLimiter(24 * time.Hour)
	assert.True(t, rl.CheckAllowed("nonexistent", 5, 1000), "unknown key should be allowed")
}

// TestRollingInMemoryRateLimiter_CheckAllowed_BelowMax verifies that a queue
// shorter than max is allowed regardless of timestamp age.
func TestRollingInMemoryRateLimiter_CheckAllowed_BelowMax(t *testing.T) {
	rl := NewRollingInMemoryRateLimiter(24 * time.Hour)
	key := "test:checkallowed:belowmax"
	var duration int64 = 5000 // 5s, longer than test runtime

	rl.Record(key, 3, duration)
	rl.Record(key, 3, duration)

	assert.True(t, rl.CheckAllowed(key, 3, duration), "below max should be allowed")
}

// TestRollingInMemoryRateLimiter_CheckAllowed_AtMaxWithinWindow verifies that
// a full queue whose oldest timestamp is still inside the window rejects.
func TestRollingInMemoryRateLimiter_CheckAllowed_AtMaxWithinWindow(t *testing.T) {
	rl := NewRollingInMemoryRateLimiter(24 * time.Hour)
	key := "test:checkallowed:within"
	var duration int64 = 5000 // 5s, longer than test runtime

	rl.Record(key, 3, duration)
	rl.Record(key, 3, duration)
	rl.Record(key, 3, duration)
	require.Equal(t, 3, rl.Count(key))

	assert.False(t, rl.CheckAllowed(key, 3, duration), "at max within window should be rejected")
}

// TestRollingInMemoryRateLimiter_CheckAllowed_AtMaxWindowExpired is the
// regression test for the bug where users were permanently rejected after
// hitting the limit. After the rolling window passes, a full queue must
// allow new requests because the oldest timestamp has left the window.
//
// Previously CheckAllowed did not exist and the middleware called Count,
// which only returns the raw queue length. Since rejected requests never
// invoke Record (which trims expired entries), the queue stayed at max
// forever and the user could not recover until the 7-day cleanup goroutine
// deleted the entire key.
func TestRollingInMemoryRateLimiter_CheckAllowed_AtMaxWindowExpired(t *testing.T) {
	rl := NewRollingInMemoryRateLimiter(24 * time.Hour)
	key := "test:checkallowed:expired"
	// duration is in seconds (matches the protocol used by Record and the
	// middleware tier config). 1s is the smallest valid window and lets the
	// test exercise the expiration path without a long sleep.
	var duration int64 = 1

	rl.Record(key, 3, duration)
	rl.Record(key, 3, duration)
	rl.Record(key, 3, duration)
	require.Equal(t, 3, rl.Count(key))

	// Sanity: still inside window right after recording.
	assert.False(t, rl.CheckAllowed(key, 3, duration))

	// Wait past the rolling window. No Record happens in the meantime
	// (mimicking a user whose requests are all rejected).
	time.Sleep(1100 * time.Millisecond)

	// Expected (fixed): oldest timestamp has left the window → allow.
	// Buggy (before CheckAllowed existed): Count returns 3 → 3 < 3 is false → reject forever.
	assert.True(t, rl.CheckAllowed(key, 3, duration), "should allow after window expired even when queue is full")
}
