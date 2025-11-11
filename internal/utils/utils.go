package utils

func RemoveIndexFromSlice[T any](s []T, removeIdx int) []T {
	if removeIdx < 0 || removeIdx >= len(s) {
		return s
	}
	return append(s[:removeIdx], s[removeIdx+1:]...)
}
