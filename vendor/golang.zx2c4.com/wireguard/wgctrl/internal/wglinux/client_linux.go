//go:build linux
// +build linux

package wglinux

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
	"github.com/mdlayher/netlink/nlenc"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/internal/wginternal"
	"golang.zx2c4.com/wireguard/wgctrl/internal/wglinux/internal/wgh"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var _ wginternal.Client = &Client{}

// A Client provides access to Linux WireGuard netlink information.
type Client struct {
	c      *genetlink.Conn
	family genetlink.Family

	interfaces func() ([]string, error)
}

// New creates a new Client and returns whether or not the generic netlink
// interface is available.
func New() (*Client, bool, error) {
	c, err := genetlink.Dial(nil)
	if err != nil {
		return nil, false, err
	}

	return initClient(c)
}

// initClient is the internal Client constructor used in some tests.
func initClient(c *genetlink.Conn) (*Client, bool, error) {
	f, err := c.GetFamily(wgh.GenlName)
	if err != nil {
		_ = c.Close()

		if errors.Is(err, os.ErrNotExist) {
			// The generic netlink interface is not available.
			return nil, false, nil
		}

		return nil, false, err
	}

	return &Client{
		c:      c,
		family: f,

		// By default, gather only WireGuard interfaces using rtnetlink.
		interfaces: rtnlInterfaces,
	}, true, nil
}

// Close implements wginternal.Client.
func (c *Client) Close() error {
	return c.c.Close()
}

// Devices implements wginternal.Client.
func (c *Client) Devices() ([]*wgtypes.Device, error) {
	// By default, rtnetlink is used to fetch a list of all interfaces and then
	// filter that list to only find WireGuard interfaces.
	//
	// The remainder of this function assumes that any returned device from this
	// function is a valid WireGuard device.
	ifis, err := c.interfaces()
	if err != nil {
		return nil, err
	}

	ds := make([]*wgtypes.Device, 0, len(ifis))
	for _, ifi := range ifis {
		d, err := c.Device(ifi)
		if err != nil {
			return nil, err
		}

		ds = append(ds, d)
	}

	return ds, nil
}

// Device implements wginternal.Client.
func (c *Client) Device(name string) (*wgtypes.Device, error) {
	// Don't bother querying netlink with empty input.
	if name == "" {
		return nil, os.ErrNotExist
	}

	// Fetching a device by interface index is possible as well, but we only
	// support fetching by name as it seems to be more convenient in general.
	b, err := netlink.MarshalAttributes([]netlink.Attribute{{
		Type: wgh.DeviceAIfname,
		Data: nlenc.Bytes(name),
	}})
	if err != nil {
		return nil, err
	}

	msgs, err := c.execute(wgh.CmdGetDevice, netlink.Request|netlink.Dump, b)
	if err != nil {
		return nil, err
	}

	return parseDevice(msgs)
}

// ConfigureDevice implements wginternal.Client.
func (c *Client) ConfigureDevice(name string, cfg wgtypes.Config) error {
	// Large configurations are split into batches for use with netlink.
	for _, b := range buildBatches(cfg) {
		attrs, err := configAttrs(name, b)
		if err != nil {
			return err
		}

		// Request acknowledgement of our request from netlink, even though the
		// output messages are unused.  The netlink package checks and trims the
		// status code value.
		if _, err := c.execute(wgh.CmdSetDevice, netlink.Request|netlink.Acknowledge, attrs); err != nil {
			return err
		}
	}

	return nil
}

// execute executes a single WireGuard netlink request with the specified command,
// header flags, and attribute arguments.
func (c *Client) execute(command uint8, flags netlink.HeaderFlags, attrb []byte) ([]genetlink.Message, error) {
	msg := genetlink.Message{
		Header: genetlink.Header{
			Command: command,
			Version: wgh.GenlVersion,
		},
		Data: attrb,
	}

	msgs, err := c.c.Execute(msg, c.family.ID, flags)
	if err == nil {
		return msgs, nil
	}

	// We don't want to expose netlink errors directly to callers so unpack to
	// something more generic.
	oerr, ok := err.(*netlink.OpError)
	if !ok {
		// Expect all errors to conform to netlink.OpError.
		return nil, fmt.Errorf("wglinux: netlink operation returned non-netlink error (please file a bug: https://golang.zx2c4.com/wireguard/wgctrl): %v", err)
	}

	switch oerr.Err {
	// Convert "no such device" and "not a wireguard device" to an error
	// compatible with os.ErrNotExist for easy checking.
	case unix.ENODEV, unix.ENOTSUP:
		return nil, os.ErrNotExist
	default:
		// Expose the inner error directly (such as EPERM).
		return nil, oerr.Err
	}
}

// rtnlInterfaces uses rtnetlink to fetch a list of WireGuard interfaces.
func rtnlInterfaces() ([]string, error) {
	// Use the stdlib's rtnetlink helpers to get ahold of a table of all
	// interfaces, so we can begin filtering it down to just WireGuard devices.
	tab, err := syscall.NetlinkRIB(unix.RTM_GETLINK, unix.AF_UNSPEC)
	if err != nil {
		return nil, fmt.Errorf("wglinux: failed to get list of interfaces from rtnetlink: %v", err)
	}

	msgs, err := syscall.ParseNetlinkMessage(tab)
	if err != nil {
		return nil, fmt.Errorf("wglinux: failed to parse rtnetlink messages: %v", err)
	}

	return parseRTNLInterfaces(msgs)
}

// parseRTNLInterfaces unpacks rtnetlink messages and returns WireGuard
// interface names.
func parseRTNLInterfaces(msgs []syscall.NetlinkMessage) ([]string, error) {
	var ifis []string
	for _, m := range msgs {
		// Only deal with link messages, and they must have an ifinfomsg
		// structure appear before the attributes.
		if m.Header.Type != unix.RTM_NEWLINK {
			continue
		}

		if len(m.Data) < unix.SizeofIfInfomsg {
			return nil, fmt.Errorf("wglinux: rtnetlink message is too short for ifinfomsg: %d", len(m.Data))
		}

		ad, err := netlink.NewAttributeDecoder(m.Data[syscall.SizeofIfInfomsg:])
		if err != nil {
			return nil, err
		}

		// Determine the interface's name and if it's a WireGuard device.
		var (
			ifi  string
			isWG bool
		)

		for ad.Next() {
			switch ad.Type() {
			case unix.IFLA_IFNAME:
				ifi = ad.String()
			case unix.IFLA_LINKINFO:
				ad.Do(isWGKind(&isWG))
			}
		}

		if err := ad.Err(); err != nil {
			return nil, err
		}

		if isWG {
			// Found one; append it to the list.
			ifis = append(ifis, ifi)
		}
	}

	return ifis, nil
}

// wgKind is the IFLA_INFO_KIND value for WireGuard devices.
const wgKind = "wireguard"

// isWGKind parses netlink attributes to determine if a link is a WireGuard
// device, then populates ok with the result.
func isWGKind(ok *bool) func(b []byte) error {
	return func(b []byte) error {
		ad, err := netlink.NewAttributeDecoder(b)
		if err != nil {
			return err
		}

		for ad.Next() {
			if ad.Type() != unix.IFLA_INFO_KIND {
				continue
			}

			if ad.String() == wgKind {
				*ok = true
				return nil
			}
		}

		return ad.Err()
	}
}
