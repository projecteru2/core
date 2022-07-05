package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	s1 := []int{1, 2, 3, 4, 5}
	s2 := Map(s1, func(e int) string {
		return fmt.Sprintf("%d", e)
	})
	assert.Equal(t, []string{"1", "2", "3", "4", "5"}, s2)
}

func TestSum(t *testing.T) {
	s1 := []int{1, 2, 3, 4, 5}
	s2 := Sum(s1)
	assert.Equal(t, 15, s2)
}

func TestMin(t *testing.T) {
	assert.Equal(t, 1, Min(1, 2, 3))
	assert.Equal(t, 1.0, Min(2.0, 1.0, 3.0))
	assert.Equal(t, uint32(1), Min(uint32(2), uint32(1), uint32(3)))
}

func TestMax(t *testing.T) {
	assert.Equal(t, 3, Max(1, 2, 3))
	assert.Equal(t, 3.0, Max(2.0, 1.0, 3.0))
	assert.Equal(t, uint32(3), Max(uint32(2), uint32(1), uint32(3)))
}
