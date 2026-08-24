package utils

import (
	"cmp"
	"slices"
)

type numbers interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 |
		~complex64 | ~complex128
}

// Map returns f applied to every element of slice.
func Map[T1, T2 any](slice []T1, f func(T1) T2) []T2 {
	result := make([]T2, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

func Min[T cmp.Ordered](x T, xs ...T) T {
	if len(xs) == 0 {
		return x
	}
	if m := Min(xs[0], xs[1:]...); m < x {
		return m
	}
	return x
}

func Max[T cmp.Ordered](x T, xs ...T) T {
	if len(xs) == 0 {
		return x
	}
	if m := Max(xs[0], xs[1:]...); m > x {
		return m
	}
	return x
}

// Filter returns a new slice of the elements satisfying f, or nil when slice is nil.
func Filter[T any](slice []T, f func(T) bool) []T {
	if slice == nil {
		return slice
	}
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if f(v) {
			result = append(result, v)
		}
	}
	return result
}

// AdvancedDivide returns 0 when either operand is 0.
func AdvancedDivide[T numbers](a, b T) T {
	if a == 0 || b == 0 {
		return 0
	}
	return a / b
}

func Reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Unique sorts s in place and returns n, where s[:n] holds the distinct elements.
func Unique[T, E cmp.Ordered](s []T, getVal func(int) E) (j int) {
	slices.Sort(s)
	var lastVal E
	for i := range s {
		if getVal(i) == lastVal && i != 0 {
			continue
		}
		lastVal = getVal(i)
		s[i], s[j] = s[j], s[i]
		j++
	}
	return j
}
