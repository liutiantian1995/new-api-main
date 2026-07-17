package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatRollingDuration(t *testing.T) {
	tests := []struct {
		sec      int64
		expected string
	}{
		{18000, "5 小时"},
		{86400, "1 天"},
		{604800, "1 周"},
		{2592000, "30 天"},
		{3600, "1 小时"},
		{7200, "2 小时"},
		{90000, "25 小时"},
		{60, "1 小时"}, // rounds up sub-hour to 1 hour
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, FormatRollingDuration(tt.sec))
	}
}
