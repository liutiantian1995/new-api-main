package setting

import (
	"fmt"
	"math"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const (
	MaxRollingRateTiers    = 5
	MinRollingRateDuration = 60 // seconds
)

// RollingRateLimitTier describes a single rolling-window quota.
// Duration is the window length in seconds; Limit is the maximum number of
// successful requests within any rolling window of that length.
type RollingRateLimitTier struct {
	Duration int64 `json:"duration"`
	Limit    int   `json:"limit"`
}

var (
	UserRollingRateLimitEnabled = false
	UserRollingRateLimitGroup   = make(map[string][]RollingRateLimitTier)
	rollingRateLimitMutex       sync.RWMutex
)

func UserRollingRateLimitGroup2JSONString() string {
	rollingRateLimitMutex.RLock()
	defer rollingRateLimitMutex.RUnlock()
	jsonBytes, err := common.Marshal(UserRollingRateLimitGroup)
	if err != nil {
		common.SysLog("error marshalling rolling rate limit group: " + err.Error())
		return "{}"
	}
	return string(jsonBytes)
}

func UpdateRollingRateLimitGroupByJSONString(jsonStr string) error {
	rollingRateLimitMutex.Lock()
	defer rollingRateLimitMutex.Unlock()
	newGroup := make(map[string][]RollingRateLimitTier)
	if err := common.Unmarshal([]byte(jsonStr), &newGroup); err != nil {
		return err
	}
	UserRollingRateLimitGroup = newGroup
	return nil
}

func CheckRollingRateLimitGroup(jsonStr string) error {
	check := make(map[string][]RollingRateLimitTier)
	if err := common.Unmarshal([]byte(jsonStr), &check); err != nil {
		return err
	}
	for group, tiers := range check {
		if len(tiers) > MaxRollingRateTiers {
			return fmt.Errorf("group %s exceeds max tier count %d: got %d", group, MaxRollingRateTiers, len(tiers))
		}
		seen := make(map[int64]bool)
		for _, tier := range tiers {
			if tier.Duration < MinRollingRateDuration {
				return fmt.Errorf("group %s tier duration %d below minimum %d", group, tier.Duration, MinRollingRateDuration)
			}
			if tier.Duration > math.MaxInt32 {
				return fmt.Errorf("group %s tier duration %d overflow", group, tier.Duration)
			}
			if tier.Limit < 1 {
				return fmt.Errorf("group %s tier limit %d below minimum 1", group, tier.Limit)
			}
			if tier.Limit > math.MaxInt32 {
				return fmt.Errorf("group %s tier limit %d overflow", group, tier.Limit)
			}
			if seen[tier.Duration] {
				return fmt.Errorf("group %s has duplicate duration %d", group, tier.Duration)
			}
			seen[tier.Duration] = true
		}
	}
	return nil
}

// GetGroupRollingRateLimit returns the tier list for a group and whether the
// group was configured. Missing group returns found = false.
func GetGroupRollingRateLimit(group string) ([]RollingRateLimitTier, bool) {
	rollingRateLimitMutex.RLock()
	defer rollingRateLimitMutex.RUnlock()
	tiers, found := UserRollingRateLimitGroup[group]
	if !found {
		return nil, false
	}
	return tiers, true
}
