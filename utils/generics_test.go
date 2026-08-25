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

func TestAdvancedDivide(t *testing.T) {
	s1 := AdvancedDivide(0, 0)
	assert.Equal(t, 0, s1)
	s2 := AdvancedDivide(1, 0)
	assert.Equal(t, 0, s2)
	s3 := AdvancedDivide(1, 1)
	assert.Equal(t, 1, s3)
}
