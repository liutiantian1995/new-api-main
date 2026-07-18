package dto

// RoutingInfo 记录单次 API 请求的渠道路由决策依据，由 distributor 计算，
// 存入 gin context（ContextKeyRoutingBasis），最终由 RecordConsumeLog 写入
// Log.Other.routing_info（仅管理员可见）。
//
// basis 判定优先级：fallback > affinity > tier_boost > default
type RoutingInfo struct {
	// Basis 决策路径类型：affinity | tier_boost | default | fallback
	Basis string `json:"basis"`
	// EstTokens 估算输入 token 数（0=未估算/非文本路径）
	EstTokens int `json:"est_tokens"`
	// BasePriority 选中渠道的 base priority
	BasePriority int64 `json:"base_priority"`
	// EffectivePriority 选中渠道的 effective priority（base + Σ boost）
	EffectivePriority int64 `json:"effective_priority"`
	// Boost 生效的 boost 总量（= effective_priority - base_priority，可为负）
	Boost int64 `json:"boost"`
	// Fallback 是否因 max_tokens 全空回退到全集合
	Fallback bool `json:"fallback"`
	// TierMaxTokens 命中的最大 token_tier 的 max_tokens（max_tokens 最大的那档）；
	// 仅 basis=tier_boost 时有值。omitempty 保证旧日志反序列化向后兼容。
	TierMaxTokens int `json:"tier_max_tokens,omitempty"`
}

// Routing basis 类型常量
const (
	RoutingBasisAffinity  = "affinity"
	RoutingBasisTierBoost = "tier_boost"
	RoutingBasisDefault   = "default"
	RoutingBasisFallback  = "fallback"
)
