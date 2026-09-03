package utils

// Map returns f applied to every element of slice.
func Map[T1, T2 any](slice []T1, f func(T1) T2) []T2 {
	result := make([]T2, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

// AdvancedDivide returns 0 instead of dividing by zero.
func AdvancedDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
