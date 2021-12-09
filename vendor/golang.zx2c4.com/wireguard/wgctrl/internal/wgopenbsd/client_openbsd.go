//go:build openbsd
// +build openbsd

package wgopenbsd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/internal/wginternal"
	"golang.zx2c4.com/wireguard/wgctrl/internal/wgopenbsd/internal/wgh"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	// ifGroupWG is the WireGuard interface group name passed to the kernel.
	ifGroupWG = [16]byte{0: 'w', 1: 'g'}
)

var _ wginternal.Client = &Client{}

// A Client provides access to OpenBSD WireGuard ioctl information.
type Client struct {
	// Hooks which use system calls by default, but can also be swapped out
	// during tests.
	close           func() error
	ioctlIfgroupreq func(ifg *wgh.Ifgroupreq) error
	ioctlWGDataIO   func(data *wgh.WGDataIO) error
}

// New creates a new Client and returns whether or not the ioctl interface
// is available.
func New() (*Client, bool, error) {
	// The OpenBSD ioctl interface operates on a generic AF_INET socket.
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return nil, false, err
	}

	// TODO(mdlayher): find a call to invoke here to probe for availability.
	// c.Devices won't work because it returns a "not found" error when the
	// kernel WireGuard implementation is available but the interface group
	// has no members.

	// By default, use system call implementations for all hook functions.
	return &Client{
		close:           func() error { return unix.Close(fd) },
		ioctlIfgroupreq: ioctlIfgroupreq(fd),
		ioctlWGDataIO:   ioctlWGDataIO(fd),
	}, true, nil
}

// Close implements wginternal.Client.
func (c *Client) Close() error {
	return c.close()
}

// Devices implements wginternal.Client.
func (c *Client) Devices() ([]*wgtypes.Device, error) {
	ifg := wgh.Ifgroupreq{
		// Query for devices in the "wg" group.
		Name: ifGroupWG,
	}

	// Determine how many device names we must allocate memory for.
	if err := c.ioctlIfgroupreq(&ifg); err != nil {
		return nil, err
	}

	// ifg.Len is size in bytes; allocate enough memory for the correct number
	// of wgh.Ifgreq and then store a pointer to the memory where the data
	// should be written (ifgrs) in ifg.Groups.
	//
	// From a thread in golang-nuts, this pattern is valid:
	// "It would be OK to pass a pointer to a struct to ioctl if the struct
	// contains a pointer to other Go memory, but the struct field must have
	// pointer type."
	// See: https://groups.google.com/forum/#!topic/golang-nuts/FfasFTZvU_o.
	ifgrs := make([]wgh.Ifgreq, ifg.Len/wgh.SizeofIfgreq)
	ifg.Groups = &ifgrs[0]

	// Now actually fetch the device names.
	if err := c.ioctlIfgroupreq(&ifg); err != nil {
		return nil, err
	}

	// Keep this alive until we're done doing the ioctl dance.
	runtime.KeepAlive(&ifg)

	devices := make([]*wgtypes.Device, 0, len(ifgrs))
	for _, ifgr := range ifgrs {
		// Remove any trailing NULL bytes from the interface names.
		d, err := c.Device(string(bytes.TrimRight(ifgr.Ifgrqu[:], "\x00")))
		if err != nil {
			return nil, err
		}

		devices = append(devices, d)
	}

	return devices, nil
}

// Device implements wginternal.Client.
func (c *Client) Device(name string) (*wgtypes.Device, error) {
	dname, err := deviceName(name)
	if err != nil {
		return nil, err
	}

	// First, specify the name of the device and determine how much memory
	// must be allocated in order to store the WGInterfaceIO structure and
	// any trailing WGPeerIO/WGAIPIOs.
	data := wgh.WGDataIO{Name: dname}

	// TODO: consider preallocating some memory to avoid a second system call
	// if it proves to be a concern.
	var mem []byte
	for {
		if err := c.ioctlWGDataIO(&data); err != nil {
			// ioctl functions always return a wrapped unix.Errno value.
			// Conform to the wgctrl contract by unwrapping some values:
			//   ENXIO: "no such device": (no such WireGuard device)
			//   ENOTTY: "inappropriate ioctl for device" (device is not a
			//	   WireGuard device)
			switch err.(*os.SyscallError).Err {
			case unix.ENXIO, unix.ENOTTY:
				return nil, os.ErrNotExist
			default:
				return nil, err
			}
		}

		if len(mem) >= int(data.Size) {
			// Allocated enough memory!
			break
		}

		// Ensure we don't unsafe cast into uninitialized memory. We need at very
		// least a single WGInterfaceIO with no peers.
		if data.Size < wgh.SizeofWGInterfaceIO {
			return nil, fmt.Errorf("wgopenbsd: kernel returned unexpected number of bytes for WGInterfaceIO: %d", data.Size)
		}

		// Allocate the appropriate amount of memory and point the kernel at
		// the first byte of our slice's backing array. When the loop continues,
		// we will check if we've allocated enough memory.
		mem = make([]byte, data.Size)
		data.Interface = (*wgh.WGInterfaceIO)(unsafe.Pointer(&mem[0]))
	}

	return parseDevice(name, data.Interface)
}

