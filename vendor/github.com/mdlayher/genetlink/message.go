package genetlink

import "errors"

// errInvalidMessage is returned when a Message is malformed.
var errInvalidMessage = errors.New("generic netlink message is invalid or too short")

// A Header is a generic netlink header. A Header is sent and received with
// each generic netlink message to indicate metadata regarding a Message.
type Header struct {
	// Command specifies a command to issue to netlink.
	Command uint8

	// Version specifies the version of a command to use.
	Version uint8
}

// headerLen is the length of a Header.
const headerLen = 4 // unix.GENL_HDRLEN

// A Message is a generic netlink message. It contains a Header and an
// arbitrary byte payload, which may be decoded using information from the
// Header.
//
// Data is encoded using the native endianness of the host system. Use
// the netlink.AttributeDecoder and netlink.AttributeEncoder types to decode
// and encode data.
type Message struct {
	Header Header
	Data   []byte
}

// MarshalBinary marshals a Message into a byte slice.
func (m Message) MarshalBinary() ([]byte, error) {
	b := make([]byte, headerLen)

	b[0] = m.Header.Command
	b[1] = m.Header.Version

	// b[2] and b[3] are padding bytes and set to zero

	return append(b, m.Data...), nil
}

// UnmarshalBinary unmarshals the contents of a byte slice into a Message.
func (m *Message) UnmarshalBinary(b []byte) error {
	if len(b) < headerLen {
		return errInvalidMessage
	}

	// Don't allow reserved pad bytes to be set
	if b[2] != 0 || b[3] != 0 {
		return errInvalidMessage
	}

	m.Header.Command = b[0]
	m.Header.Version = b[1]

	m.Data = b[4:]
	return nil
}
