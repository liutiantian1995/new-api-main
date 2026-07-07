package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func BenchmarkComputeEffectivePriority(b *testing.B) {
	tiers := []TokenTier{
		{MaxTokens: 50000, PriorityBoost: 5},
		{MaxTokens: 200000, PriorityBoost: 3},
	}
	ch := &Channel{
		Priority:   new(int64),
		Weight:     new(uint),
		TokenTiers: tiers,
	}
	*ch.Priority = 10
	w := uint(1)
	ch.Weight = &w
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = computeEffectivePriority(ch, 80000)
	}
}

func BenchmarkFilterChannelsByMaxTokens_50(b *testing.B) {
	channels := make([]*Channel, 50)
	ids := make([]int, 50)
	for i := 0; i < 50; i++ {
		ids[i] = i + 1
		channels[i] = &Channel{
			Id:        i + 1,
			Priority:  new(int64),
			Weight:    new(uint),
			MaxTokens: (i + 1) * 5000,
		}
		*channels[i].Priority = int64(i)
		w := uint(1)
		channels[i].Weight = &w
	}
	prevMem := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	prevIDM := channelsIDM
	channelsIDM = make(map[int]*Channel, 50)
	for _, ch := range channels {
		channelsIDM[ch.Id] = ch
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = filterChannelsByMaxTokens(ids, 80000)
	}
	b.Cleanup(func() {
		common.MemoryCacheEnabled = prevMem
		channelsIDM = prevIDM
	})
}
