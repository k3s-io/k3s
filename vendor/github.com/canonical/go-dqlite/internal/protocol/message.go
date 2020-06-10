package protocol

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

// NamedValues is a type alias of a slice of driver.NamedValue. It's used by
// schema.sh to generate encoding logic for statement parameters.
type NamedValues = []driver.NamedValue

// Nodes is a type alias of a slice of NodeInfo. It's used by schema.sh to
// generate decoding logic for the heartbeat response.
type Nodes []NodeInfo

// Message holds data about a single request or response.
type Message struct {
	words  uint32
	mtype  uint8
	flags  uint8
	extra  uint16
	header []byte // Statically allocated header buffer
	body   buffer // Message body data.
}

// Init initializes the message using the given initial size for the data
// buffer, which is re-used across requests or responses encoded or decoded
// using this message object.
func (m *Message) Init(initialBufferSize int) {
	if (initialBufferSize % messageWordSize) != 0 {
		panic("initial buffer size is not aligned to word boundary")
	}
	m.header = make([]byte, messageHeaderSize)
	m.body.Bytes = make([]byte, initialBufferSize)
	m.reset()
}

// Reset the state of the message so it can be used to encode or decode again.
func (m *Message) reset() {
	m.words = 0
	m.mtype = 0
	m.flags = 0
	m.extra = 0
	for i := 0; i < messageHeaderSize; i++ {
		m.header[i] = 0
	}
	m.body.Offset = 0
}

// Append a byte slice to the message.
func (m *Message) putBlob(v []byte) {
	size := len(v)
	m.putUint64(uint64(size))

	pad := 0
	if (size % messageWordSize) != 0 {
		// Account for padding
		pad = messageWordSize - (size % messageWordSize)
		size += pad
	}

	b := m.bufferForPut(size)
	defer b.Advance(size)

	// Copy the bytes into the buffer.
	offset := b.Offset
	copy(b.Bytes[offset:], v)
	offset += len(v)

	// Add padding
	for i := 0; i < pad; i++ {
		b.Bytes[offset] = 0
		offset++
	}
}

// Append a string to the message.
func (m *Message) putString(v string) {
	size := len(v) + 1
	pad := 0
	if (size % messageWordSize) != 0 {
		// Account for padding
		pad = messageWordSize - (size % messageWordSize)
		size += pad
	}

	b := m.bufferForPut(size)
	defer b.Advance(size)

	// Copy the string bytes into the buffer.
	offset := b.Offset
	copy(b.Bytes[offset:], v)
	offset += len(v)

	// Add a nul byte
	b.Bytes[offset] = 0
	offset++

	// Add padding
	for i := 0; i < pad; i++ {
		b.Bytes[offset] = 0
		offset++
	}
}

// Append a byte to the message.
func (m *Message) putUint8(v uint8) {
	b := m.bufferForPut(1)
	defer b.Advance(1)

	b.Bytes[b.Offset] = v
}

// Append a 2-byte word to the message.
func (m *Message) putUint16(v uint16) {
	b := m.bufferForPut(2)
	defer b.Advance(2)

	binary.LittleEndian.PutUint16(b.Bytes[b.Offset:], v)
}

// Append a 4-byte word to the message.
func (m *Message) putUint32(v uint32) {
	b := m.bufferForPut(4)
	defer b.Advance(4)

	binary.LittleEndian.PutUint32(b.Bytes[b.Offset:], v)
}

// Append an 8-byte word to the message.
func (m *Message) putUint64(v uint64) {
	b := m.bufferForPut(8)
	defer b.Advance(8)

	binary.LittleEndian.PutUint64(b.Bytes[b.Offset:], v)
}

// Append a signed 8-byte word to the message.
func (m *Message) putInt64(v int64) {
	b := m.bufferForPut(8)
	defer b.Advance(8)

	binary.LittleEndian.PutUint64(b.Bytes[b.Offset:], uint64(v))
}

