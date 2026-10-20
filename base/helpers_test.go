package base

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkBy(t *testing.T) {
	tests := []struct {
		name      string
		items     []int
		chunkSize int
		expected  [][]int
	}{
		{"exact multiple", []int{1, 2, 3, 4}, 2, [][]int{{1, 2}, {3, 4}}},
		{"remainder", []int{1, 2, 3, 4, 5}, 2, [][]int{{1, 2}, {3, 4}, {5}}},
		{"fewer items than chunk size", []int{1, 2}, 6, [][]int{{1, 2}}},
		{"chunk size 1", []int{1, 2, 3}, 1, [][]int{{1}, {2}, {3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, chunkBy(tt.items, tt.chunkSize))
		})
	}
}
