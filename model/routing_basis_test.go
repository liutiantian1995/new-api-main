package model

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 4.1 TestComputeRoutingBasis — basis 判定优先级
// fallback > affinity > tier_boost > default
// ---------------------------------------------------------------------------

func TestComputeRoutingBasis_Fallback(t *testing.T) {
	// 即使 boost 生效，fallback 优先级最高
	ch := makeChannel(1, 10, 1, 0, []TokenTier{{MaxTokens: 50000, PriorityBoost: 5}})
	info := ComputeRoutingBasis(ch, 1000, false, true)
	require.NotNil(t, info)
	assert.Equal(t, dto.RoutingBasisFallback, info.Basis)
	assert.True(t, info.Fallback)
	// fallback 时仍计算 effective_priority（用于诊断）
	assert.Equal(t, int64(15), info.EffectivePriority)
	assert.Equal(t, int64(5), info.Boost)
	// fallback 路径下仍记录命中的 tier（用于诊断 fallback 命中的是哪档）
	assert.Equal(t, 50000, info.TierMaxTokens)
}

func TestComputeRoutingBasis_Affinity(t *testing.T) {
	// 亲和命中，即使有 boost 也不算 tier_boost
	ch := makeChannel(1, 10, 1, 0, []TokenTier{{MaxTokens: 50000, PriorityBoost: 5}})
	info := ComputeRoutingBasis(ch, 1000, true, false)
	require.NotNil(t, info)
	assert.Equal(t, dto.RoutingBasisAffinity, info.Basis)
	assert.False(t, info.Fallback)
	// affinity 路径下 effective = base，boost = 0
	assert.Equal(t, int64(10), info.EffectivePriority)
	assert.Equal(t, int64(0), info.Boost)
	// affinity 路径未走 tier 计算，tier 字段为 0
	assert.Equal(t, 0, info.TierMaxTokens)
}

func TestComputeRoutingBasis_TierBoost(t *testing.T) {
	ch := makeChannel(1, 10, 1, 0, []TokenTier{{MaxTokens: 50000, PriorityBoost: 5}})
	info := ComputeRoutingBasis(ch, 1000, false, false)
	require.NotNil(t, info)
	assert.Equal(t, dto.RoutingBasisTierBoost, info.Basis)
	assert.False(t, info.Fallback)
	assert.Equal(t, int64(15), info.EffectivePriority)
	assert.Equal(t, int64(5), info.Boost)
	assert.Equal(t, 1000, info.EstTokens)
	// 命中最大档：max_tokens=50000
	assert.Equal(t, 50000, info.TierMaxTokens)
}

func TestComputeRoutingBasis_TierBoost_MultipleHits(t *testing.T) {
	// 配置两档 [{50000,+5},{200000,+3}]，estTokens=30000 同时命中两档
	// boost 累加 = 8，最大档为 200000（max_tokens 最大的那档）
	ch := makeChannel(1, 10, 1, 0, []TokenTier{
		{MaxTokens: 50000, PriorityBoost: 5},
		{MaxTokens: 200000, PriorityBoost: 3},
	})
	info := ComputeRoutingBasis(ch, 30000, false, false)
	require.NotNil(t, info)
	assert.Equal(t, dto.RoutingBasisTierBoost, info.Basis)
	// boost 累加
	assert.Equal(t, int64(18), info.EffectivePriority)
	assert.Equal(t, int64(8), info.Boost)
	// 最大档：max_tokens 最大的那档 = 200000
	assert.Equal(t, 200000, info.TierMaxTokens)
}

func TestComputeRoutingBasis_Default_NoEstTokens(t *testing.T) {
	ch := makeChannel(1, 10, 1, 0, nil)
	info := ComputeRoutingBasis(ch, 0, false, false)
	require.NotNil(t, info)
	assert.Equal(t, dto.RoutingBasisDefault, info.Basis)
	assert.Equal(t, int64(10), info.EffectivePriority)
	assert.Equal(t, int64(0), info.Boost)
	assert.Equal(t, 0, info.TierMaxTokens)
}

func TestComputeRoutingBasis_Default_BoostNotEffective(t *testing.T) {
	// estTokens 超过 tier max → boost 不生效 → default
	ch := makeChannel(1, 10, 1, 0, []TokenTier{{MaxTokens: 50000, PriorityBoost: 5}})
	info := ComputeRoutingBasis(ch, 100000, false, false)
	require.NotNil(t, info)
	assert.Equal(t, dto.RoutingBasisDefault, info.Basis)
	assert.Equal(t, int64(10), info.EffectivePriority)
	assert.Equal(t, int64(0), info.Boost)
	// 未命中任何 tier → tier 字段为 0
	assert.Equal(t, 0, info.TierMaxTokens)
}

func TestComputeRoutingBasis_NilChannel(t *testing.T) {
	info := ComputeRoutingBasis(nil, 1000, false, false)
	assert.Nil(t, info)
}

// ---------------------------------------------------------------------------
// 4.2 TestFormatUserLogs_StripsRoutingInfo — 非管理员日志剥离 routing_info
// ---------------------------------------------------------------------------

