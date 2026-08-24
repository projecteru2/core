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

// Map like map in Python
func Map[T1, T2 any](slice []T1, f func(T1) T2) []T2 {
	result := make([]T2, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

// Sum returns sum of all elements in slice
func Sum[T numbers](slice []T) T {
	var result T
	for _, v := range slice {
		result += v
	}
	return result
}

// Min returns the lesser one.
func Min[T cmp.Ordered](x T, xs ...T) T {
	if len(xs) == 0 {
		return x
	}
	if m := Min(xs[0], xs[1:]...); m < x {
		return m
	}
	return x
}

// Max returns the biggest one.
func Max[T cmp.Ordered](x T, xs ...T) T {
	if len(xs) == 0 {
		return x
	}
	if m := Max(xs[0], xs[1:]...); m > x {
		return m
	}
	return x
}

// Any returns true if any element in slice meets the requirement
func Any[T any](slice []T, f func(T) bool) bool {
	return slices.ContainsFunc(slice, f)
}

// Filter returns a new slice with elements that meet the requirement
func Filter[T any](slice []T, f func(T) bool) []T {
	if slice == nil {
		return slice
	}
	result := make([]T, 0)
	for _, v := range slice {
		if f(v) {
			result = append(result, v)
		}
	}
	return result
}

// GenerateSlice generates a slice with factory function
func GenerateSlice[T any](l int, factory func() T) []T {
	result := make([]T, l)
	for i := range l {
		result[i] = factory()
	}
	return result
}

// AdvancedDivide returns 0 when 0/0
func AdvancedDivide[T numbers](a, b T) T {
	if a == 0 || b == 0 {
		return 0
	}
	return a / b
}

// Reverse any slice
func Reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Unique return a index, where s[:index] is a unique slice
// Unique requires sorted slice
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
