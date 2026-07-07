package model

// channel_cache_testing.go — Test-only helpers exported so service-layer tests
// can drive GetRandomSatisfiedChannel without a database. Lives outside _test.go
// so other packages (service) can reference it; production code must NOT call it.
//
// 约定：本文件中的导出符号（ResetChannelCacheForTest）仅供测试代码使用。
// 生产代码（非 *_test.go）严禁调用，否则会破坏缓存一致性。
// 如果未来需要更严格的隔离，可移至 model/internal/testutil 子包，
// 但当前为减少跨包 import 改动，保留在 model 包内并依赖命名约定约束。

// ResetChannelCacheForTest populates the in-memory channel cache (channelsIDM,
// group2model2channels, channel2advancedCustomConfig) with the given fixtures.
// Caller is responsible for saving/restoring original state if needed.
//
// Usage is restricted to tests: combine with common.MemoryCacheEnabled = true
// before calling. The function intentionally does not touch MemoryCacheEnabled
// because that flag lives in the common package and several call sites prefer
// to set it themselves with proper cleanup.
func ResetChannelCacheForTest(channels []*Channel, group2model map[string][]int) {
	idm := make(map[int]*Channel, len(channels))
	for _, ch := range channels {
		idm[ch.Id] = ch
	}
	channelsIDM = idm
	group2model2channels = map[string]map[string][]int{
		"default": group2model,
	}
	channel2advancedCustomConfig = nil
}
