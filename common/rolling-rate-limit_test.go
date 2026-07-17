package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