func TestFormatUserLogs_StripsRoutingInfo(t *testing.T) {
	routingInfo := dto.RoutingInfo{
		Basis:             dto.RoutingBasisTierBoost,
		EstTokens:         1000,
		BasePriority:      10,
		EffectivePriority: 15,
		Boost:             5,
		Fallback:          false,
		TierMaxTokens:     50000,
	}
	otherMap := map[string]interface{}{
		"routing_info": routingInfo,
		"admin_info":   "some admin detail",
	}
	otherBytes, _ := json.Marshal(otherMap)
	logs := []*Log{
		{Id: 1, Other: string(otherBytes)},
	}
	formatUserLogs(logs, 0)

	var result map[string]interface{}
	err := json.Unmarshal([]byte(logs[0].Other), &result)
	require.NoError(t, err)

	// routing_info 应被剥离
	_, exists := result["routing_info"]
	assert.False(t, exists, "routing_info 应被 formatUserLogs 剥离")
	// admin_info 也应被剥离（验证剥离机制正常工作）
	_, exists = result["admin_info"]
	assert.False(t, exists, "admin_info 应被剥离")
}

// ---------------------------------------------------------------------------
// 4.3 TestFormatUserLogs_OldLogNoRoutingInfo — 旧日志兼容
// ---------------------------------------------------------------------------

func TestFormatUserLogs_OldLogNoRoutingInfo(t *testing.T) {
	// 旧日志 Other 中无 routing_info key
	otherMap := map[string]interface{}{
		"some_field": "value",
	}
	otherBytes, _ := json.Marshal(otherMap)
	logs := []*Log{
		{Id: 1, Other: string(otherBytes)},
	}
	formatUserLogs(logs, 0)

	var result map[string]interface{}
	err := json.Unmarshal([]byte(logs[0].Other), &result)
	require.NoError(t, err)
	// 旧日志正常处理，some_field 保留
	assert.Equal(t, "value", result["some_field"])
	// routing_info 不存在（正常）
	_, exists := result["routing_info"]
	assert.False(t, exists)
}

func TestFormatUserLogs_EmptyOther(t *testing.T) {
	// Other 为空字符串 → StrToMap 返回 nil → MapToJsonStr(nil) = "null"
	// 这是 formatUserLogs 的既有行为，测试验证不 panic 即可
	logs := []*Log{
		{Id: 1, Other: ""},
	}
	formatUserLogs(logs, 0)
	// 不应 panic，Other 被标准化为 "null"（MapToJsonStr(nil) 的行为）
	assert.NotPanics(t, func() {
		formatUserLogs(logs, 0)
	})
}

// ---------------------------------------------------------------------------
// 4.4 TestRecordConsumeLog_InjectsRoutingInfo — RecordConsumeLog 注入 routing_info
// ---------------------------------------------------------------------------

func TestRecordConsumeLog_InjectsRoutingInfo(t *testing.T) {
	truncateTables(t)
	// 创建测试用户
	user := createTestUserForBatch(t, "routing_info_test_user")

	// 构造 gin context 并设置 routing_info
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("username", user.Username)
	c.Set(common.RequestIdKey, "test-req-id")
	routingInfo := &dto.RoutingInfo{
		Basis:             dto.RoutingBasisTierBoost,
		EstTokens:         1000,
		BasePriority:      10,
		EffectivePriority: 15,
		Boost:             5,
		Fallback:          false,
	}
	common.SetContextKey(c, constant.ContextKeyRoutingBasis, routingInfo)

	params := RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     100,
		CompletionTokens: 50,
		ModelName:        "gpt-4",
		TokenName:        "test-token",
		Quota:            1000,
		Content:          "test content",
		TokenId:          1,
		UseTimeSeconds:   1,
		IsStream:         false,
		Group:            "default",
	}

	RecordConsumeLog(c, user.Id, params)

	// 从 DB 查询日志验证 routing_info 存在
	var log Log
	err := DB.Where("user_id = ?", user.Id).Order("id DESC").First(&log).Error
	require.NoError(t, err)

	var otherMap map[string]interface{}
	err = json.Unmarshal([]byte(log.Other), &otherMap)
	require.NoError(t, err)

	info, exists := otherMap["routing_info"]
	require.True(t, exists, "routing_info 应被注入到 Other")
	infoMap, ok := info.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, dto.RoutingBasisTierBoost, infoMap["basis"])
}

func TestRecordConsumeLog_NoRoutingInfoInContext(t *testing.T) {
	truncateTables(t)
	user := createTestUserForBatch(t, "no_routing_info_test_user")

	// 不设置 ContextKeyRoutingBasis
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("username", user.Username)
	c.Set(common.RequestIdKey, "test-req-id-2")

	params := RecordConsumeLogParams{
		ChannelId: 1,
		ModelName: "gpt-4",
		Group:     "default",
	}

	RecordConsumeLog(c, user.Id, params)

	var log Log
	err := DB.Where("user_id = ?", user.Id).Order("id DESC").First(&log).Error
	require.NoError(t, err)

	// Other 可能为空或无 routing_info
	if log.Other != "" {
		var otherMap map[string]interface{}
		err = json.Unmarshal([]byte(log.Other), &otherMap)
		require.NoError(t, err)
		_, exists := otherMap["routing_info"]
		assert.False(t, exists, "context 无 routing_info 时 Other 不应包含该 key")
	}
}
