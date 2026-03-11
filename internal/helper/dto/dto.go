package helper

func MapToDTO[S any, Target any](source []S, f func(S) Target) []Target {
	result := make([]Target, len(source))
	for i, v := range source {
		result[i] = f(v)
	}
	return result
}
