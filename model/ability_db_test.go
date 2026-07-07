package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeDBChannel 在测试 DB 中创建一条 Channel 记录，返回带自增 Id 的指针。
// 用于 GetChannel DB 直查路径测试。
func makeDBChannel(t *testing.T, name string, priority int64, weight uint, maxTokens int, tiers []TokenTier) *Channel {
	t.Helper()
	ch := &Channel{
		Name:       name,
		Key:        "sk-test-" + name,
		Status:     common.ChannelStatusEnabled,
		Priority:   &priority,
		Weight:     &weight,
		MaxTokens:  maxTokens,
		TokenTiers: tiers,
		Group:      "default",
		Models:     "gpt-4",
	}
	require.NoError(t, DB.Create(ch).Error)
	return ch
}

// makeDBAbility 在测试 DB 中创建一条 Ability 记录，关联 group/model 到 channel。
func makeDBAbility(t *testing.T, group, model string, channelId int, enabled bool, priority int64, weight uint) {
	t.Helper()
	ability := Ability{
		Group:     group,
		Model:     model,
		ChannelId: channelId,
		Enabled:   enabled,
		Priority:  &priority,
		Weight:    weight,
	}
	require.NoError(t, DB.Create(&ability).Error)
}

// ---------------------------------------------------------------------------
// filterChannelsByMaxTokensFromMap — 纯函数测试（无需 DB）
// ---------------------------------------------------------------------------

func TestFilterChannelsByMaxTokensFromMap_NoEstTokens(t *testing.T) {
	channelsById := map[int]*Channel{
		1: {Id: 1, MaxTokens: 50000},
		2: {Id: 2, MaxTokens: 0},
	}
	got, fallback := filterChannelsByMaxTokensFromMap([]int{1, 2}, channelsById, 0)
	require.False(t, fallback)
	assert.ElementsMatch(t, []int{1, 2}, got)
}

func TestFilterChannelsByMaxTokensFromMap_SoftFilter(t *testing.T) {
	channelsById := map[int]*Channel{
		1: {Id: 1, MaxTokens: 50000},
		2: {Id: 2, MaxTokens: 200000},
		3: {Id: 3, MaxTokens: 0}, // 未配置 → 永远通过
	}
	// estTokens=80000 → ch1 (50k) 被排除
	got, fallback := filterChannelsByMaxTokensFromMap([]int{1, 2, 3}, channelsById, 80000)
	require.False(t, fallback)
	assert.ElementsMatch(t, []int{2, 3}, got)
}

func TestFilterChannelsByMaxTokensFromMap_FallbackWhenAllExcluded(t *testing.T) {
	channelsById := map[int]*Channel{
		1: {Id: 1, MaxTokens: 50000},
		2: {Id: 2, MaxTokens: 80000},
	}
	// estTokens=500000 → 全部被排除 → fallback 返回原始集合
	got, fallback := filterChannelsByMaxTokensFromMap([]int{1, 2}, channelsById, 500000)
	require.True(t, fallback)
	assert.ElementsMatch(t, []int{1, 2}, got)
}

func TestFilterChannelsByMaxTokensFromMap_UnknownIdKept(t *testing.T) {
	// map 中不存在的 id 保留，让下游发出一致性错误
	channelsById := map[int]*Channel{
		1: {Id: 1, MaxTokens: 50000},
	}
	// estTokens=80000 → ch1 被 max_tokens 排除，999 未知 → 保留
	got, fallback := filterChannelsByMaxTokensFromMap([]int{1, 999}, channelsById, 80000)
	require.False(t, fallback)
	assert.ElementsMatch(t, []int{999}, got)
}

func TestFilterChannelsByMaxTokensFromMap_EmptyInput(t *testing.T) {
	got, fallback := filterChannelsByMaxTokensFromMap(nil, nil, 1000)
	require.False(t, fallback)
	assert.Empty(t, got)
}

// ---------------------------------------------------------------------------
// GetChannel — DB 集成测试
// ---------------------------------------------------------------------------

