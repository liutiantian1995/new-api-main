package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRollingRateLimitGroup2JSONString(t *testing.T) {
	UserRollingRateLimitGroup = map[string][]RollingRateLimitTier{
		"default": {{Duration: 18000, Limit: 500}, {Duration: 86400, Limit: 2000}},
	}
	got := UserRollingRateLimitGroup2JSONString()
	assert.Contains(t, got, "18000")
	assert.Contains(t, got, "500")
}

func TestUpdateRollingRateLimitGroupByJSONString(t *testing.T) {
	input := `{"default": [{"duration": 18000, "limit": 500}, {"duration": 86400, "limit": 2000}]}`
	err := UpdateRollingRateLimitGroupByJSONString(input)
	require.NoError(t, err)
	require.Len(t, UserRollingRateLimitGroup, 1)
	require.Len(t, UserRollingRateLimitGroup["default"], 2)
	assert.Equal(t, int64(18000), UserRollingRateLimitGroup["default"][0].Duration)
	assert.Equal(t, 500, UserRollingRateLimitGroup["default"][0].Limit)
}

func TestUpdateRollingRateLimitGroupByJSONString_Invalid(t *testing.T) {
	err := UpdateRollingRateLimitGroupByJSONString("not json")
	assert.Error(t, err)
}

func TestCheckRollingRateLimitGroup(t *testing.T) {
	// valid
	valid := `{"default": [{"duration": 18000, "limit": 500}]}`
	require.NoError(t, CheckRollingRateLimitGroup(valid))

	// too many tiers (> 5)
	many := `{"default": [` +
		`{"duration": 60, "limit": 1},` +
		`{"duration": 120, "limit": 2},` +
		`{"duration": 180, "limit": 3},` +
		`{"duration": 240, "limit": 4},` +
		`{"duration": 300, "limit": 5},` +
		`{"duration": 360, "limit": 6}]}`
	assert.Error(t, CheckRollingRateLimitGroup(many))

	// duplicate duration
	dup := `{"default": [{"duration": 18000, "limit": 100}, {"duration": 18000, "limit": 200}]}`
	assert.Error(t, CheckRollingRateLimitGroup(dup))

	// duration < 60
	short := `{"default": [{"duration": 30, "limit": 1}]}`
	assert.Error(t, CheckRollingRateLimitGroup(short))

	// limit < 1
	zero := `{"default": [{"duration": 60, "limit": 0}]}`
	assert.Error(t, CheckRollingRateLimitGroup(zero))

	// empty object is valid
	require.NoError(t, CheckRollingRateLimitGroup(`{}`))
}

func TestGetGroupRollingRateLimit(t *testing.T) {
	UserRollingRateLimitGroup = map[string][]RollingRateLimitTier{
		"vip": {{Duration: 18000, Limit: 5000}},
	}
	tiers, found := GetGroupRollingRateLimit("vip")
	assert.True(t, found)
	require.Len(t, tiers, 1)
	assert.Equal(t, int64(18000), tiers[0].Duration)

	_, found = GetGroupRollingRateLimit("nonexistent")
	assert.False(t, found)
}
