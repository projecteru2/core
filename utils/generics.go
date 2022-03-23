package utils

// Map like map in Python
func Map[T1, T2 any](slice []T1, f func(T1) T2) []T2 {
	result := make([]T2, len(slice))
	for i, v := range slice {
		result[i] = f(v)
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
