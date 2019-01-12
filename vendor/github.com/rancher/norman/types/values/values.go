package values

import "github.com/rancher/norman/types/convert"

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

func GetStringSlice(data map[string]interface{}, keys ...string) ([]string, bool) {
	val, ok := GetValue(data, keys...)
	if !ok {
		return nil, ok
	}

	slice, typeOk := val.([]string)
	if typeOk {
		return slice, typeOk
	}

	sliceNext, typeOk := val.([]interface{})
	if !typeOk {
		return nil, typeOk
	}

	var result []string
	for _, item := range sliceNext {
		result = append(result, convert.ToString(item))
	}

	return result, true
}

func GetSlice(data map[string]interface{}, keys ...string) ([]map[string]interface{}, bool) {
	val, ok := GetValue(data, keys...)
	if !ok {
		return nil, ok
	}

	slice, typeOk := val.([]map[string]interface{})
	if typeOk {
		return slice, typeOk
	}

	sliceNext, typeOk := val.([]interface{})
	if !typeOk {
		return nil, typeOk
	}

	result := []map[string]interface{}{}
	for _, val := range sliceNext {
		if v, ok := val.(map[string]interface{}); ok {
			result = append(result, v)
		}
	}

	return result, true

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
