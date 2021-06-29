package iputils

import (
	"encoding/binary"
	"math"
	"net"

	"github.com/pkg/errors"
)

func AddIPInt(ip net.IP, i int) (net.IP, error) {
	ip = ip.To4()
	if ip == nil {
		return nil, errors.Errorf("expected IPv4 address, got %s", ip.String())
	}
	ui32 := binary.BigEndian.Uint32(ip)
	resInt64 := int64(ui32) + int64(i)
	if resInt64 > int64(math.MaxUint32) {
		return nil, errors.Errorf("%s + %d overflows", ip.String(), i)
	}
	res := make(net.IP, 4)
	binary.BigEndian.PutUint32(res, uint32(resInt64))
	return res, nil
}