// parseDevice unpacks a Device from ifio, along with its associated peers
// and their allowed IPs.
func parseDevice(name string, ifio *wgh.WGInterfaceIO) (*wgtypes.Device, error) {
	d := &wgtypes.Device{
		Name: name,
		Type: wgtypes.OpenBSDKernel,
	}

	// The kernel populates ifio.Flags to indicate which fields are present.

	if ifio.Flags&wgh.WG_INTERFACE_HAS_PRIVATE != 0 {
		d.PrivateKey = wgtypes.Key(ifio.Private)
	}

	if ifio.Flags&wgh.WG_INTERFACE_HAS_PUBLIC != 0 {
		d.PublicKey = wgtypes.Key(ifio.Public)
	}

	if ifio.Flags&wgh.WG_INTERFACE_HAS_PORT != 0 {
		d.ListenPort = int(ifio.Port)
	}

	if ifio.Flags&wgh.WG_INTERFACE_HAS_RTABLE != 0 {
		d.FirewallMark = int(ifio.Rtable)
	}

	d.Peers = make([]wgtypes.Peer, 0, ifio.Peers_count)

	// If there were no peers, exit early so we do not advance the pointer
	// beyond the end of the WGInterfaceIO structure.
	if ifio.Peers_count == 0 {
		return d, nil
	}

	// Set our pointer to the beginning of the first peer's location in memory.
	peer := (*wgh.WGPeerIO)(unsafe.Pointer(
		uintptr(unsafe.Pointer(ifio)) + wgh.SizeofWGInterfaceIO,
	))

	for i := 0; i < int(ifio.Peers_count); i++ {
		p := parsePeer(peer)

		// Same idea, we know how many allowed IPs we need to account for, so
		// reserve the space and advance the pointer through each WGAIP structure.
		p.AllowedIPs = make([]net.IPNet, 0, peer.Aips_count)
		for j := uintptr(0); j < uintptr(peer.Aips_count); j++ {
			aip := (*wgh.WGAIPIO)(unsafe.Pointer(
				uintptr(unsafe.Pointer(peer)) + wgh.SizeofWGPeerIO + j*wgh.SizeofWGAIPIO,
			))

			p.AllowedIPs = append(p.AllowedIPs, parseAllowedIP(aip))
		}

		// Prepare for the next iteration.
		d.Peers = append(d.Peers, p)
		peer = (*wgh.WGPeerIO)(unsafe.Pointer(
			uintptr(unsafe.Pointer(peer)) + wgh.SizeofWGPeerIO +
				uintptr(peer.Aips_count)*wgh.SizeofWGAIPIO,
		))
	}

	return d, nil
}

// ConfigureDevice implements wginternal.Client.
func (c *Client) ConfigureDevice(name string, cfg wgtypes.Config) error {
	// Currently read-only: we must determine if a device belongs to this driver,
	// and if it does, return a sentinel so integration tests that configure a
	// device can be skipped.
	if _, err := c.Device(name); err != nil {
		return err
	}

	return wginternal.ErrReadOnly
}

// deviceName converts an interface name string to the format required to pass
// with wgh.WGGetServ.
func deviceName(name string) ([16]byte, error) {
	var out [unix.IFNAMSIZ]byte
	if len(name) > unix.IFNAMSIZ {
		return out, fmt.Errorf("wgopenbsd: interface name %q too long", name)
	}

	copy(out[:], name)
	return out, nil
}

