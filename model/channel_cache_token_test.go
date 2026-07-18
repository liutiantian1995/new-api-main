package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// setupChannelCacheForTest populates channelsIDM and group2model2channels with a
// minimal in-memory set of channels so GetRandomSatisfiedChannel can run
// without hitting the database. Caller is responsible for saving/restoring the
// original global state.
func setupChannelCacheForTest(t *testing.T, channels []*Channel, group2model map[string][]int) {
	t.Helper()
	prevIDM := channelsIDM
	prevG2M := group2model2channels
	prevAdvanced := channel2advancedCustomConfig
	prevMemCache := common.MemoryCacheEnabled
	t.Cleanup(func() {
		channelsIDM = prevIDM
		group2model2channels = prevG2M
		channel2advancedCustomConfig = prevAdvanced
		common.MemoryCacheEnabled = prevMemCache
	})

	common.MemoryCacheEnabled = true
	channelsIDM = make(map[int]*Channel, len(channels))
	for _, ch := range channels {
		channelsIDM[ch.Id] = ch
	}
	group2model2channels = map[string]map[string][]int{
		"default": group2model,
	}
	channel2advancedCustomConfig = nil
}

func makeChannel(id int, priority int64, weight int, maxTokens int, tiers []TokenTier) *Channel {
	w := uint(weight)
	return &Channel{
		Id:         id,
		Name:       "ch-" + string(rune('A'+id-1)),
		Priority:   &priority,
		Weight:     &w,
		MaxTokens:  maxTokens,
		TokenTiers: tiers,
		Status:     1, // common.ChannelStatusEnabled
		Group:      "default",
		Models:     "gpt-4",
	}
}

func TestComputeEffectivePriority_NoTiers(t *testing.T) {
	ch := makeChannel(1, 10, 1, 0, nil)
	require.Equal(t, int64(10), computeEffectivePriority(ch, 0))
	require.Equal(t, int64(10), computeEffectivePriority(ch, 100000))
}

func TestComputeEffectivePriority_SingleTierHit(t *testing.T) {
	ch := makeChannel(1, 10, 1, 0, []TokenTier{{MaxTokens: 50000, PriorityBoost: 5}})
	require.Equal(t, int64(15), computeEffectivePriority(ch, 50000))
	require.Equal(t, int64(15), computeEffectivePriority(ch, 1000))
	// estTokens above tier → no boost
	require.Equal(t, int64(10), computeEffectivePriority(ch, 50001))
}

func TestComputeEffectivePriority_MultiTierAccumulate(t *testing.T) {
	ch := makeChannel(1, 10, 1, 0, []TokenTier{
		{MaxTokens: 50000, PriorityBoost: 5},
		{MaxTokens: 200000, PriorityBoost: 3},
	})
	// 1k ≤ 50k → +5+3 = +8
	require.Equal(t, int64(18), computeEffectivePriority(ch, 1000))
	// 50k ≤ 200k → +3
	require.Equal(t, int64(13), computeEffectivePriority(ch, 60000))
	// above all → +0
	require.Equal(t, int64(10), computeEffectivePriority(ch, 300000))
}

func TestFilterChannelsByMaxTokens_NoEstTokens(t *testing.T) {
	channels := []*Channel{makeChannel(1, 10, 1, 100, nil), makeChannel(2, 10, 1, 0, nil)}
	setupChannelCacheForTest(t, channels, map[string][]int{"gpt-4": {1, 2}})
	got, fallback := filterChannelsByMaxTokens([]int{1, 2}, 0)
	require.False(t, fallback)
	require.ElementsMatch(t, []int{1, 2}, got)
}

func TestFilterChannelsByMaxTokens_SoftFilter(t *testing.T) {
	channels := []*Channel{
		makeChannel(1, 10, 1, 50000, nil),
		makeChannel(2, 10, 1, 200000, nil),
		makeChannel(3, 10, 1, 0, nil), // unconfigured → always pass
	}
	setupChannelCacheForTest(t, channels, map[string][]int{"gpt-4": {1, 2, 3}})
	// estTokens=80000 → ch1 (50k) excluded
	got, fallback := filterChannelsByMaxTokens([]int{1, 2, 3}, 80000)
	require.False(t, fallback)
	require.ElementsMatch(t, []int{2, 3}, got)
}

