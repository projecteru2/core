package utils

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

// AdvancedDivide returns 0 when either operand is 0.
func AdvancedDivide[T numbers](a, b T) T {
	if a == 0 || b == 0 {
		return 0
	}
	return a / b
}
