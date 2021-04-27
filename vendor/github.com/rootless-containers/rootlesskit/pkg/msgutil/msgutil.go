// Package msgutil provides utility for JSON message with uint32le header
package msgutil

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
)

const (
	maxLength = 1 << 16
)

func MarshalToWriter(w io.Writer, x interface{}) (int, error) {
	b, err := json.Marshal(x)
	if err != nil {
		return 0, err
	}
	if len(b) > maxLength {
		return 0, errors.Errorf("bad message length: %d (max: %d)", len(b), maxLength)
	}
	h := make([]byte, 4)
	binary.LittleEndian.PutUint32(h, uint32(len(b)))
	return w.Write(append(h, b...))
}

func UnmarshalFromReader(r io.Reader, x interface{}) (int, error) {
	hdr := make([]byte, 4)
	n, err := r.Read(hdr)
	if err != nil {
		return n, err
	}
	if n != 4 {
		return n, errors.Errorf("read %d bytes, expected 4 bytes", n)
	}
	bLen := binary.LittleEndian.Uint32(hdr)
	if bLen > maxLength || bLen < 1 {
		return n, errors.Errorf("bad message length: %d (max: %d)", bLen, maxLength)
	}
	b := make([]byte, bLen)
	n, err = r.Read(b)
	if err != nil {
		return 4 + n, err
	}
	if n != int(bLen) {
		return 4 + n, errors.Errorf("read %d bytes, expected %d bytes", n, bLen)
	}
	return 4 + n, json.Unmarshal(b, x)
}

func Marshal(x interface{}) ([]byte, error) {
	var b bytes.Buffer
	_, err := MarshalToWriter(&b, x)
	return b.Bytes(), err
}

func Unmarshal(b []byte, x interface{}) error {
	n, err := UnmarshalFromReader(bytes.NewReader(b), x)
	if n != len(b) {
		return errors.Errorf("read %d bytes, expected %d bytes", n, len(b))
	}
	return err
}
