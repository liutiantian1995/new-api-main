package common

import "fmt"

// FormatRollingDuration converts a duration in seconds to a human-readable
// string suitable for user-facing 429 messages. Mirrors on the frontend
// via rolling-rate-limit-types.ts formatDuration.
func FormatRollingDuration(sec int64) string {
	if sec <= 0 {
		return ""
	}
	const (
		hour = 3600
		day  = 86400
		week = 604800
	)
	switch {
	case sec == week:
		return "1 周"
	case sec == day:
		return "1 天"
	case sec%day == 0:
		return fmt.Sprintf("%d 天", sec/day)
	case sec%hour == 0:
		return fmt.Sprintf("%d 小时", sec/hour)
	default:
		hours := (sec + hour - 1) / hour // round up
		if hours <= 0 {
			hours = 1
		}
		return fmt.Sprintf("%d 小时", hours)
	}
}
