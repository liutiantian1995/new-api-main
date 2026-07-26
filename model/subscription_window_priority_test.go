package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPreConsumeUserSubscription_WindowedPriority covers the two-pass scan
// introduced by the prefer-windowed-subscription change. When a user holds
// both a daily-windowed subscription (active for the current minute) and an
// all-day subscription, the windowed one MUST be consumed first regardless
// of end_time ordering. Outside the window, behavior falls back to the
// existing end_time-asc single-loop semantics.
//
// All windows are constructed relative to currentMinuteOfDay() so the test
// is deterministic at any wall-clock time without mocking the clock.
func TestPreConsumeUserSubscription_WindowedPriority(t *testing.T) {
	// Helper: build a wide window centered on the current minute that
	// safely contains "now" without flaking near minute boundaries.
	// Returns (start, end) in minute-of-day units; (mNow-5, mNow+5)
	// wrapped modulo 1440. The 10-minute width is enough to absorb
	// sub-minute drift between currentMinuteOfDay() calls within a
	// single test.
	makeInsideWindow := func() (int, int) {
		mNow := currentMinuteOfDay()
		start := (mNow + 1440 - 5) % 1440
		end := (mNow + 5) % 1440
		return start, end
	}
	// Helper: build a narrow window sitting strictly AFTER the current
	// minute so "now" is outside it. Used to simulate window-outside
	// scenarios deterministically.
	makeOutsideWindow := func() (int, int) {
		mNow := currentMinuteOfDay()
		start := (mNow + 10) % 1440
		end := (mNow + 20) % 1440
		return start, end
	}

	// Helper: create a user + plan + N subscriptions in one shot. Each
	// subscription is configured by the caller. Returns the created
	// subscription IDs in the same order as the input specs, plus the
	// user id for the PreConsume call.
	type subSpec struct {
		startMin int
		endMin   int
		amountTotal int64
		amountUsed  int64
		endTimeDelta int64 // seconds from now; default +86400 (1 day)
	}

	build := func(t *testing.T, specs ...subSpec) (userId int, subIds []int) {
		truncateTables(t)
		user := &User{
			Username:    "window_priority_user",
			Password:    "$2a$10$dummyhashplaceholderdummyhashplaceholderdummyhash",
			DisplayName: "window_priority_user",
			Role:        1,
			Status:      1,
		}
		require.NoError(t, DB.Create(user).Error)

		plan := &SubscriptionPlan{
			Title:            "Window Priority Plan",
			PriceAmount:      1.0,
			Currency:         "USD",
			DurationUnit:     SubscriptionDurationDay,
			DurationValue:    1,
			Enabled:          true,
			TotalAmount:      1_000_000,
			QuotaResetPeriod: SubscriptionResetNever,
		}
		require.NoError(t, DB.Create(plan).Error)

		now := GetDBTimestamp()
		for i, s := range specs {
			endDelta := s.endTimeDelta
			if endDelta == 0 {
				endDelta = 86400
			}
			sub := &UserSubscription{
				UserId:                  user.Id,
				PlanId:                  plan.Id,
				AmountTotal:             s.amountTotal,
				AmountUsed:              s.amountUsed,
				StartTime:               now - 60,
				EndTime:                 now + endDelta,
				Status:                  "active",
				Source:                  "admin",
				DailyActiveStartMinutes: s.startMin,
				DailyActiveEndMinutes:   s.endMin,
			}
			require.NoErrorf(t, DB.Create(sub).Error, "create sub #%d", i)
			subIds = append(subIds, sub.Id)
		}
		return user.Id, subIds
	}

	// Sub-test 1: real-windowed sub + all-day sub, all-day end_time EARLIER.
	// Expect: windowed sub is consumed (first pass wins).
	t.Run("windowed sub wins over earlier-ending all-day sub", func(t *testing.T) {
		wStart, wEnd := makeInsideWindow()
		// all-day sub: end_time +1h (earlier than windowed sub's +24h)
		userId, ids := build(t,
			subSpec{startMin: wStart, endMin: wEnd, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 86400},
			subSpec{startMin: 0, endMin: 0, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 3600},
		)
		windowedId := ids[0]
		_ = ids[1] // all-day, should not be touched

		res, err := PreConsumeUserSubscription("req-prio-1", userId, "any-model", 0, 10)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, windowedId, res.UserSubscriptionId,
			"windowed sub MUST be consumed first even when all-day sub ends earlier")
		assert.Equal(t, int64(10), res.PreConsumed)
	})

	// Sub-test 2: real-windowed sub + all-day sub, windowed end_time EARLIER.
	// Expect: windowed sub is consumed (both passes agree, behavior unchanged).
	t.Run("windowed sub with earlier end_time still wins", func(t *testing.T) {
		wStart, wEnd := makeInsideWindow()
		userId, ids := build(t,
			subSpec{startMin: wStart, endMin: wEnd, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 3600},
			subSpec{startMin: 0, endMin: 0, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 86400},
		)
		windowedId := ids[0]

		res, err := PreConsumeUserSubscription("req-prio-2", userId, "any-model", 0, 10)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, windowedId, res.UserSubscriptionId,
			"windowed sub with earlier end_time MUST be consumed (matches both old and new behavior)")
	})

	// Sub-test 3: real-windowed sub OUTSIDE window + all-day sub.
	// Expect: all-day sub is consumed (second pass), behavior unchanged.
	t.Run("windowed sub outside window falls back to all-day", func(t *testing.T) {
		outStart, outEnd := makeOutsideWindow()
		userId, ids := build(t,
			subSpec{startMin: outStart, endMin: outEnd, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 86400},
			subSpec{startMin: 0, endMin: 0, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 86400 * 7},
		)
		windowedId := ids[0]
		allDayId := ids[1]
		_ = windowedId

		res, err := PreConsumeUserSubscription("req-prio-3", userId, "any-model", 0, 10)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, allDayId, res.UserSubscriptionId,
			"windowed sub outside window MUST defer to all-day sub (behavior unchanged)")
	})

	// Sub-test 4: real-windowed sub in-window but exhausted + all-day sub.
	// Expect: first pass skips windowed (quota), second pass consumes all-day.
	t.Run("windowed sub exhausted falls back to all-day", func(t *testing.T) {
		wStart, wEnd := makeInsideWindow()
		userId, ids := build(t,
			subSpec{startMin: wStart, endMin: wEnd, amountTotal: 100, amountUsed: 100, endTimeDelta: 86400},
			subSpec{startMin: 0, endMin: 0, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 86400},
		)
		allDayId := ids[1]

		res, err := PreConsumeUserSubscription("req-prio-4", userId, "any-model", 0, 10)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, allDayId, res.UserSubscriptionId,
			"windowed sub with exhausted quota MUST fall back to all-day sub")
	})

	// Sub-test 5: two real-windowed subs both in-window → end_time asc decides.
	t.Run("two in-window subs resolve by end_time asc", func(t *testing.T) {
		wStart, wEnd := makeInsideWindow()
		// Two identical windows; the first has earlier end_time.
		userId, ids := build(t,
			subSpec{startMin: wStart, endMin: wEnd, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 3600},
			subSpec{startMin: wStart, endMin: wEnd, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 86400},
		)
		earlierEndId := ids[0]

		res, err := PreConsumeUserSubscription("req-prio-5", userId, "any-model", 0, 10)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, earlierEndId, res.UserSubscriptionId,
			"two in-window subs MUST resolve by end_time asc (no extra priority rules)")
	})

	// Sub-test 6: only all-day subscription → behavior fully unchanged.
	t.Run("only all-day sub behaves unchanged", func(t *testing.T) {
		userId, ids := build(t,
			subSpec{startMin: 0, endMin: 0, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 86400},
		)
		allDayId := ids[0]

		res, err := PreConsumeUserSubscription("req-prio-6", userId, "any-model", 0, 10)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, allDayId, res.UserSubscriptionId,
			"only-all-day case MUST be unchanged")
	})

	// Sub-test 7: only windowed sub, outside window → "no active subscription".
	t.Run("only windowed sub outside window returns error", func(t *testing.T) {
		outStart, outEnd := makeOutsideWindow()
		userId, _ := build(t,
			subSpec{startMin: outStart, endMin: outEnd, amountTotal: 1_000_000, amountUsed: 0, endTimeDelta: 86400},
		)

		res, err := PreConsumeUserSubscription("req-prio-7", userId, "any-model", 0, 10)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "subscription quota insufficient",
			"only-outside-window MUST reuse existing error path: %v", err)
	})

	// Sub-test 8: all subscriptions exhausted → "quota insufficient".
	t.Run("all subs exhausted returns quota insufficient", func(t *testing.T) {
		wStart, wEnd := makeInsideWindow()
		userId, _ := build(t,
			subSpec{startMin: wStart, endMin: wEnd, amountTotal: 100, amountUsed: 100, endTimeDelta: 86400},
			subSpec{startMin: 0, endMin: 0, amountTotal: 100, amountUsed: 100, endTimeDelta: 86400},
		)

		res, err := PreConsumeUserSubscription("req-prio-8", userId, "any-model", 0, 10)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "subscription quota insufficient",
			"all-exhausted MUST return existing quota-insufficient error: %v", err)
	})
}
