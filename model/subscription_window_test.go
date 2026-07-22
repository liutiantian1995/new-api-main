package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsWithinDailyWindow covers the pure window-judgment function with
// deterministic table-driven cases including all-day, same-day, and
// cross-midnight windows. Boundaries are inclusive of start, exclusive of end.
func TestIsWithinDailyWindow(t *testing.T) {
	cases := []struct {
		name           string
		start, end, m  int
		expected       bool
	}{
		// All-day window (0/0 or equal) → always true
		{"all-day zero at midnight", 0, 0, 0, true},
		{"all-day zero at noon", 0, 0, 720, true},
		{"all-day zero at end of day", 0, 0, 1439, true},
		{"equal non-zero treated as all-day", 600, 600, 600, true},

		// Same-day window [start, end)
		{"same-day at start (inclusive)", 0, 360, 0, true},
		{"same-day one minute before end", 0, 360, 359, true},
		{"same-day at end (exclusive)", 0, 360, 360, false},
		{"same-day before window", 660, 1380, 600, false},
		{"same-day at start 11:00", 660, 1380, 660, true},
		{"same-day at 22:59", 660, 1380, 1379, true},
		{"same-day at 23:00 (exclusive)", 660, 1380, 1380, false},

		// Cross-midnight window (start > end)
		{"cross-midnight at start 23:00", 1380, 360, 1380, true},
		{"cross-midnight at 23:59", 1380, 360, 1439, true},
		{"cross-midnight at 00:00", 1380, 360, 0, true},
		{"cross-midnight at 05:00", 1380, 360, 300, true},
		{"cross-midnight at 05:59", 1380, 360, 359, true},
		{"cross-midnight at 06:00 (exclusive)", 1380, 360, 360, false},
		{"cross-midnight at 10:00 (outside)", 1380, 360, 600, false},
		{"cross-midnight at 22:59 (outside)", 1380, 360, 1379, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsWithinDailyWindow(tc.start, tc.end, tc.m)
			assert.Equalf(t, tc.expected, got,
				"IsWithinDailyWindow(%d, %d, %d) = %v, want %v",
				tc.start, tc.end, tc.m, got, tc.expected)
		})
	}
}

// TestNormalizeDefaults_DailyWindowClampingAndEqualZero verifies that
// out-of-range values are clamped into [0, 1439] and that equal non-zero
// values collapse to 0/0 (all-day).
func TestNormalizeDefaults_DailyWindowClampingAndEqualZero(t *testing.T) {
	cases := []struct {
		name                             string
		startIn, endIn                   int
		startWant, endWant               int
	}{
		{"equal non-zero collapses to all-day", 600, 600, 0, 0},
		{"equal zero stays zero", 0, 0, 0, 0},
		{"cross-midnight preserved", 1380, 360, 1380, 360},
		{"same-day preserved", 0, 360, 0, 360},
		{"negative start clamped to 0", -10, 360, 0, 360},
		{"over-max start clamped to 1439", 2000, 360, 1439, 360},
		{"negative end clamped to 0", 600, -5, 600, 0},
		{"over-max end clamped to 1439", 600, 5000, 600, 1439},
		// After clamping, equal non-zero values still collapse to 0/0.
		{"both clamp to same value → all-day", 2000, 2000, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &SubscriptionPlan{
				DailyActiveStartMinutes: tc.startIn,
				DailyActiveEndMinutes:   tc.endIn,
			}
			p.NormalizeDefaults()
			assert.Equal(t, tc.startWant, p.DailyActiveStartMinutes, "start mismatch")
			assert.Equal(t, tc.endWant, p.DailyActiveEndMinutes, "end mismatch")
		})
	}
}

