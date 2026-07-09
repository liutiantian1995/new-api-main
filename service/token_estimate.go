package service

import (
	"github.com/tidwall/gjson"
)

// EstimateInputTokens 对请求体做字符近似 token 估算。
//
// 支持的协议（按字段优先级解析）：
//   - OpenAI Chat / Anthropic Messages: messages[*].content (string 或 array of {type:"text", text:"..."})
//   - OpenAI Completions:               prompt (string 或 array)
//   - Gemini:                           contents[*].parts[*].text
//   - Embeddings:                       input (string 或 array)
//   - OpenAI Responses API:             instructions (string) + input (string 或 message array with nested content)
//
// 算法：非 CJK 4 字符 ≈ 1 token，CJK 1.5 字符 ≈ 1 token。未知协议或空 body 返回 0。
// 不修改 body、不报错，纯函数无副作用。
func EstimateInputTokens(body []byte) int {
	if len(body) == 0 {
		return 0
	}
	text := extractTextForEstimation(body)
	if text == "" {
		return 0
	}
	return approximateInputTokens(text)
}

// extractTextForEstimation 按协议字段尝试解析，返回所有命中字段拼接的文本。
// 同一 body 命中多种字段时全部累加（实际多协议互斥，不会重复）。
func extractTextForEstimation(body []byte) string {
	var sb []byte

	// OpenAI Chat / Anthropic Messages: messages[*].content
	messages := gjson.GetBytes(body, "messages")
	if messages.IsArray() {
		messages.ForEach(func(_, msg gjson.Result) bool {
			content := msg.Get("content")
			if !content.Exists() {
				return true
			}
			sb = appendContentText(sb, content)
			return true
		})
	}

	// OpenAI Completions: prompt (string 或 array)
	prompt := gjson.GetBytes(body, "prompt")
	if prompt.Exists() {
		sb = appendContentText(sb, prompt)
	}

	// Gemini: contents[*].parts[*].text
	contents := gjson.GetBytes(body, "contents")
	if contents.IsArray() {
		contents.ForEach(func(_, content gjson.Result) bool {
			parts := content.Get("parts")
			if !parts.IsArray() {
				return true
			}
			parts.ForEach(func(_, part gjson.Result) bool {
				t := part.Get("text")
				if t.Type == gjson.String {
					sb = append(sb, t.String()...)
				}
				return true
			})
			return true
		})
	}

	// OpenAI Responses API: instructions (string)
	instructions := gjson.GetBytes(body, "instructions")
	if instructions.Type == gjson.String {
		sb = append(sb, instructions.String()...)
	}

	// Embeddings / OpenAI Responses API: input
	// Embeddings input: string or array of strings
	// Responses API input: string, or array of message objects with nested
	// content[].text (e.g. {type:"message", content:[{type:"input_text", text:"..."}]})
	input := gjson.GetBytes(body, "input")
	if input.Exists() {
		sb = appendResponsesInputText(sb, input)
	}

	return string(sb)
}

// appendResponsesInputText extracts text from an `input` field that may be a
// plain string, a flat array of strings/objects, or a Responses API array of
// message objects with nested content.
func appendResponsesInputText(buf []byte, v gjson.Result) []byte {
	switch {
	case v.Type == gjson.String:
		return append(buf, v.String()...)
	case v.IsArray():
		v.ForEach(func(_, item gjson.Result) bool {
			if item.Type == gjson.String {
				buf = append(buf, item.String()...)
				return true
			}
			if item.IsObject() {
				// Direct {type:"text"/"input_text", text:"..."}
				if t := item.Get("text"); t.Type == gjson.String {
					buf = append(buf, t.String()...)
				}
				// Nested message: {type:"message", content:[{type:"input_text", text:"..."}]}
				content := item.Get("content")
				if content.IsArray() {
					content.ForEach(func(_, c gjson.Result) bool {
						if t := c.Get("text"); t.Type == gjson.String {
							buf = append(buf, t.String()...)
						}
						return true
					})
				}
			}
			return true
		})
	}
	return buf
}

// appendContentText 把 gjson 结果（可能是 string、array of {type:"text",text:"..."} 或 array of string）的文本部分追加到 buf。
func appendContentText(buf []byte, v gjson.Result) []byte {
	switch {
	case v.Type == gjson.String:
		return append(buf, v.String()...)
	case v.IsArray():
		v.ForEach(func(_, item gjson.Result) bool {
			if item.Type == gjson.String {
				buf = append(buf, item.String()...)
				return true
			}
			// 对象形式：{type:"text", text:"..."}，仅累加 text 部分
			if item.IsObject() {
				if t := item.Get("text"); t.Type == gjson.String {
					buf = append(buf, t.String()...)
				}
			}
			return true
		})
	}
	return buf
}

// approximateInputTokens 字符近似法：非 CJK 4 字符 ≈ 1 token，CJK 1.5 字符 ≈ 1 token。
// 复用 service 已有的 isCJK 判定。实现：cjk/1.5 + nonCJK/4，向下取整。
func approximateInputTokens(s string) int {
	var cjk, nonCJK int
	for _, r := range s {
		if isCJK(r) {
			cjk++
		} else {
			nonCJK++
		}
	}
	// cjk / 1.5 = cjk * 2 / 3
	return (cjk * 2 / 3) + (nonCJK / 4)
}
