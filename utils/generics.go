package utils

// Map like map in Python
func Map[T1, T2 any](slice []T1, f func(T1) T2) []T2 {
	result := make([]T2, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

// Any returns true if any element in slice meets the requirement
func Any[T any](slice []T, f func(T) bool) bool {
	for _, v := range slice {
		if f(v) {
			return true
		}
	}
	return false
}

// Filter returns a new slice with elements that meet the requirement
func Filter[T any](slice []T, f func(T) bool) []T {
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
	for i := 0; i < l; i++ {
		result[i] = factory()
	}
	return result
}

type addable interface {
	~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64 | ~float32 | ~float64 | ~complex64 | ~complex128
}

// Sum returns sum of all elements in slice
func Sum[T addable](slice []T) T {
	var result T
	for _, v := range slice {
		result += v
	}
	return result
}

type dividable interface {
	~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64 | ~float32 | ~float64
}

// AdvancedDivide returns 0 when 0/0
func AdvancedDivide[T dividable](a, b T) T {
	if a == 0 || b == 0 {
		return 0
	}
	return a / b
}