// TestPreConsumeUserSubscription_DailyWindowSkip verifies that a subscription
// whose daily window excludes "now" is skipped, yielding the same "no active
// subscription"-style error as if the user had no subscription at all.
//
// Window values are computed relative to the current minute-of-day so the test
// is deterministic at any run time without mocking the clock.
func TestPreConsumeUserSubscription_DailyWindowSkip(t *testing.T) {
	truncateTables(t)

	// Create a fresh user.
	user := &User{
		Username:    "window_test_user",
		Password:    "$2a$10$dummyhashplaceholderdummyhashplaceholderdummyhash",
		DisplayName: "window_test_user",
		Role:        1,
		Status:      1,
	}
	require.NoError(t, DB.Create(user).Error)

	// A plan with abundant quota so the only filter is the daily window.
	planAllDay := &SubscriptionPlan{
		Title:                    "Window Test Plan",
		PriceAmount:              1.0,
		Currency:                 "USD",
		DurationUnit:             SubscriptionDurationDay,
		DurationValue:            1,
		Enabled:                  true,
		TotalAmount:              1_000_000,
		QuotaResetPeriod:         SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(planAllDay).Error)

	now := int64(0) // populated below via GetDBTimestamp at creation time
	_ = now

	// Window that excludes the current minute: a 1-minute window sitting
	// exactly one minute ahead of now. We build it defensively so that even
	// if now == 1439 we wrap correctly.
	mNow := currentMinuteOfDay()
	// outsideStart/outsideEnd define a window that does NOT contain mNow.
	// Pick a window starting at (mNow + 5) mod 1440 with width 2 minutes.
	outsideStart := (mNow + 5) % 1440
	outsideEnd := (outsideStart + 2) % 1440
	// Inside window: a 60-minute window centered on mNow. We make it wide
	// enough that the test does not flake near minute boundaries.
	insideStart := (mNow + 1440 - 5) % 1440 // 5 min before now, wrapping
	insideEnd := (mNow + 5) % 1440          // 5 min after now, wrapping

	// --- Sub-test 1: window excludes "now" → expect skip → error.
	subOutside := &UserSubscription{
		UserId:                  user.Id,
		PlanId:                  planAllDay.Id,
		AmountTotal:             1_000_000,
		AmountUsed:              0,
		StartTime:               GetDBTimestamp() - 60,
		EndTime:                 GetDBTimestamp() + 86400,
		Status:                  "active",
		Source:                  "admin",
		DailyActiveStartMinutes: outsideStart,
		DailyActiveEndMinutes:   outsideEnd,
	}
	require.NoError(t, DB.Create(subOutside).Error, "create outside-window sub")

	res, err := PreConsumeUserSubscription("req-window-skip-outside", user.Id, "any-model", 0, 10)
	require.Error(t, err, "expected error when subscription is outside daily window")
	assert.Nil(t, res, "result must be nil when skipped")
	// The candidate exists but is skipped by the window filter, so the loop
	// falls through to the EXISTING "quota insufficient" path — no new error
	// code is introduced. Caller (funding source chain) treats both the
	// "no active subscription" and "quota insufficient" messages as
	// "no usable subscription → fall back to wallet".
	assert.Contains(t, err.Error(), "subscription quota insufficient",
		"should reuse existing no-usable-subscription error path: %v", err)

	// Clean up to isolate the inside-window sub-test.
	require.NoError(t, DB.Unscoped().Delete(subOutside).Error)

	// --- Sub-test 2: same subscription, but with a window that includes "now"
	// → expect successful selection and pre-consume.
	subInside := &UserSubscription{
		UserId:                  user.Id,
		PlanId:                  planAllDay.Id,
		AmountTotal:             1_000_000,
		AmountUsed:              0,
		StartTime:               GetDBTimestamp() - 60,
		EndTime:                 GetDBTimestamp() + 86400,
		Status:                  "active",
		Source:                  "admin",
		DailyActiveStartMinutes: insideStart,
		DailyActiveEndMinutes:   insideEnd,
	}
	require.NoError(t, DB.Create(subInside).Error, "create inside-window sub")

	res2, err2 := PreConsumeUserSubscription("req-window-skip-inside", user.Id, "any-model", 0, 10)
	require.NoError(t, err2, "expected success when inside daily window: %v", err2)
	require.NotNil(t, res2, "result must not be nil when window allows")
	assert.Equal(t, subInside.Id, res2.UserSubscriptionId, "should select the inside-window subscription")
	assert.Equal(t, int64(10), res2.PreConsumed, "pre-consume amount mismatch")
}

