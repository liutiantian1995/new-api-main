package model

import (
	"github.com/QuantumNous/new-api/dto"
)

// ComputeRoutingBasis 根据路由决策路径计算 RoutingInfo。
//
// basis 判定优先级（先匹配先返回）：
//  1. fallback — 全空回退（优先级最高，即使 boost 生效也标记 fallback）
//  2. affinity — 亲和命中分支选中（未走加权随机）
//  3. tier_boost — estTokens>0 且 effective_priority > base_priority
//  4. default — 其余所有情况
//
// 参数：
//   - channel: 选中的渠道（用于读取 base_priority）
//   - estTokens: 估算输入 token 数
//   - affinityUsed: 是否通过亲和命中分支选中
//   - fallback: 是否因 max_tokens 全空回退到全集合
//
// affinity 路径下 effective_priority = base_priority（未走 computeEffectivePriority），
// boost = 0；其余路径调用 computeEffectivePriority 计算。
func ComputeRoutingBasis(channel *Channel, estTokens int, affinityUsed, fallback bool) *dto.RoutingInfo {
	if channel == nil {
		return nil
	}

	basePriority := channel.GetPriority()
	var effectivePriority int64
	var topTier *TokenTier
	if affinityUsed {
		// 亲和命中未走 effective_priority 计算，直接用 base
		effectivePriority = basePriority
	} else {
		effectivePriority, topTier = computeEffectivePriorityWithTier(channel, estTokens)
	}
	boost := effectivePriority - basePriority

	basis := dto.RoutingBasisDefault
	if fallback {
		basis = dto.RoutingBasisFallback
	} else if affinityUsed {
		basis = dto.RoutingBasisAffinity
	} else if estTokens > 0 && effectivePriority > basePriority {
		basis = dto.RoutingBasisTierBoost
	}

	info := &dto.RoutingInfo{
		Basis:             basis,
		EstTokens:         estTokens,
		BasePriority:      basePriority,
		EffectivePriority: effectivePriority,
		Boost:             boost,
		Fallback:          fallback,
	}
	// topTier 指向 channel.TokenTiers 切片元素；此处 copy 出值后写入 DTO，
	// 避免后续 channel 缓存替换时产生悬挂引用。
	if topTier != nil {
		info.TierMaxTokens = topTier.MaxTokens
	}
	return info
}
