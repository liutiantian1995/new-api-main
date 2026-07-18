package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// parseCSVInts is the production helper under test.
func TestParseCSVInts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{"empty", "", nil},
		{"single", "5", []int{5}},
		{"multiple", "1,2,3", []int{1, 2, 3}},
		{"with whitespace", " 1 , 2 ", []int{1, 2}},
		{"with zeros", "0,1,0,2", []int{1, 2}},
		{"with invalid", "1,abc,3", []int{1, 3}},
		{"all invalid", "a,b,c", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCSVInts(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