// Append a floating point number to the message.
func (m *Message) putFloat64(v float64) {
	b := m.bufferForPut(8)
	defer b.Advance(8)

	binary.LittleEndian.PutUint64(b.Bytes[b.Offset:], math.Float64bits(v))
}

// Encode the given driver values as binding parameters.
func (m *Message) putNamedValues(values NamedValues) {
	n := uint8(len(values)) // N of params
	if n == 0 {
		return
	}

	m.putUint8(n)

	for i := range values {
		if values[i].Ordinal != i+1 {
			panic("unexpected ordinal")
		}

		switch values[i].Value.(type) {
		case int64:
			m.putUint8(Integer)
		case float64:
			m.putUint8(Float)
		case bool:
			m.putUint8(Boolean)
		case []byte:
			m.putUint8(Blob)
		case string:
			m.putUint8(Text)
		case nil:
			m.putUint8(Null)
		case time.Time:
			m.putUint8(ISO8601)
		default:
			panic("unsupported value type")
		}
	}

	b := m.bufferForPut(1)

	if trailing := b.Offset % messageWordSize; trailing != 0 {
		// Skip padding bytes
		b.Advance(messageWordSize - trailing)
	}

	for i := range values {
		switch v := values[i].Value.(type) {
		case int64:
			m.putInt64(v)
		case float64:
			m.putFloat64(v)
		case bool:
			if v {
				m.putUint64(1)
			} else {
				m.putUint64(0)
			}
		case []byte:
			m.putBlob(v)
		case string:
			m.putString(v)
		case nil:
			m.putInt64(0)
		case time.Time:
			timestamp := v.Format(iso8601Formats[0])
			m.putString(timestamp)
		default:
			panic("unsupported value type")
		}
	}

}

// Finalize the message by setting the message type and the number
// of words in the body (calculated from the body size).
func (m *Message) putHeader(mtype uint8) {
	if m.body.Offset <= 0 {
		panic("static offset is not positive")
	}

	if (m.body.Offset % messageWordSize) != 0 {
		panic("static body is not aligned")
	}

	m.mtype = mtype
	m.flags = 0
	m.extra = 0

	m.words = uint32(m.body.Offset) / messageWordSize

	m.finalize()
}

func (m *Message) finalize() {
	if m.words == 0 {
		panic("empty message body")
	}

	binary.LittleEndian.PutUint32(m.header[0:], m.words)
	m.header[4] = m.mtype
	m.header[5] = m.flags
	binary.LittleEndian.PutUint16(m.header[6:], m.extra)
}

func (m *Message) bufferForPut(size int) *buffer {
	for (m.body.Offset + size) > len(m.body.Bytes) {
		// Grow message buffer.
		bytes := make([]byte, len(m.body.Bytes)*2)
		copy(bytes, m.body.Bytes)
		m.body.Bytes = bytes
	}

	return &m.body
}

// Return the message type and its flags.
func (m *Message) getHeader() (uint8, uint8) {
	return m.mtype, m.flags
}

// Read a string from the message body.
func (m *Message) getString() string {
	b := m.bufferForGet()

	index := bytes.IndexByte(b.Bytes[b.Offset:], 0)
	if index == -1 {
		panic("no string found")
	}
	s := string(b.Bytes[b.Offset : b.Offset+index])

	index++

	if trailing := index % messageWordSize; trailing != 0 {
		// Account for padding, moving index to the next word boundary.
		index += messageWordSize - trailing
	}

	b.Advance(index)

	return s
}

func (m *Message) getBlob() []byte {
	size := m.getUint64()
	data := make([]byte, size)
	for i := range data {
		data[i] = m.getUint8()
	}
	pad := 0
	if (size % messageWordSize) != 0 {
		// Account for padding
		pad = int(messageWordSize - (size % messageWordSize))
	}
	// Consume padding
	for i := 0; i < pad; i++ {
		m.getUint8()
	}
	return data
}

// Read a byte from the message body.
func (m *Message) getUint8() uint8 {
	b := m.bufferForGet()
	defer b.Advance(1)

	return b.Bytes[b.Offset]
}