func TestGetChannel_DefaultsUnchanged(t *testing.T) {
	truncateTables(t)
	// 无 token 配置 → 行为同 main 分支：按 base priority 分 tier，weight 加权随机
	ch1 := makeDBChannel(t, "ch-default-1", 10, 1, 0, nil)
	ch2 := makeDBChannel(t, "ch-default-2", 10, 1, 0, nil)
	ch3 := makeDBChannel(t, "ch-default-3", 5, 1, 0, nil)
	makeDBAbility(t, "default", "gpt-4", ch1.Id, true, 10, 1)
	makeDBAbility(t, "default", "gpt-4", ch2.Id, true, 10, 1)
	makeDBAbility(t, "default", "gpt-4", ch3.Id, true, 5, 1)

	// retry=0 → 最高 base priority tier (10)
	got, fallback, err := GetChannel("default", "gpt-4", 0, "", 0)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, fallback)
	assert.Contains(t, []int{ch1.Id, ch2.Id}, got.Id)

	// retry=1 → 次低 base priority tier (5)
	got, fallback, err = GetChannel("default", "gpt-4", 1, "", 0)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, fallback)
	assert.Equal(t, ch3.Id, got.Id)
}

func TestGetChannel_MaxTokensSoftFilter(t *testing.T) {
	truncateTables(t)
	// 同 base priority 10，ch1 max 50k, ch2 max 1M
	// 80k 请求 → ch1 被软过滤，只剩 ch2
	// 注意：DB 路径 getChannelQuery 按 base priority 分 tier 查询，
	// 同 tier 内才能互相替代；不同 tier 不会跨层补位。
	ch1 := makeDBChannel(t, "ch-maxfilter-1", 10, 1, 50000, nil)
	ch2 := makeDBChannel(t, "ch-maxfilter-2", 10, 1, 1000000, nil)
	makeDBAbility(t, "default", "gpt-4", ch1.Id, true, 10, 1)
	makeDBAbility(t, "default", "gpt-4", ch2.Id, true, 10, 1)

	got, fallback, err := GetChannel("default", "gpt-4", 0, "", 80000)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, fallback)
	assert.Equal(t, ch2.Id, got.Id, "ch1 应被 max_tokens 软过滤，只剩 ch2")
}

func TestGetChannel_AllFilteredFallback(t *testing.T) {
	truncateTables(t)
	// 两个渠道 max_tokens 都 < 500000 → 全部被过滤 → fallback
	ch1 := makeDBChannel(t, "ch-fallback-1", 10, 1, 50000, nil)
	ch2 := makeDBChannel(t, "ch-fallback-2", 8, 1, 80000, nil)
	makeDBAbility(t, "default", "gpt-4", ch1.Id, true, 10, 1)
	makeDBAbility(t, "default", "gpt-4", ch2.Id, true, 8, 1)

	got, fallback, err := GetChannel("default", "gpt-4", 0, "", 500000)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.True(t, fallback, "所有候选被 max_tokens 过滤 → fallback=true")
	assert.Contains(t, []int{ch1.Id, ch2.Id}, got.Id)
}

func TestGetChannel_TierBoostReordersWithinBaseTier(t *testing.T) {
	truncateTables(t)
	// 同 base priority 10，但 ch1 有 boost +5（estTokens ≤ 50k 时）
	// ch1: effective = 15（小请求时）
	// ch2: effective = 10（无 boost）
	ch1 := makeDBChannel(t, "ch-boost-1", 10, 1, 0, []TokenTier{{MaxTokens: 50000, PriorityBoost: 5}})
	ch2 := makeDBChannel(t, "ch-boost-2", 10, 1, 0, nil)
	makeDBAbility(t, "default", "gpt-4", ch1.Id, true, 10, 1)
	makeDBAbility(t, "default", "gpt-4", ch2.Id, true, 10, 1)

	// 小请求 → ch1 boost 生效 → effective tier [15] 优先
	got, fallback, err := GetChannel("default", "gpt-4", 0, "", 1000)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, fallback)
	assert.Equal(t, ch1.Id, got.Id, "小请求 boost 生效应选 ch1")

	// retry=1 → 降到 ch2 所在 effective tier (10)
	got, fallback, err = GetChannel("default", "gpt-4", 1, "", 1000)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, fallback)
	assert.Equal(t, ch2.Id, got.Id, "retry=1 应降到 ch2 所在 effective tier")
}