// parsePeer unpacks a wgtypes.Peer from a WGPeerIO structure.
func parsePeer(pio *wgh.WGPeerIO) wgtypes.Peer {
	p := wgtypes.Peer{
		ReceiveBytes:    int64(pio.Rxbytes),
		TransmitBytes:   int64(pio.Txbytes),
		ProtocolVersion: int(pio.Protocol_version),
	}

	// Only set last handshake if a non-zero timespec was provided, matching
	// the time.Time.IsZero() behavior of internal/wglinux.
	if pio.Last_handshake.Sec > 0 && pio.Last_handshake.Nsec > 0 {
		p.LastHandshakeTime = time.Unix(
			pio.Last_handshake.Sec,
			// Conversion required for GOARCH=386.
			int64(pio.Last_handshake.Nsec),
		)
	}

	if pio.Flags&wgh.WG_PEER_HAS_PUBLIC != 0 {
		p.PublicKey = wgtypes.Key(pio.Public)
	}

	if pio.Flags&wgh.WG_PEER_HAS_PSK != 0 {
		p.PresharedKey = wgtypes.Key(pio.Psk)
	}

	if pio.Flags&wgh.WG_PEER_HAS_PKA != 0 {
		p.PersistentKeepaliveInterval = time.Duration(pio.Pka) * time.Second
	}

	if pio.Flags&wgh.WG_PEER_HAS_ENDPOINT != 0 {
		p.Endpoint = parseEndpoint(pio.Endpoint)
	}

	return p
}

// parseAllowedIP unpacks a net.IPNet from a WGAIP structure.
func parseAllowedIP(aip *wgh.WGAIPIO) net.IPNet {
	switch aip.Af {
	case unix.AF_INET:
		return net.IPNet{
			IP:   net.IP(aip.Addr[:net.IPv4len]),
			Mask: net.CIDRMask(int(aip.Cidr), 32),
		}
	case unix.AF_INET6:
		return net.IPNet{
			IP:   net.IP(aip.Addr[:]),
			Mask: net.CIDRMask(int(aip.Cidr), 128),
		}
	default:
		panicf("wgopenbsd: invalid address family for allowed IP: %+v", aip)
		return net.IPNet{}
	}
}

// parseEndpoint parses a peer endpoint from a wgh.WGIP structure.
func parseEndpoint(ep [28]byte) *net.UDPAddr {
	// sockaddr* structures have family at index 1.
	switch ep[1] {
	case unix.AF_INET:
		sa := *(*unix.RawSockaddrInet4)(unsafe.Pointer(&ep[0]))

		ep := &net.UDPAddr{
			IP:   make(net.IP, net.IPv4len),
			Port: bePort(sa.Port),
		}
		copy(ep.IP, sa.Addr[:])

		return ep
	case unix.AF_INET6:
		sa := *(*unix.RawSockaddrInet6)(unsafe.Pointer(&ep[0]))

		// TODO(mdlayher): IPv6 zone?
		ep := &net.UDPAddr{
			IP:   make(net.IP, net.IPv6len),
			Port: bePort(sa.Port),
		}
		copy(ep.IP, sa.Addr[:])

		return ep
	default:
		// No endpoint configured.
		return nil
	}
}

// bePort interprets a port integer stored in native endianness as a big
// endian value. This is necessary for proper endpoint port handling on
// little endian machines.
func bePort(port uint16) int {
	b := *(*[2]byte)(unsafe.Pointer(&port))
	return int(binary.BigEndian.Uint16(b[:]))
}

// ioctlIfgroupreq returns a function which performs the appropriate ioctl on
// fd to retrieve members of an interface group.
func ioctlIfgroupreq(fd int) func(*wgh.Ifgroupreq) error {
	return func(ifg *wgh.Ifgroupreq) error {
		return ioctl(fd, wgh.SIOCGIFGMEMB, unsafe.Pointer(ifg))
	}
}

// ioctlWGDataIO returns a function which performs the appropriate ioctl on
// fd to issue a WireGuard data I/O.
func ioctlWGDataIO(fd int) func(*wgh.WGDataIO) error {
	return func(data *wgh.WGDataIO) error {
		return ioctl(fd, wgh.SIOCGWG, unsafe.Pointer(data))
	}
}

// ioctl is a raw wrapper for the ioctl system call.
func ioctl(fd int, req uint, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(arg))
	if errno != 0 {
		return os.NewSyscallError("ioctl", errno)
	}

	return nil
}

func panicf(format string, a ...interface{}) {
	panic(fmt.Sprintf(format, a...))
}
