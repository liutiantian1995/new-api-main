package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractCacheTokens_Empty(t *testing.T) {
	assert.Equal(t, 0, extractCacheTokens(""))
}

func TestExtractCacheTokens_MalformedJSON(t *testing.T) {
	assert.Equal(t, 0, extractCacheTokens("not json"))
}

func TestExtractCacheTokens_NoUsage(t *testing.T) {
	assert.Equal(t, 0, extractCacheTokens(`{"foo":"bar"}`))
}

func TestExtractCacheTokens_OpenAIFormat(t *testing.T) {
	// OpenAI: usage.prompt_tokens_details.cached_tokens
	j := `{"usage":{"prompt_tokens":1000,"prompt_tokens_details":{"cached_tokens":300},"completion_tokens":500}}`
	assert.Equal(t, 300, extractCacheTokens(j))
}

func TestExtractCacheTokens_ClaudeFormat(t *testing.T) {
	// Claude: usage.cache_read_input_tokens + cache_creation_input_tokens
	j := `{"usage":{"input_tokens":1000,"cache_read_input_tokens":200,"cache_creation_input_tokens":50,"output_tokens":400}}`
	assert.Equal(t, 250, extractCacheTokens(j))
}

func TestExtractCacheTokens_BothFormatsSummed(t *testing.T) {
	// Hypothetical log with both fields populated (e.g. legacy format)
	j := `{"usage":{"prompt_tokens_details":{"cached_tokens":100},"cache_read_input_tokens":80,"cache_creation_input_tokens":20}}`
	assert.Equal(t, 200, extractCacheTokens(j))
}

func TestExtractCacheTokens_DetailsObjectButNoCachedTokens(t *testing.T) {
	j := `{"usage":{"prompt_tokens_details":{"image_tokens":50}}}`
	assert.Equal(t, 0, extractCacheTokens(j))
}

func TestExtractCacheTokens_NumericAsString(t *testing.T) {
	// If JSON encodes numbers as strings (defensive), expect 0 — we only handle float64
	j := `{"usage":{"cache_read_input_tokens":"100"}}`
	assert.Equal(t, 0, extractCacheTokens(j))
}

func TestExtractCacheTokens_TopLevelNewAPIFormat(t *testing.T) {
	// new-api 自身写入路径：service/log_info_generate.go:GenerateTextOtherInfo
	// other["cache_tokens"] 直接放在顶层，不嵌套在 usage 子对象下。
	j := `{"cache_tokens":300,"cache_ratio":0.5,"model_ratio":0.1,"group_ratio":1.0}`
	assert.Equal(t, 300, extractCacheTokens(j))
}

func TestExtractCacheTokens_TopLevelZeroIgnored(t *testing.T) {
	// 顶层 cache_tokens=0 时应被 float64 解析跳过（类型断言成功但值为 0，贡献 0）
	j := `{"cache_tokens":0,"cache_ratio":0}`
	assert.Equal(t, 0, extractCacheTokens(j))
}

func TestExtractCacheTokens_TopLevelMissingOtherFieldsPresent(t *testing.T) {
	// other 字段存在但无 cache_tokens 顶层键，也无 usage 嵌套，应返回 0
	j := `{"model_ratio":0.1,"frt":1234.5,"admin_info":{"use_channel":["ch1"]}}`
	assert.Equal(t, 0, extractCacheTokens(j))
}
