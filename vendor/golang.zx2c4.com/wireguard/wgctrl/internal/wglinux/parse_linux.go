//go:build linux
// +build linux

package wglinux

import (
	"fmt"
	"net"
	"time"
	"unsafe"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/internal/wglinux/internal/wgh"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// parseDevice parses a Device from a slice of generic netlink messages,
// automatically merging peer lists from subsequent messages into the Device
// from the first message.
func parseDevice(msgs []genetlink.Message) (*wgtypes.Device, error) {
	var first wgtypes.Device
	knownPeers := make(map[wgtypes.Key]int)

	for i, m := range msgs {
		d, err := parseDeviceLoop(m)
		if err != nil {
			return nil, err
		}

		if i == 0 {
			// First message contains our target device.
			first = *d

			// Gather the known peers so that we can merge
			// them later if needed
			for i := range first.Peers {
				knownPeers[first.Peers[i].PublicKey] = i
			}

			continue
		}

		// Any subsequent messages have their peer contents merged into the
		// first "target" message.
		mergeDevices(&first, d, knownPeers)
	}

	return &first, nil
}

// parseDeviceLoop parses a Device from a single generic netlink message.
func parseDeviceLoop(m genetlink.Message) (*wgtypes.Device, error) {
	ad, err := netlink.NewAttributeDecoder(m.Data)
	if err != nil {
		return nil, err
	}

	d := wgtypes.Device{Type: wgtypes.LinuxKernel}
	for ad.Next() {
		switch ad.Type() {
		case wgh.DeviceAIfindex:
			// Ignored; interface index isn't exposed at all in the userspace
			// configuration protocol, and name is more friendly anyway.
		case wgh.DeviceAIfname:
			d.Name = ad.String()
		case wgh.DeviceAPrivateKey:
			ad.Do(parseKey(&d.PrivateKey))
		case wgh.DeviceAPublicKey:
			ad.Do(parseKey(&d.PublicKey))
		case wgh.DeviceAListenPort:
			d.ListenPort = int(ad.Uint16())
		case wgh.DeviceAFwmark:
			d.FirewallMark = int(ad.Uint32())
		case wgh.DeviceAPeers:
			// Netlink array of peers.
			//
			// Errors while parsing are propagated up to top-level ad.Err check.
			ad.Nested(func(nad *netlink.AttributeDecoder) error {
				// Initialize to the number of peers in this decoder and begin
				// handling nested Peer attributes.
				d.Peers = make([]wgtypes.Peer, 0, nad.Len())
				for nad.Next() {
					nad.Nested(func(nnad *netlink.AttributeDecoder) error {
						d.Peers = append(d.Peers, parsePeer(nnad))
						return nil
					})
				}

				return nil
			})
		}
	}

	if err := ad.Err(); err != nil {
		return nil, err
	}

	return &d, nil
}

// parseAllowedIPs parses a wgtypes.Peer from a netlink attribute payload.
func parsePeer(ad *netlink.AttributeDecoder) wgtypes.Peer {
	var p wgtypes.Peer
	for ad.Next() {
		switch ad.Type() {
		case wgh.PeerAPublicKey:
			ad.Do(parseKey(&p.PublicKey))
		case wgh.PeerAPresharedKey:
			ad.Do(parseKey(&p.PresharedKey))
		case wgh.PeerAEndpoint:
			p.Endpoint = &net.UDPAddr{}
			ad.Do(parseSockaddr(p.Endpoint))
		case wgh.PeerAPersistentKeepaliveInterval:
			p.PersistentKeepaliveInterval = time.Duration(ad.Uint16()) * time.Second
		case wgh.PeerALastHandshakeTime:
			ad.Do(parseTimespec(&p.LastHandshakeTime))
		case wgh.PeerARxBytes:
			p.ReceiveBytes = int64(ad.Uint64())
		case wgh.PeerATxBytes:
			p.TransmitBytes = int64(ad.Uint64())
		case wgh.PeerAAllowedips:
			ad.Nested(parseAllowedIPs(&p.AllowedIPs))
		case wgh.PeerAProtocolVersion:
			p.ProtocolVersion = int(ad.Uint32())
		}
	}

	return p
}

// parseAllowedIPs parses a slice of net.IPNet from a netlink attribute payload.
func parseAllowedIPs(ipns *[]net.IPNet) func(ad *netlink.AttributeDecoder) error {
	return func(ad *netlink.AttributeDecoder) error {
		// Initialize to the number of allowed IPs and begin iterating through
		// the netlink array to decode each one.
		*ipns = make([]net.IPNet, 0, ad.Len())
		for ad.Next() {
			// Allowed IP nested attributes.
			ad.Nested(func(nad *netlink.AttributeDecoder) error {
				var (
					ipn    net.IPNet
					mask   int
					family int
				)

				for nad.Next() {
					switch nad.Type() {
					case wgh.AllowedipAIpaddr:
						nad.Do(parseAddr(&ipn.IP))
					case wgh.AllowedipACidrMask:
						mask = int(nad.Uint8())
					case wgh.AllowedipAFamily:
						family = int(nad.Uint16())
					}
				}

				if err := nad.Err(); err != nil {
					return err
				}

				// The address family determines the correct number of bits in
				// the mask.
				switch family {
				case unix.AF_INET:
					ipn.Mask = net.CIDRMask(mask, 32)
				case unix.AF_INET6:
					ipn.Mask = net.CIDRMask(mask, 128)
				}

				*ipns = append(*ipns, ipn)
				return nil
			})
		}

		return nil
	}
}

// parseKey parses a wgtypes.Key from a byte slice.
func parseKey(key *wgtypes.Key) func(b []byte) error {
	return func(b []byte) error {
		k, err := wgtypes.NewKey(b)
		if err != nil {
			return err
		}

		*key = k
		return nil
	}
}

// parseAddr parses a net.IP from raw in_addr or in6_addr struct bytes.
func parseAddr(ip *net.IP) func(b []byte) error {
	return func(b []byte) error {
		switch len(b) {
		case net.IPv4len, net.IPv6len:
			// Okay to convert directly to net.IP; memory layout is identical.
			*ip = make(net.IP, len(b))
			copy(*ip, b)
			return nil
		default:
			return fmt.Errorf("wglinux: unexpected IP address size: %d", len(b))
		}
	}
}

// parseSockaddr parses a *net.UDPAddr from raw sockaddr_in or sockaddr_in6 bytes.
func parseSockaddr(endpoint *net.UDPAddr) func(b []byte) error {
	return func(b []byte) error {
		switch len(b) {
		case unix.SizeofSockaddrInet4:
			// IPv4 address parsing.
			sa := *(*unix.RawSockaddrInet4)(unsafe.Pointer(&b[0]))

			*endpoint = net.UDPAddr{
				IP:   net.IP(sa.Addr[:]).To4(),
				Port: int(sockaddrPort(int(sa.Port))),
			}

			return nil
		case unix.SizeofSockaddrInet6:
			// IPv6 address parsing.
			sa := *(*unix.RawSockaddrInet6)(unsafe.Pointer(&b[0]))

			*endpoint = net.UDPAddr{
				IP:   net.IP(sa.Addr[:]),
				Port: int(sockaddrPort(int(sa.Port))),
			}

			return nil
		default:
			return fmt.Errorf("wglinux: unexpected sockaddr size: %d", len(b))
		}
	}
}

// timespec32 is a unix.Timespec with 32-bit integers.
type timespec32 struct {
	Sec  int32
	Nsec int32
}

// timespec64 is a unix.Timespec with 64-bit integers.
type timespec64 struct {
	Sec  int64
	Nsec int64
}

const (
	sizeofTimespec32 = int(unsafe.Sizeof(timespec32{}))
	sizeofTimespec64 = int(unsafe.Sizeof(timespec64{}))
)

// parseTimespec parses a time.Time from raw timespec bytes.
func parseTimespec(t *time.Time) func(b []byte) error {
	return func(b []byte) error {
		// It would appear that WireGuard can return a __kernel_timespec which
		// uses 64-bit integers, even on 32-bit platforms. Clarification of this
		// behavior is being sought in:
		// https://lists.zx2c4.com/pipermail/wireguard/2019-April/004088.html.
		//
		// In the mean time, be liberal and accept 32-bit and 64-bit variants.
		var sec, nsec int64

		switch len(b) {
		case sizeofTimespec32:
			ts := *(*timespec32)(unsafe.Pointer(&b[0]))

			sec = int64(ts.Sec)
			nsec = int64(ts.Nsec)
		case sizeofTimespec64:
			ts := *(*timespec64)(unsafe.Pointer(&b[0]))

			sec = ts.Sec
			nsec = ts.Nsec
		default:
			return fmt.Errorf("wglinux: unexpected timespec size: %d bytes, expected 8 or 16 bytes", len(b))
		}

		// Only set fields if UNIX timestamp value is greater than 0, so the
		// caller will see a zero-value time.Time otherwise.
		if sec > 0 || nsec > 0 {
			*t = time.Unix(sec, nsec)
		}

		return nil
	}
}

// mergeDevices merges Peer information from d into target.  mergeDevices is
// used to deal with multiple incoming netlink messages for the same device.
func mergeDevices(target, d *wgtypes.Device, knownPeers map[wgtypes.Key]int) {
	for i := range d.Peers {
		// Peer is already known, append to it's allowed IP networks
		if peerIndex, ok := knownPeers[d.Peers[i].PublicKey]; ok {
			target.Peers[peerIndex].AllowedIPs = append(target.Peers[peerIndex].AllowedIPs, d.Peers[i].AllowedIPs...)
		} else { // New peer, add it to the target peers.
			target.Peers = append(target.Peers, d.Peers[i])
			knownPeers[d.Peers[i].PublicKey] = len(target.Peers) - 1
		}
	}
}
