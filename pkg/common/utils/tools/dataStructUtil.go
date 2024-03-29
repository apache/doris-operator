package tools

// IsElementInArray Determine whether the element exists in the array
func IsElementInArray[T comparable](arr []T, element T) bool {
	for _, v := range arr {
		if v == element {
			return true
		}
	}
	return false
}
