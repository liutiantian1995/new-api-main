package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEstimateInputTokens_OpenAIChatStringContent(t *testing.T) {
	// 7000 非CJK + 1000 CJK
	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role":"user","content":"` + strings.Repeat("a", 7000) + `"},
			{"role":"assistant","content":"你好世界` + strings.Repeat("中", 996) + `"}
		]
	}`)
	// 7000/4 + 1000*2/3 = 1750 + 666 = 2416
	require.InDelta(t, 2416, EstimateInputTokens(body), 5)
}

func TestEstimateInputTokens_OpenAIChatArrayContent(t *testing.T) {
	// 多模态：text 部分 20 字符 + 12 字符 = 32 非CJK → 8 token
	body := []byte(`{
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"hello world hello"},
				{"type":"image_url","image_url":{"url":"data:..."}},
				{"type":"text","text":"another text  !"}
			]}
		]
	}`)
	got := EstimateInputTokens(body)
	require.Equal(t, 8, got)
}

func TestEstimateInputTokens_OpenAICompletionsPrompt(t *testing.T) {
	body := []byte(`{"prompt":"` + strings.Repeat("x", 40) + `"}`)
	require.Equal(t, 10, EstimateInputTokens(body))
}

func TestEstimateInputTokens_OpenAICompletionsPromptArray(t *testing.T) {
	body := []byte(`{"prompt":["` + strings.Repeat("x", 20) + `","` + strings.Repeat("y", 20) + `"]}`)
	require.Equal(t, 10, EstimateInputTokens(body))
}

func TestEstimateInputTokens_AnthropicMessages(t *testing.T) {
	body := []byte(`{
		"model":"claude-3-5-sonnet",
		"messages":[{"role":"user","content":[{"type":"text","text":"你好` + strings.Repeat("字", 2999) + `"}]}]
	}`)
	// 3000 CJK → 3000*2/3 = 2000
	require.Equal(t, 2000, EstimateInputTokens(body))
}

func TestEstimateInputTokens_Gemini(t *testing.T) {
	body := []byte(`{
		"contents":[
			{"parts":[{"text":"` + strings.Repeat("a", 100) + `"},{"text":"` + strings.Repeat("b", 100) + `"}]}
		]
	}`)
	require.Equal(t, 50, EstimateInputTokens(body))
}

func TestEstimateInputTokens_Embeddings(t *testing.T) {
	body := []byte(`{"input":"` + strings.Repeat("z", 40) + `"}`)
	require.Equal(t, 10, EstimateInputTokens(body))
}

func TestEstimateInputTokens_EmbeddingsArray(t *testing.T) {
	body := []byte(`{"input":["` + strings.Repeat("z", 20) + `","` + strings.Repeat("z", 20) + `"]}`)
	require.Equal(t, 10, EstimateInputTokens(body))
}

func TestEstimateInputTokens_UnknownProtocol(t *testing.T) {
	body := []byte(`{"foo":"bar","baz":123}`)
	require.Equal(t, 0, EstimateInputTokens(body))
}

func TestEstimateInputTokens_EmptyBody(t *testing.T) {
	require.Equal(t, 0, EstimateInputTokens(nil))
	require.Equal(t, 0, EstimateInputTokens([]byte(``)))
	require.Equal(t, 0, EstimateInputTokens([]byte(`{}`)))
}

func TestEstimateInputTokens_PureCJK(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"` + strings.Repeat("你", 3000) + `"}]}`)
	require.Equal(t, 2000, EstimateInputTokens(body))
}

func TestEstimateInputTokens_PureEnglish(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"` + strings.Repeat("a", 4000) + `"}]}`)
	require.Equal(t, 1000, EstimateInputTokens(body))
}

func TestEstimateInputTokens_MixedText(t *testing.T) {
	// 2000 CJK + 2000 nonCJK → 2000*2/3 + 2000/4 = 1333 + 500 = 1833
	body := []byte(`{"messages":[{"role":"user","content":"` + strings.Repeat("你", 2000) + strings.Repeat("a", 2000) + `"}]}`)
	require.Equal(t, 1833, EstimateInputTokens(body))
}

func TestEstimateInputTokens_DoesNotPanicOnInvalidJSON(t *testing.T) {
	body := []byte(`{not-valid-json`)
	// 不应 panic，结果可以是 0
	_ = EstimateInputTokens(body)
}
