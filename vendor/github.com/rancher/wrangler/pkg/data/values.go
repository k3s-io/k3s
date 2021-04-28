package data

func RemoveValue(data map[string]interface{}, keys ...string) (interface{}, bool) {
	for i, key := range keys {
		if i == len(keys)-1 {
			val, ok := data[key]
			delete(data, key)
			return val, ok
		}
		data, _ = data[key].(map[string]interface{})
	}

	return nil, false
}

func GetValueN(data map[string]interface{}, keys ...string) interface{} {
	val, _ := GetValue(data, keys...)
	return val
}

func GetValue(data map[string]interface{}, keys ...string) (interface{}, bool) {
	for i, key := range keys {
		if i == len(keys)-1 {
			val, ok := data[key]
			return val, ok
		}
		data, _ = data[key].(map[string]interface{})
	}

	return nil, false
}

func PutValue(data map[string]interface{}, val interface{}, keys ...string) {
	if data == nil {
		return
	}

	// This is so ugly
	for i, key := range keys {
		if i == len(keys)-1 {
			data[key] = val
		} else {
			newData, ok := data[key]
			if ok {
				newMap, ok := newData.(map[string]interface{})
				if ok {
					data = newMap
				} else {
					return
				}
			} else {
				newMap := map[string]interface{}{}
				data[key] = newMap
				data = newMap
			}
		}
	}
}
