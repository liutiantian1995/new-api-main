package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// TestValidateChannel_TokenRoutingRules verifies the token-aware routing
// validation added for the max_tokens / token_tiers fields.
func TestValidateChannel_TokenRoutingRules(t *testing.T) {
	t.Run("valid: no token routing config", func(t *testing.T) {
		ch := &model.Channel{Key: "k", MaxTokens: 0, TokenTiers: nil}
		require.NoError(t, validateChannel(ch, true))
	})

	t.Run("valid: max_tokens > 0 and well-formed tiers", func(t *testing.T) {
		ch := &model.Channel{
			Key:       "k",
			MaxTokens: 200000,
			TokenTiers: []model.TokenTier{
				{MaxTokens: 50000, PriorityBoost: 5},
				{MaxTokens: 200000, PriorityBoost: -3},
			},
		}
		require.NoError(t, validateChannel(ch, true))
	})

	t.Run("invalid: negative max_tokens", func(t *testing.T) {
		ch := &model.Channel{Key: "k", MaxTokens: -1}
		err := validateChannel(ch, true)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_tokens")
	})

	t.Run("invalid: tier max_tokens = 0", func(t *testing.T) {
		ch := &model.Channel{
			Key:        "k",
			MaxTokens:  100000,
			TokenTiers: []model.TokenTier{{MaxTokens: 0, PriorityBoost: 5}},
		}
		err := validateChannel(ch, true)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_tokens")
	})

	t.Run("invalid: priority_boost over upper bound", func(t *testing.T) {
		ch := &model.Channel{
			Key:        "k",
			MaxTokens:  100000,
			TokenTiers: []model.TokenTier{{MaxTokens: 50000, PriorityBoost: 101}},
		}
		err := validateChannel(ch, true)
		require.Error(t, err)
		require.Contains(t, err.Error(), "priority_boost")
	})

	t.Run("invalid: priority_boost under lower bound", func(t *testing.T) {
		ch := &model.Channel{
			Key:        "k",
			MaxTokens:  100000,
			TokenTiers: []model.TokenTier{{MaxTokens: 50000, PriorityBoost: -101}},
		}
		err := validateChannel(ch, true)
		require.Error(t, err)
		require.Contains(t, err.Error(), "priority_boost")
	})

	t.Run("invalid: token_tiers count over limit (11 > 10)", func(t *testing.T) {
		tiers := make([]model.TokenTier, 11)
		for i := range tiers {
			tiers[i] = model.TokenTier{MaxTokens: 10000 * (i + 1), PriorityBoost: 1}
		}
		ch := &model.Channel{
			Key:        "k",
			MaxTokens:  200000,
			TokenTiers: tiers,
		}
		err := validateChannel(ch, true)
		require.Error(t, err)
		require.Contains(t, err.Error(), "token_tiers")
	})

	t.Run("valid: token_tiers at limit (10)", func(t *testing.T) {
		tiers := make([]model.TokenTier, 10)
		for i := range tiers {
			tiers[i] = model.TokenTier{MaxTokens: 10000 * (i + 1), PriorityBoost: 1}
		}
		ch := &model.Channel{
			Key:        "k",
			MaxTokens:  200000,
			TokenTiers: tiers,
		}
		require.NoError(t, validateChannel(ch, true))
	})

	t.Run("missing fields default to zero values (legacy client)", func(t *testing.T) {
		// 客户端不带 max_tokens / token_tiers → Go 零值 → 校验通过
		ch := &model.Channel{Key: "k"}
		require.NoError(t, validateChannel(ch, true))
		require.Equal(t, 0, ch.MaxTokens)
		require.Nil(t, ch.TokenTiers)
	})
}
