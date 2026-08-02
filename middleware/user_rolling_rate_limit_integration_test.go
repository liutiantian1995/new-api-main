package middleware

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
)

// TestRollingLimitTTL verifies the TTL helper adds a 10% buffer (floored to 60s)
// on top of the configured duration, with a minimum buffer of 60 seconds.
func TestRollingLimitTTL(t *testing.T) {
	tests := []struct {
		name     string
		duration int64
		minTTL   time.Duration
		maxTTL   time.Duration
	}{
		{"5 hours", 18000, 18000 * time.Second, 20000 * time.Second},
		{"1 day", 86400, 86400 * time.Second, 96000 * time.Second},
		{"1 week", 604800, 604800 * time.Second, 665280 * time.Second},
		{"sub-minute uses 60s floor", 30, 90 * time.Second, 120 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rollingLimitTTL(tt.duration)
			assert.GreaterOrEqual(t, got, tt.minTTL, "TTL should be >= duration + min buffer")
			assert.LessOrEqual(t, got, tt.maxTTL, "TTL should be <= duration + 20%")
		})
	}
}

// TestResolveRollingLimits_UserOverrideInvalidJSON verifies that invalid JSON
// in the user override silently falls through to the group default.
func TestResolveRollingLimits_UserOverrideInvalidJSON(t *testing.T) {
	origGroups := setting.UserRollingRateLimitGroup
	defer func() { setting.UserRollingRateLimitGroup = origGroups }()

	setting.UserRollingRateLimitGroup = map[string][]setting.RollingRateLimitTier{
		"default": {{Duration: 86400, Limit: 2000}},
	}

	limits := resolveRollingLimits("not valid json", "default")
	require.Len(t, limits, 1)
	assert.Equal(t, int64(86400), limits[0].Duration)
	assert.Equal(t, 2000, limits[0].Limit)
}

// TestResolveRollingLimits_EmptyUserOverrideArray verifies that an empty
// JSON array in the user override falls through to the group default.
func TestResolveRollingLimits_EmptyUserOverrideArray(t *testing.T) {
	origGroups := setting.UserRollingRateLimitGroup
	defer func() { setting.UserRollingRateLimitGroup = origGroups }()

	setting.UserRollingRateLimitGroup = map[string][]setting.RollingRateLimitTier{
		"default": {{Duration: 86400, Limit: 2000}},
	}

	// `[]` parses as a non-nil but empty slice, which the function treats as
	// "no override" and falls through to group lookup.
	limits := resolveRollingLimits("[]", "default")
	require.Len(t, limits, 1)
	assert.Equal(t, 2000, limits[0].Limit)
}

// TestRecordRollingRequest_InMemory exercises the full in-memory record + count
// cycle through the middleware's singleton limiter. Must run with Redis disabled.
func TestRecordRollingRequest_InMemory(t *testing.T) {
	origRedis := common.RedisEnabled
	origLimiter := rollingInMemoryLimiter
	defer func() {
		common.RedisEnabled = origRedis
		rollingInMemoryLimiter = origLimiter
		// Reset the package-level Once so subsequent tests (or the production
		// code path) re-initialize a fresh limiter. We avoid copying sync.Once
		// (it contains noCopy) by simply assigning a fresh zero value here.
		rollingInMemoryLimiterOnce = sync.Once{}
	}()

	common.RedisEnabled = false
	// Reset the singleton: nil the instance and reset the Once so the next
	// call to getRollingInMemoryLimiter() creates a fresh limiter.
	rollingInMemoryLimiter = nil
	rollingInMemoryLimiterOnce = sync.Once{}

	key := "test:integration:user1:18000"

	limiter := getRollingInMemoryLimiter()
	require.NotNil(t, limiter)
	require.Equal(t, 0, limiter.Count(key))

	// Record 3 entries with max=5
	recordRollingRequest(key, 5, 18000)
	recordRollingRequest(key, 5, 18000)
	recordRollingRequest(key, 5, 18000)
	require.Equal(t, 3, limiter.Count(key))

	// Record 5 more (total 8, should trim to max=5)
	for i := 0; i < 5; i++ {
		recordRollingRequest(key, 5, 18000)
	}
	require.Equal(t, 5, limiter.Count(key), "should trim to max=5")
}

// TestCheckRollingLimit_InMemory_WindowRecovery verifies the in-memory branch
// of checkRollingLimit recovers after the rolling window passes. This is the
// regression guard for the bug where users were locked out far longer than
// the configured window because Count-based checking ignored the timestamps
// and the queue only shrank on Record (which is never called for rejected
// requests). The fix routes the in-memory path through CheckAllowed.
//
// The Redis branch already had the correct sliding-window semantics; we only
// need to cover the in-memory path here.
func TestCheckRollingLimit_InMemory_WindowRecovery(t *testing.T) {
	origRedis := common.RedisEnabled
	origLimiter := rollingInMemoryLimiter
	defer func() {
		common.RedisEnabled = origRedis
		rollingInMemoryLimiter = origLimiter
		rollingInMemoryLimiterOnce = sync.Once{}
	}()

	common.RedisEnabled = false
	rollingInMemoryLimiter = nil
	rollingInMemoryLimiterOnce = sync.Once{}

	key := "test:checklimit:recovery"
	var duration int64 = 1 // 1 second; smallest valid window

	// Fill to max=3 with successful records.
	recordRollingRequest(key, 3, duration)
	recordRollingRequest(key, 3, duration)
	recordRollingRequest(key, 3, duration)

	// Inside window → reject. checkRollingLimit's in-memory branch does not
	// touch the gin.Context, so nil is safe here.
	require.False(t, checkRollingLimit(nil, key, 3, duration), "should reject at max within window")

	// Wait past the rolling window. No Record happens in between, mimicking
	// a user whose requests keep getting rejected.
	time.Sleep(1100 * time.Millisecond)

	// After the window passes → allow. Before the fix this stayed false
	// until the 7-day cleanup goroutine deleted the key.
	require.True(t, checkRollingLimit(nil, key, 3, duration), "should allow after window expired")
}