// TestHasActiveUserSubscription_DailyWindow ensures the existence pre-check
// respects the daily window: an otherwise-active subscription outside its
// window must NOT be counted as "available".
func TestHasActiveUserSubscription_DailyWindow(t *testing.T) {
	truncateTables(t)

	user := &User{
		Username:    "has_active_window_user",
		Password:    "$2a$10$dummyhashplaceholderdummyhashplaceholderdummyhash",
		DisplayName: "has_active_window_user",
		Role:        1,
		Status:      1,
	}
	require.NoError(t, DB.Create(user).Error)

	// With no subscriptions → false.
	got, err := HasActiveUserSubscription(user.Id)
	require.NoError(t, err)
	assert.False(t, got, "expected false with no subscriptions")

	mNow := currentMinuteOfDay()
	outsideStart := (mNow + 5) % 1440
	outsideEnd := (outsideStart + 2) % 1440

	subOutside := &UserSubscription{
		UserId:                  user.Id,
		AmountTotal:             1_000_000,
		AmountUsed:              0,
		StartTime:               GetDBTimestamp() - 60,
		EndTime:                 GetDBTimestamp() + 86400,
		Status:                  "active",
		Source:                  "admin",
		DailyActiveStartMinutes: outsideStart,
		DailyActiveEndMinutes:   outsideEnd,
	}
	require.NoError(t, DB.Create(subOutside).Error)

	// Active but outside window → still false.
	got, err = HasActiveUserSubscription(user.Id)
	require.NoError(t, err)
	assert.False(t, got, "active-but-outside-window subscription must NOT count as available")

	// Add a same-day wide window subscription that includes now.
	insideStart := (mNow + 1440 - 5) % 1440
	insideEnd := (mNow + 5) % 1440
	subInside := &UserSubscription{
		UserId:                  user.Id,
		AmountTotal:             1_000_000,
		AmountUsed:              0,
		StartTime:               GetDBTimestamp() - 60,
		EndTime:                 GetDBTimestamp() + 86400,
		Status:                  "active",
		Source:                  "admin",
		DailyActiveStartMinutes: insideStart,
		DailyActiveEndMinutes:   insideEnd,
	}
	require.NoError(t, DB.Create(subInside).Error)

	// At least one in-window subscription → true.
	got, err = HasActiveUserSubscription(user.Id)
	require.NoError(t, err)
	assert.True(t, got, "expected true when at least one subscription is in-window")

	// Sanity log for debugging on CI:
	t.Logf("mNow=%d outsideStart=%d outsideEnd=%d insideStart=%d insideEnd=%d",
		mNow, outsideStart, outsideEnd, insideStart, insideEnd)
	_ = fmt.Sprintf // keep fmt import meaningful in case of future verbose logs
}

// TestUserActiveSubscriptionsAllowWalletOverflow_DailyWindow verifies that a
// strict (allow_wallet_overflow=false) subscription blocks wallet fallback
// ONLY while it is inside its daily window. Outside the window it must be
// treated as invisible so the user can still fall back to wallet balance.
//
// This is the regression test for the bug where UserActiveSubscriptionsAllowWalletOverflow
// ignored the daily window: a user holding a strict windowed subscription would be
// completely rejected (neither subscription nor wallet) during the window-outside period.
func TestUserActiveSubscriptionsAllowWalletOverflow_DailyWindow(t *testing.T) {
	truncateTables(t)

	user := &User{
		Username:    "overflow_window_user",
		Password:    "$2a$10$dummyhashplaceholderdummyhashplaceholderdummyhash",
		DisplayName: "overflow_window_user",
		Role:        1,
		Status:      1,
	}
	require.NoError(t, DB.Create(user).Error)

	// With no subscriptions → wallet allowed.
	got, err := UserActiveSubscriptionsAllowWalletOverflow(user.Id)
	require.NoError(t, err)
	assert.True(t, got, "no subscriptions → wallet fallback allowed")

	mNow := currentMinuteOfDay()
	outsideStart := (mNow + 5) % 1440
	outsideEnd := (outsideStart + 2) % 1440
	insideStart := (mNow + 1440 - 5) % 1440
	insideEnd := (mNow + 5) % 1440

	// Strict subscription OUTSIDE its window → wallet MUST still be allowed.
	subOutside := &UserSubscription{
		UserId:                  user.Id,
		AmountTotal:             1_000_000,
		AmountUsed:              0,
		StartTime:               GetDBTimestamp() - 60,
		EndTime:                 GetDBTimestamp() + 86400,
		Status:                  "active",
		Source:                  "admin",
		AllowWalletOverflow:     false, // strict
		DailyActiveStartMinutes: outsideStart,
		DailyActiveEndMinutes:   outsideEnd,
	}
	require.NoError(t, DB.Create(subOutside).Error)

	got, err = UserActiveSubscriptionsAllowWalletOverflow(user.Id)
	require.NoError(t, err)
	assert.True(t, got,
		"strict subscription outside its window MUST NOT block wallet fallback")

	// Add a strict subscription INSIDE its window → wallet MUST be blocked.
	subInside := &UserSubscription{
		UserId:                  user.Id,
		AmountTotal:             1_000_000,
		AmountUsed:              0,
		StartTime:               GetDBTimestamp() - 60,
		EndTime:                 GetDBTimestamp() + 86400,
		Status:                  "active",
		Source:                  "admin",
		AllowWalletOverflow:     false, // strict
		DailyActiveStartMinutes: insideStart,
		DailyActiveEndMinutes:   insideEnd,
	}
	require.NoError(t, DB.Create(subInside).Error)

	got, err = UserActiveSubscriptionsAllowWalletOverflow(user.Id)
	require.NoError(t, err)
	assert.False(t, got,
		"strict subscription inside its window MUST block wallet fallback")

	// Sanity log.
	t.Logf("mNow=%d outsideStart=%d outsideEnd=%d insideStart=%d insideEnd=%d",
		mNow, outsideStart, outsideEnd, insideStart, insideEnd)
}
