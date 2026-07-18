package service

import (
	"strings"
	"testing"
)

func BenchmarkEstimateInputTokens_100KB(b *testing.B) {
	// 100KB body，混合 CJK 与英文
	// 单条消息内容约 100KB
	content := strings.Repeat("a", 70000) + strings.Repeat("你", 15000) // ~70000 + 45000 = 115000 bytes
	body := []byte(`{"messages":[{"role":"user","content":"` + content + `"}]}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EstimateInputTokens(body)
	}
}