func TestFilterChannelsByMaxTokens_FallbackWhenAllExcluded(t *testing.T) {
	channels := []*Channel{
		makeChannel(1, 10, 1, 50000, nil),
		makeChannel(2, 10, 1, 80000, nil),
	}
	setupChannelCacheForTest(t, channels, map[string][]int{"gpt-4": {1, 2}})
	// estTokens=500000 → both excluded → fallback returns original set
	got, fallback := filterChannelsByMaxTokens([]int{1, 2}, 500000)
	require.True(t, fallback)
	require.ElementsMatch(t, []int{1, 2}, got)
}

func TestGetRandomSatisfiedChannel_DefaultsUnchanged(t *testing.T) {
	// No token config → behaves like main branch: base priority tiers, weight random.
	channels := []*Channel{
		makeChannel(1, 10, 1, 0, nil),
		makeChannel(2, 10, 1, 0, nil),
		makeChannel(3, 5, 1, 0, nil),
	}
	setupChannelCacheForTest(t, channels, map[string][]int{"gpt-4": {1, 2, 3}})

	// retry=0 → top tier (priority 10)
	ch, fallback, err := GetRandomSatisfiedChannel("default", "gpt-4", 0, "", 0)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.False(t, fallback)
	require.Contains(t, []int{1, 2}, ch.Id)

	// retry=1 → next tier (priority 5)
	ch, fallback, err = GetRandomSatisfiedChannel("default", "gpt-4", 1, "", 0)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.False(t, fallback)
	require.Equal(t, 3, ch.Id)
}

func TestGetRandomSatisfiedChannel_TierBoostReordersTiers(t *testing.T) {
	// ch1 base 10, boost +5 when estTokens ≤ 50k → effective 15
	// ch2 base 8, no boost → effective 8
	channels := []*Channel{
		makeChannel(1, 10, 1, 0, []TokenTier{{MaxTokens: 50000, PriorityBoost: 5}}),
		makeChannel(2, 8, 1, 0, nil),
	}
	setupChannelCacheForTest(t, channels, map[string][]int{"gpt-4": {1, 2}})

	// small request → ch1 boost applies → tier [15] first
	ch, fallback, err := GetRandomSatisfiedChannel("default", "gpt-4", 0, "", 1000)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.False(t, fallback)
	require.Equal(t, 1, ch.Id)

	// ch1 should be the only channel in the top effective tier; retry=1 picks ch2
	ch, fallback, err = GetRandomSatisfiedChannel("default", "gpt-4", 1, "", 1000)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.False(t, fallback)
	require.Equal(t, 2, ch.Id)
}

func TestGetRandomSatisfiedChannel_MaxTokensFallbackReturnsTrue(t *testing.T) {
	channels := []*Channel{
		makeChannel(1, 10, 1, 50000, nil),
		makeChannel(2, 8, 1, 80000, nil),
	}
	setupChannelCacheForTest(t, channels, map[string][]int{"gpt-4": {1, 2}})

	// huge request → both excluded → fallback to full set, fallback flag returned
	ch, fallback, err := GetRandomSatisfiedChannel("default", "gpt-4", 0, "", 500000)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.True(t, fallback)
	require.Contains(t, []int{1, 2}, ch.Id)
}

func TestGetRandomSatisfiedChannel_MaxTokensFiltersOut(t *testing.T) {
	// ch1 max 50k, ch2 max 1M. 80k request → only ch2 candidate.
	channels := []*Channel{
		makeChannel(1, 10, 1, 50000, nil),
		makeChannel(2, 5, 1, 1000000, nil),
	}
	setupChannelCacheForTest(t, channels, map[string][]int{"gpt-4": {1, 2}})

	ch, fallback, err := GetRandomSatisfiedChannel("default", "gpt-4", 0, "", 80000)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.False(t, fallback)
	require.Equal(t, 2, ch.Id)
}
