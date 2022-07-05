package utils

type addable interface {
	~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64 | ~float32 | ~float64 | ~complex64 | ~complex128
}

type ordered interface {
	~int | ~int32 | ~int64 | ~uint | ~uint32 | ~uint64 | ~float32 | ~float64
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
func Sum[T addable](slice []T) T {
	var result T
	for _, v := range slice {
		result += v
	}
	return result
}

// Min returns the lesser one.
func Min[T ordered](x T, xs ...T) T {
	if len(xs) == 0 {
		return x
	}
	if m := Min(xs[0], xs[1:]...); m < x {
		return m
	}
	return x
}

// Max returns the biggest one.
func Max[T ordered](x T, xs ...T) T {
	if len(xs) == 0 {
		return x
	}
	if m := Max(xs[0], xs[1:]...); m > x {
		return m
	}
	return x
}
