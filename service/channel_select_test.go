package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// setupChannelCacheForServiceTest populates model cache globals so that
// CacheGetRandomSatisfiedChannel → model.GetRandomSatisfiedChannel can run
// without DB. Mirrors model.setupChannelCacheForTest but lives in service pkg.
func setupChannelCacheForServiceTest(t *testing.T, channels []*model.Channel, group2model map[string][]int) {
	t.Helper()
	prevMemCache := common.MemoryCacheEnabled
	t.Cleanup(func() {
		common.MemoryCacheEnabled = prevMemCache
		// Best-effort: restore via model layer's InitChannelCache on next real sync.
	})
	common.MemoryCacheEnabled = true
	model.ResetChannelCacheForTest(channels, group2model)
}

func makeServiceChannel(id int, priority int64, weight int, maxTokens int, tiers []model.TokenTier) *model.Channel {
	w := uint(weight)
	return &model.Channel{
		Id:         id,
		Name:       "svc-ch-" + string(rune('A'+id-1)),
		Priority:   &priority,
		Weight:     &w,
		MaxTokens:  maxTokens,
		TokenTiers: tiers,
		Status:     1,
		Group:      "default",
		Models:     "gpt-4",
	}
}

func newEstTokensContext() *gin.Context {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request, _ = http.NewRequest("POST", "/v1/chat/completions", nil)
	return ctx
}

// TestRetryParam_EstTokensField_Preserved verifies the EstTokens field travels
// with RetryParam through IncreaseRetry/GetRetry without being reset.
func TestRetryParam_EstTokensField_Preserved(t *testing.T) {
	param := RetryParam{
		TokenGroup: "default",
		ModelName:  "gpt-4",
		EstTokens:  80000,
	}
	require.Equal(t, 80000, param.EstTokens)
	param.IncreaseRetry()
	require.Equal(t, 80000, param.EstTokens, "EstTokens must survive IncreaseRetry")
	require.Equal(t, 1, param.GetRetry())
}

// TestCacheGetRandomSatisfiedChannel_PassesEstTokens verifies the non-auto
// branch forwards EstTokens to model.GetRandomSatisfiedChannel so that a
// token-tier boost changes which channel is selected.
func TestCacheGetRandomSatisfiedChannel_PassesEstTokens(t *testing.T) {
	// ch1: base priority 10, +5 boost when estTokens ≤ 50k → effective 15
	// ch2: base priority 12, no boost → effective 12
	// With estTokens=1000, ch1 (effective 15) should win.
	channels := []*model.Channel{
		makeServiceChannel(1, 10, 1, 0, []model.TokenTier{{MaxTokens: 50000, PriorityBoost: 5}}),
		makeServiceChannel(2, 12, 1, 0, nil),
	}
	setupChannelCacheForServiceTest(t, channels, map[string][]int{"gpt-4": {1, 2}})

	retry := 0
	param := &RetryParam{
		Ctx:         newEstTokensContext(),
		TokenGroup:  "default",
		ModelName:   "gpt-4",
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
		EstTokens:   1000,
	}

	ch, _, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.Equal(t, 1, ch.Id, "estTokens boost should promote ch1 over higher-base ch2")
}

// TestCacheGetRandomSatisfiedChannel_ZeroEstTokensIgnoredBoost verifies that
// when EstTokens=0, no boost is applied and selection follows base priority.
func TestCacheGetRandomSatisfiedChannel_ZeroEstTokensIgnoredBoost(t *testing.T) {
	// ch1: base 10, +5 boost at ≤50k (skipped because estTokens=0)
	// ch2: base 12, no boost
	// With estTokens=0, ch2 (base 12) should win — boost must not fire.
	channels := []*model.Channel{
		makeServiceChannel(1, 10, 1, 0, []model.TokenTier{{MaxTokens: 50000, PriorityBoost: 5}}),
		makeServiceChannel(2, 12, 1, 0, nil),
	}
	setupChannelCacheForServiceTest(t, channels, map[string][]int{"gpt-4": {1, 2}})

	retry := 0
	param := &RetryParam{
		Ctx:         newEstTokensContext(),
		TokenGroup:  "default",
		ModelName:   "gpt-4",
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
		EstTokens:   0,
	}

	ch, _, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.Equal(t, 2, ch.Id, "estTokens=0 must skip boost so ch2 (base 12) wins")
}
