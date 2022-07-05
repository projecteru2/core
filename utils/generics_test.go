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

func TestAny(t *testing.T) {
	s1 := []int{1, 2, 3, 4, 5}
	s2 := Any(s1, func(e int) bool {
		return e == 3
	})
	assert.True(t, s2)
	s1 = nil
	s2 = Any(s1, func(e int) bool {
		return e == 3
	})
	assert.False(t, s2)
}

func TestGenerateSlice(t *testing.T) {
	s1 := GenerateSlice(5, func() int {
		return 0
	})
	assert.Equal(t, []int{0, 0, 0, 0, 0}, s1)
	s2 := GenerateSlice(0, func() int {
		return 0
	})
	assert.Equal(t, []int{}, s2)
}

func TestAdvancedDivide(t *testing.T) {
	s1 := AdvancedDivide(0, 0)
	assert.Equal(t, 0, s1)
	s2 := AdvancedDivide(1, 0)
	assert.Equal(t, 0, s2)
	s3 := AdvancedDivide(1, 1)
	assert.Equal(t, 1, s3)
}
