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
