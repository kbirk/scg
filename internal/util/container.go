package util

func MergeMap[K comparable, V any](dest map[K]V, src map[K]V) map[K]V {
	for k, v := range src {
		dest[k] = v
	}
	return dest
}

func RemoveDuplicates[T comparable](slice []T) []T {
	seen := make(map[T]struct{})
	j := 0
	for _, v := range slice {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		slice[j] = v
		j++
	}
	return slice[:j]
}