// Read a 2-byte word from the message body.
func (m *Message) getUint16() uint16 {
	b := m.bufferForGet()
	defer b.Advance(2)

	return binary.LittleEndian.Uint16(b.Bytes[b.Offset:])
}

// Read a 4-byte word from the message body.
func (m *Message) getUint32() uint32 {
	b := m.bufferForGet()
	defer b.Advance(4)

	return binary.LittleEndian.Uint32(b.Bytes[b.Offset:])
}

// Read reads an 8-byte word from the message body.
func (m *Message) getUint64() uint64 {
	b := m.bufferForGet()
	defer b.Advance(8)

	return binary.LittleEndian.Uint64(b.Bytes[b.Offset:])
}

// Read a signed 8-byte word from the message body.
func (m *Message) getInt64() int64 {
	b := m.bufferForGet()
	defer b.Advance(8)

	return int64(binary.LittleEndian.Uint64(b.Bytes[b.Offset:]))
}

// Read a floating point number from the message body.
func (m *Message) getFloat64() float64 {
	b := m.bufferForGet()
	defer b.Advance(8)

	return math.Float64frombits(binary.LittleEndian.Uint64(b.Bytes[b.Offset:]))
}

// Decode a list of server objects from the message body.
func (m *Message) getNodes() Nodes {
	n := m.getUint64()
	servers := make(Nodes, n)

	for i := 0; i < int(n); i++ {
		servers[i].ID = m.getUint64()
		servers[i].Address = m.getString()
		servers[i].Role = NodeRole(m.getUint64())
	}

	return servers
}

// Decode a statement result object from the message body.
func (m *Message) getResult() Result {
	return Result{
		LastInsertID: m.getUint64(),
		RowsAffected: m.getUint64(),
	}
}

// Decode a query result set object from the message body.
func (m *Message) getRows() Rows {
	// Read the column count and column names.
	columns := make([]string, m.getUint64())

	for i := range columns {
		columns[i] = m.getString()
	}

	rows := Rows{
		Columns: columns,
		message: m,
	}
	return rows
}

func (m *Message) getFiles() Files {
	files := Files{
		n:       m.getUint64(),
		message: m,
	}
	return files
}

func (m *Message) hasBeenConsumed() bool {
	size := int(m.words * messageWordSize)
	return m.body.Offset == size
}

func (m *Message) lastByte() byte {
	size := int(m.words * messageWordSize)
	return m.body.Bytes[size-1]
}

func (m *Message) bufferForGet() *buffer {
	size := int(m.words * messageWordSize)
	// The static body has been exahusted, use the dynamic one.
	if m.body.Offset == size {
		err := fmt.Errorf("short message: type=%d words=%d off=%d", m.mtype, m.words, m.body.Offset)
		panic(err)
	}

	return &m.body
}

// Result holds the result of a statement.
type Result struct {
	LastInsertID uint64
	RowsAffected uint64
}

// Rows holds a result set encoded in a message body.
type Rows struct {
	Columns []string
	message *Message
	types   []uint8
}

// columnTypes returns the row's column types
// if save is true, it will restore the buffer offset
func (r *Rows) columnTypes(save bool) ([]uint8, error) {
	// use cached values if possible if not advancing the buffer offset
	if save && r.types != nil {
		return r.types, nil
	}
	// column types should never change between rows
	// use cached copy to allow getting types when no more rows
	if r.types == nil {
		r.types = make([]uint8, len(r.Columns))
	}

	// Each column needs a 4 byte slot to store the column type. The row
	// header must be padded to reach word boundary.
	headerBits := len(r.types) * 4
	padBits := 0
	if trailingBits := (headerBits % messageWordBits); trailingBits != 0 {
		padBits = (messageWordBits - trailingBits)
	}

	headerSize := (headerBits + padBits) / messageWordBits * messageWordSize

	for i := 0; i < headerSize; i++ {
		slot := r.message.getUint8()

		if slot == 0xee {
			// More rows are available.
			if save {
				r.message.bufferForGet().Advance(-(i + 1))
			}
			return r.types, ErrRowsPart
		}

		if slot == 0xff {
			// Rows EOF marker
			if save {
				r.message.bufferForGet().Advance(-(i + 1))
			}
			return r.types, io.EOF
		}

		index := i * 2

		if index >= len(r.types) {
			continue // This is padding.
		}

		r.types[index] = slot & 0x0f

		index++

		if index >= len(r.types) {
			continue // This is padding byte.
		}

		r.types[index] = slot >> 4
	}
	if save {
		r.message.bufferForGet().Advance(-headerSize)
	}
	return r.types, nil
}

