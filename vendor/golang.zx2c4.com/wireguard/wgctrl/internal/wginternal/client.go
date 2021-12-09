package wginternal

import (
	"errors"
	"io"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// ErrReadOnly indicates that the driver backing a device is read-only. It is
// a sentinel value used in integration tests.
// TODO(mdlayher): consider exposing in API.
var ErrReadOnly = errors.New("driver is read-only")

// A Client is a type which can control a WireGuard device.
type Client interface {
	io.Closer
	Devices() ([]*wgtypes.Device, error)
	Device(name string) (*wgtypes.Device, error)
	ConfigureDevice(name string, cfg wgtypes.Config) error
}
