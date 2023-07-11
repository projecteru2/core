package utils

import (
	"fmt"
	"reflect"
	"strconv"
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

func TestFilter(t *testing.T) {
	var s0 []int
	s0 = Filter(s0, func(int) bool { return true })
	assert.Nil(t, s0)

	s1 := GenerateSlice(5, func() int {
		return 0
	})
	s2 := Filter(s1, func(int) bool { return true })
	assert.Len(t, s2, 5)
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

func TestReverse(t *testing.T) {
	s1 := []string{"a", "b", "c"}
	Reverse(s1)
	assert.True(t, reflect.DeepEqual(s1, []string{"c", "b", "a"}))

	s2 := []string{}
	Reverse(s2)

	s3 := []int{1, 2, 3, 4}
	Reverse(s3)
	assert.True(t, reflect.DeepEqual(s3, []int{4, 3, 2, 1}))
}

func TestUnique(t *testing.T) {
	s1 := []int64{1, 2, 3}
	s1 = s1[:Unique(s1, func(i int) string { return strconv.Itoa(int(s1[i])) })]
	assert.True(t, reflect.DeepEqual(s1, []int64{1, 2, 3}))

	s2 := []string{"a", "a", "a", "b", "b", "c"}
	s2 = s2[:Unique(s2, func(i int) string { return s2[i] })]
	assert.True(t, reflect.DeepEqual(s2, []string{"a", "b", "c"}))

	s3 := []string{"", "1", "1", "1", "1"}
	s3 = s3[:Unique(s3, func(i int) string { return s3[i] })]
	assert.True(t, reflect.DeepEqual(s3, []string{"", "1"}))
}
