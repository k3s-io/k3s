package protocol

// Buffer for reading responses or writing requests.
type buffer struct {
	Bytes  []byte
	Offset int
}

func (b *buffer) Advance(amount int) {
	b.Offset += amount
}
