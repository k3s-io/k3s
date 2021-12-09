//+build gofuzz

package genetlink

func Fuzz(data []byte) int {
	return fuzzMessage(data)
}

func fuzzMessage(data []byte) int {
	var m Message
	if err := (&m).UnmarshalBinary(data); err != nil {
		return 0
	}

	if _, err := m.MarshalBinary(); err != nil {
		panic(err)
	}

	return 1
}
