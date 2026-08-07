package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 构造一个 content 为 []ClaudeMediaMessage 的 ClaudeMessage。
func newContentMessage(role string, blocks ...ClaudeMediaMessage) ClaudeMessage {
	return ClaudeMessage{
		Role:    role,
		Content: blocks,
	}
}

func TestStripServerToolBlocks(t *testing.T) {
	textBlock := func(s string) ClaudeMediaMessage {
		return ClaudeMediaMessage{Type: ContentTypeText, Text: ptrString(s)}
	}
	toolUseBlock := func(id string) ClaudeMediaMessage {
		return ClaudeMediaMessage{Type: "server_tool_use", Id: id, Name: "web_search"}
	}
	toolResultBlock := func(id string) ClaudeMediaMessage {
		return ClaudeMediaMessage{Type: "server_tool_result", ToolUseId: id}
	}
	clientToolUse := func(id string) ClaudeMediaMessage {
		return ClaudeMediaMessage{Type: "tool_use", Id: id, Name: "get_weather"}
	}

	tests := []struct {
		name           string
		messages       []ClaudeMessage
		wantRemoved    int
		wantMsgCount   int
		wantLastText   string // 验证最后一条 message 的首个 text 块内容（空表示不校验）
		wantFirstTypes []string
	}{
		{
			name: "string content unchanged",
			messages: []ClaudeMessage{
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "hi"},
			},
			wantRemoved:  0,
			wantMsgCount: 2,
		},
		{
			name: "pure server_tool_use message dropped",
			messages: []ClaudeMessage{
				{Role: "user", Content: "hi"},
				newContentMessage("assistant",
					toolUseBlock("srvu_1"),
				),
				{Role: "user", Content: "next"},
			},
			wantRemoved:  1,
			wantMsgCount: 2, // assistant 那条被整条丢弃
		},
		{
			name: "mixed content partial filter",
			messages: []ClaudeMessage{
				newContentMessage("assistant",
					textBlock("thinking..."),
					toolUseBlock("srvu_1"),
					textBlock("after"),
				),
			},
			wantRemoved:    1,
			wantMsgCount:   1,
			wantFirstTypes: []string{"text", "text"},
		},
		{
			name: "server_tool_result also filtered",
			messages: []ClaudeMessage{
				newContentMessage("user",
					toolResultBlock("srvu_1"),
					textBlock("real question"),
				),
			},
			wantRemoved:    1,
			wantMsgCount:   1,
			wantFirstTypes: []string{"text"},
		},
		{
			name: "client tool_use preserved",
			messages: []ClaudeMessage{
				newContentMessage("assistant",
					clientToolUse("call_1"),
					toolUseBlock("srvu_1"),
				),
				newContentMessage("user",
					toolResultBlock("srvu_1"),
					textBlock("ok"),
				),
			},
			wantRemoved:    2,
			wantMsgCount:   2,
			wantFirstTypes: []string{"tool_use"},
		},
		{
			name:         "empty messages",
			messages:     nil,
			wantRemoved:  0,
			wantMsgCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ClaudeRequest{Messages: tt.messages}
			got := req.StripServerToolBlocks()
			assert.Equal(t, tt.wantRemoved, got, "removed count mismatch")
			require.Len(t, req.Messages, tt.wantMsgCount, "message count mismatch")

			if tt.wantFirstTypes != nil && len(req.Messages) > 0 {
				content, err := req.Messages[0].ParseContent()
				require.NoError(t, err)
				gotTypes := make([]string, 0, len(content))
				for _, m := range content {
					gotTypes = append(gotTypes, m.Type)
				}
				assert.Equal(t, tt.wantFirstTypes, gotTypes, "first message types mismatch")
			}

			// 不应残留任何 server_tool_* 块
			for i, msg := range req.Messages {
				if msg.IsStringContent() {
					continue
				}
				content, err := msg.ParseContent()
				require.NoError(t, err, "msg#%d parse failed", i)
				for j, m := range content {
					if m.Type == "server_tool_use" || m.Type == "server_tool_result" {
						t.Errorf("msg#%d block#%d still has type %q", i, j, m.Type)
					}
				}
			}
		})
	}
}

func TestStripServerToolBlocks_NilReceiver(t *testing.T) {
	var req *ClaudeRequest
	assert.NotPanics(t, func() {
		got := req.StripServerToolBlocks()
		assert.Equal(t, 0, got)
	})
}

func TestStripServerToolBlocks_RoundTripJSON(t *testing.T) {
	// 验证过滤后的 request 仍然能正常 marshal/unmarshal
	req := &ClaudeRequest{
		Model: "claude-test",
		Messages: []ClaudeMessage{
			newContentMessage("assistant",
				ClaudeMediaMessage{Type: ContentTypeText, Text: ptrString("hello")},
				ClaudeMediaMessage{Type: "server_tool_use", Id: "srvu_1", Name: "web_search"},
			),
		},
	}
	removed := req.StripServerToolBlocks()
	require.Equal(t, 1, removed)

	data, err := common.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"text":"hello"`)
	assert.NotContains(t, string(data), "server_tool_use")
}

// ptrString 返回字符串指针，方便构造 text 块。
func ptrString(s string) *string { return &s }
