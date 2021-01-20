package data

func MergeMaps(base, overlay map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		if baseMap, overlayMap, bothMaps := bothMaps(result[k], v); bothMaps {
			v = MergeMaps(baseMap, overlayMap)
		}
		result[k] = v
	}
	return result
}

func bothMaps(left, right interface{}) (map[string]interface{}, map[string]interface{}, bool) {
	leftMap, ok := left.(map[string]interface{})
	if !ok {
		return nil, nil, false
	}
	rightMap, ok := right.(map[string]interface{})
	return leftMap, rightMap, ok
}