func TestGetChannel_TierBoostNoEffectWhenEstTokensExceedsTierMax(t *testing.T) {
	truncateTables(t)
	// ch1: base 10, boost +5 when estTokens ≤ 50k
	// ch2: base 10, no boost
	// 大请求 (100k > 50k) → boost 不生效，两者 effective 都是 10
	ch1 := makeDBChannel(t, "ch-boost-large-1", 10, 1, 0, []TokenTier{{MaxTokens: 50000, PriorityBoost: 5}})
	ch2 := makeDBChannel(t, "ch-boost-large-2", 10, 1, 0, nil)
	makeDBAbility(t, "default", "gpt-4", ch1.Id, true, 10, 1)
	makeDBAbility(t, "default", "gpt-4", ch2.Id, true, 10, 1)

	// 大请求 → boost 不生效 → 两渠道同 effective tier，随机选
	got, _, err := GetChannel("default", "gpt-4", 0, "", 100000)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Contains(t, []int{ch1.Id, ch2.Id}, got.Id)
}

func TestGetChannel_NoAbilitiesReturnsNil(t *testing.T) {
	truncateTables(t)
	// 无匹配 ability → 返回 nil，无错误
	got, fallback, err := GetChannel("default", "nonexistent-model", 0, "", 0)
	require.NoError(t, err)
	assert.Nil(t, got)
	require.False(t, fallback)
}

func TestGetChannel_DisabledAbilityNotSelected(t *testing.T) {
	truncateTables(t)
	// ch1 的 ability enabled=false → 不应被选中
	ch1 := makeDBChannel(t, "ch-disabled-1", 10, 1, 0, nil)
	ch2 := makeDBChannel(t, "ch-disabled-2", 5, 1, 0, nil)
	makeDBAbility(t, "default", "gpt-4", ch1.Id, false, 10, 1) // disabled
	makeDBAbility(t, "default", "gpt-4", ch2.Id, true, 5, 1)

	got, fallback, err := GetChannel("default", "gpt-4", 0, "", 0)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, fallback)
	assert.Equal(t, ch2.Id, got.Id, "ch1 ability disabled，应选 ch2")
}

// TestGetChannel_AllWeightsZero 验证 DB 路径 weight=0 smoothing 兜底：
// 同 effective tier 内所有渠道 weight=0（Channel 默认值）时，
// 必须均匀随机选中一个，而非返回 nil 造成 503。
// 对齐内存缓存路径 (GetRandomSatisfiedChannel) 的 smoothing 行为。
func TestGetChannel_AllWeightsZero(t *testing.T) {
	truncateTables(t)
	// 两个渠道 weight=0（默认值），同 base priority
	ch1 := makeDBChannel(t, "ch-zero-1", 10, 0, 0, nil)
	ch2 := makeDBChannel(t, "ch-zero-2", 10, 0, 0, nil)
	makeDBAbility(t, "default", "gpt-4", ch1.Id, true, 10, 0)
	makeDBAbility(t, "default", "gpt-4", ch2.Id, true, 10, 0)

	// 多次抽样验证两个渠道都可能被选中（weight=0 不应导致 503）
	selected := make(map[int]bool)
	for i := 0; i < 20; i++ {
		got, fallback, err := GetChannel("default", "gpt-4", 0, "", 0)
		require.NoError(t, err)
		require.NotNil(t, got, "weight=0 时必须选中渠道，不能返回 nil")
		require.False(t, fallback)
		selected[got.Id] = true
	}
	// 20 次抽样应覆盖两个渠道（概率上几乎必然）
	assert.Len(t, selected, 2, "weight=0 smoothing 应均匀分布到两个渠道")
}

// TestGetChannel_LowWeightSmoothing 验证平均 weight < 10 时 smoothingFactor=100 生效：
// 加权随机仍能正确按比例分布，不会因 weight 过小导致选不到低 weight 渠道。
func TestGetChannel_LowWeightSmoothing(t *testing.T) {
	truncateTables(t)
	// ch1 weight=1, ch2 weight=1, 同 base priority 10
	// 平均 weight=1 < 10 → smoothingFactor=100，有效权重 100/100
	ch1 := makeDBChannel(t, "ch-low-1", 10, 1, 0, nil)
	ch2 := makeDBChannel(t, "ch-low-2", 10, 1, 0, nil)
	makeDBAbility(t, "default", "gpt-4", ch1.Id, true, 10, 1)
	makeDBAbility(t, "default", "gpt-4", ch2.Id, true, 10, 1)

	selected := make(map[int]bool)
	for i := 0; i < 20; i++ {
		got, _, err := GetChannel("default", "gpt-4", 0, "", 0)
		require.NoError(t, err)
		require.NotNil(t, got)
		selected[got.Id] = true
	}
	assert.Len(t, selected, 2, "smoothingFactor=100 应让两个 weight=1 渠道都可能被选中")
}
