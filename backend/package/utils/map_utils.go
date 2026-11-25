package utils

func UnionOfMaps[K comparable, V any](ms ...map[K]V) (result map[K]V) {
	result = make(map[K]V)
	recoverAsResult(&result)
	if len(ms) == 0 {
		return result
	}

	for _, mm := range ms {
		for k, v := range mm {
			result[k] = v
		}
	}
	return result
}
