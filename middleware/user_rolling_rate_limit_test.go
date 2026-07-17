package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/setting"
)

func TestResolveRollingLimits(t *testing.T) {
	// user override takes precedence
	userOverride := `[{"duration": 18000, "limit": 999}]`
	setting.UserRollingRateLimitGroup = map[string][]setting.RollingRateLimitTier{
		"default": {{Duration: 86400, Limit: 2000}},
	}
	limits := resolveRollingLimits(userOverride, "default")
	require.Len(t, limits, 1)
	assert.Equal(t, 999, limits[0].Limit)

	// empty user override falls back to group
	limits = resolveRollingLimits("", "default")
	require.Len(t, limits, 1)
	assert.Equal(t, 2000, limits[0].Limit)

	// empty user override + unknown group = no limits
	limits = resolveRollingLimits("", "nonexistent")
	assert.Empty(t, limits)
}
