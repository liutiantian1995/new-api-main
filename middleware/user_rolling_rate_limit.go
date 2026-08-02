package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

// rollingInMemoryLimiter is the process-wide singleton for the in-memory
// fallback path. Initialized lazily on first use via sync.Once to avoid
// goroutine leaks from concurrent first-burst initializations.
var (
	rollingInMemoryLimiter     *common.RollingInMemoryRateLimiter
	rollingInMemoryLimiterOnce sync.Once
)

// getRollingInMemoryLimiter returns the process-wide singleton, initializing
// it exactly once on first call.
func getRollingInMemoryLimiter() *common.RollingInMemoryRateLimiter {
	rollingInMemoryLimiterOnce.Do(func() {
		rollingInMemoryLimiter = common.NewRollingInMemoryRateLimiter(7 * 24 * time.Hour)
	})
	return rollingInMemoryLimiter
}

// rollingLimit is the middleware-local view of setting.RollingRateLimitTier.
// We use a copy type so resolveRollingLimits does not depend on the setting
// package's internal layout in tests.
type rollingLimit struct {
	Duration int64
	Limit    int
}

// UserRollingRateLimit checks per-user rolling-window quotas (5h / 1d / 1w, etc.)
// and rejects with HTTP 429 if any tier is exceeded. Records on success only.
// Must be mounted AFTER TokenAuth (needs c.GetInt("id")) and AFTER the user
// context is populated (needs c.GetString("group") and optionally the user's
// rolling_rate_limit override).
func UserRollingRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !setting.UserRollingRateLimitEnabled {
			c.Next()
			return
		}

		userId := c.GetInt("id")
		if userId == 0 {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		// Resolve effective tiers
		userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		userOverride := common.GetContextKeyString(c, constant.ContextKeyUserRollingRateLimit)
		limits := resolveRollingLimits(userOverride, userGroup)

		if len(limits) == 0 {
			c.Next()
			return
		}

		// Pre-check all tiers (read-only)
		for i := range limits {
			tier := &limits[i]
			key := fmt.Sprintf("rolling_limit:%d:%d", userId, tier.Duration)
			if !checkRollingLimit(c, key, tier.Limit, tier.Duration) {
				msg := fmt.Sprintf("已达到 %s 内最大请求数 %d，请稍后重试",
					common.FormatRollingDuration(tier.Duration), tier.Limit)
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, msg)
				return
			}
		}

		c.Next()

		// Record only on success
		if c.Writer.Status() < 400 {
			for i := range limits {
				tier := &limits[i]
				key := fmt.Sprintf("rolling_limit:%d:%d", userId, tier.Duration)
				recordRollingRequest(key, tier.Limit, tier.Duration)
			}
		}
	}
}

// resolveRollingLimits picks the effective tier list for a request.
// userOverride (non-empty) wins; otherwise falls back to group default via
// the setting package's RLock-protected helper.
// Returns nil if neither is configured.
func resolveRollingLimits(userOverride, group string) []rollingLimit {
	if userOverride != "" {
		var tiers []setting.RollingRateLimitTier
		if err := common.Unmarshal([]byte(userOverride), &tiers); err == nil && len(tiers) > 0 {
			out := make([]rollingLimit, len(tiers))
			for i := range tiers {
				out[i] = rollingLimit{Duration: tiers[i].Duration, Limit: tiers[i].Limit}
			}
			return out
		}
		// fall through on parse error or empty array
	}
	if tiers, found := setting.GetGroupRollingRateLimit(group); found && len(tiers) > 0 {
		out := make([]rollingLimit, len(tiers))
		for i := range tiers {
			out[i] = rollingLimit{Duration: tiers[i].Duration, Limit: tiers[i].Limit}
		}
		return out
	}
	return nil
}

func rollingLimitTTL(duration int64) time.Duration {
	buffer := duration / 10
	if buffer < 60 {
		buffer = 60
	}
	return time.Duration(duration+buffer) * time.Second
}

func checkRollingLimit(c *gin.Context, key string, maxCount int, duration int64) bool {
	if !common.RedisEnabled {
		// In-memory fallback: enforce sliding-window semantics so a user
		// recovers automatically once the oldest recorded request leaves
		// the window, mirroring the Redis path below.
		//
		// Previously this branch called Count(key) < maxCount, which only
		// looks at the raw queue length. Because rejected requests never
		// reach recordRollingRequest, the queue was never trimmed and the
		// user stayed over the limit until the 7-day cleanup goroutine
		// evicted the whole key — far longer than the configured window.
		limiter := getRollingInMemoryLimiter()
		return limiter.CheckAllowed(key, maxCount, duration)
	}
	ctx := context.Background()
	rdb := common.RDB
	listLength, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		// Fail-open on Redis errors: a transient outage should not block
		// legitimate traffic. Log so operators can diagnose.
		common.SysLog("rolling rate limit: LLen error, allowing request: " + err.Error())
		return true
	}
	if listLength < int64(maxCount) {
		return true
	}
	oldTimeStr, err := rdb.LIndex(ctx, key, -1).Result()
	if err != nil {
		// redis.Nil (list shrank between LLen and LIndex) or transient error:
		// allow the request rather than 429 on stale state.
		common.SysLog("rolling rate limit: LIndex error, allowing request: " + err.Error())
		return true
	}
	oldTime, err := time.Parse(timeFormat, oldTimeStr)
	if err != nil {
		// Unparseable timestamp (corrupted entry): allow rather than 429.
		common.SysLog("rolling rate limit: bad timestamp in list, allowing request")
		return true
	}
	nowTimeStr := time.Now().Format(timeFormat)
	nowTime, err := time.Parse(timeFormat, nowTimeStr)
	if err != nil {
		// Practically unreachable (we just formatted the time), but be safe.
		return true
	}
	if int64(nowTime.Sub(oldTime).Seconds()) < duration {
		rdb.Expire(ctx, key, rollingLimitTTL(duration))
		return false
	}
	return true
}

func recordRollingRequest(key string, maxCount int, duration int64) {
	if common.RedisEnabled {
		ctx := context.Background()
		rdb := common.RDB
		now := time.Now().Format(timeFormat)
		rdb.LPush(ctx, key, now)
		rdb.LTrim(ctx, key, 0, int64(maxCount-1))
		rdb.Expire(ctx, key, rollingLimitTTL(duration))
		return
	}
	limiter := getRollingInMemoryLimiter()
	limiter.Record(key, maxCount, duration)
}