// Next returns the next row in the result set.
func (r *Rows) Next(dest []driver.Value) error {
	types, err := r.columnTypes(false)
	if err != nil {
		return err
	}

	for i := range types {
		switch types[i] {
		case Integer:
			dest[i] = r.message.getInt64()
		case Float:
			dest[i] = r.message.getFloat64()
		case Blob:
			dest[i] = r.message.getBlob()
		case Text:
			dest[i] = r.message.getString()
		case Null:
			r.message.getUint64()
			dest[i] = nil
		case UnixTime:
			timestamp := time.Unix(r.message.getInt64(), 0)
			dest[i] = timestamp
		case ISO8601:
			value := r.message.getString()
			if value == "" {
				dest[i] = time.Time{}
				break
			}
			var t time.Time
			var timeVal time.Time
			var err error
			value = strings.TrimSuffix(value, "Z")
			for _, format := range iso8601Formats {
				if timeVal, err = time.ParseInLocation(format, value, time.UTC); err == nil {
					t = timeVal
					break
				}
			}
			if err != nil {
				return err
			}
			t = t.In(time.Local)
			dest[i] = t
		case Boolean:
			dest[i] = r.message.getInt64() != 0
		default:
			panic("unknown data type")
		}
	}

	return nil
}

// Close the result set and reset the underlying message.
func (r *Rows) Close() error {
	// If we didn't go through all rows, let's look at the last byte.
	var err error
	if !r.message.hasBeenConsumed() {
		slot := r.message.lastByte()
		if slot == 0xee {
			// More rows are available.
			err = ErrRowsPart
		} else if slot == 0xff {
			// Rows EOF marker
			err = io.EOF
		} else {
			err = fmt.Errorf("unexpected end of message")
		}
	}
	r.message.reset()
	return err
}

// Files holds a set of files encoded in a message body.
type Files struct {
	n       uint64
	message *Message
}

func (f *Files) Next() (string, []byte) {
	if f.n == 0 {
		return "", nil
	}
	f.n--
	name := f.message.getString()
	length := f.message.getUint64()
	data := make([]byte, length)
	for i := 0; i < int(length); i++ {
		data[i] = f.message.getUint8()
	}
	return name, data
}

func (f *Files) Close() {
	f.message.reset()
}

const (
	messageWordSize                 = 8
	messageWordBits                 = messageWordSize * 8
	messageHeaderSize               = messageWordSize
	messageMaxConsecutiveEmptyReads = 100
)

var iso8601Formats = []string{
	// By default, store timestamps with whatever timezone they come with.
	// When parsed, they will be returned with the same timezone.
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

// ColumnTypes returns the column types for the the result set.
func (r *Rows) ColumnTypes() ([]string, error) {
	types, err := r.columnTypes(true)
	kinds := make([]string, len(types))

	for i, t := range types {
		switch t {
		case Integer:
			kinds[i] = "INTEGER"
		case Float:
			kinds[i] = "FLOAT"
		case Blob:
			kinds[i] = "BLOB"
		case Text:
			kinds[i] = "TEXT"
		case Null:
			kinds[i] = "NULL"
		case UnixTime:
			kinds[i] = "TIME"
		case ISO8601:
			kinds[i] = "TIME"
		case Boolean:
			kinds[i] = "BOOL"
		default:
			return nil, fmt.Errorf("unknown data type: %d", t)
		}
	}

	return kinds, err
}
