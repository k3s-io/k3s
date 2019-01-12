package convert

const (
	ArrayKey = "{ARRAY}"
	MapKey   = "{MAP}"
)

type TransformerFunc func(input interface{}) interface{}

func Transform(data map[string]interface{}, path []string, transformer TransformerFunc) {
	if len(path) == 0 || len(data) == 0 {
		return
	}

	key := path[0]
	path = path[1:]
	value := data[key]

	if value == nil {
		return
	}

	if len(path) == 0 {
		data[key] = transformer(value)
		return
	}

	// You can't end a path with ARRAY/MAP.  Not supported right now
	if len(path) > 1 {
		switch path[0] {
		case ArrayKey:
			for _, valueMap := range ToMapSlice(value) {
				Transform(valueMap, path[1:], transformer)
			}
			return
		case MapKey:
			for _, valueMap := range ToMapInterface(value) {
				Transform(ToMapInterface(valueMap), path[1:], transformer)
			}
			return
		}
	}

	Transform(ToMapInterface(value), path, transformer)
}
