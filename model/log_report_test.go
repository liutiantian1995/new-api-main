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
