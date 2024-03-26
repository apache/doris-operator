package tools

func MergeMaps[K comparable, V any](maps ...map[K]V) map[K]V {
	result := maps[0]
	for i, m := range maps {
		if i != 0 {
			for k, v := range m {
				result[k] = v
			}
		}
	}
	return result
}

func IsElementInArray[T comparable](arr []T, element T) bool {
	for _, v := range arr {
		if v == element {
			return true
		}
	}
	return false
}
