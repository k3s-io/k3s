//go:build linux
// +build linux

package wglinux

import (
	"encoding/binary"
	"fmt"
	"net"
	"unsafe"

	"github.com/mdlayher/netlink"
	"github.com/mdlayher/netlink/nlenc"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/internal/wglinux/internal/wgh"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// configAttrs creates the required encoded netlink attributes to configure
// the device specified by name using the non-nil fields in cfg.
func configAttrs(name string, cfg wgtypes.Config) ([]byte, error) {
	ae := netlink.NewAttributeEncoder()
	ae.String(wgh.DeviceAIfname, name)

	if cfg.PrivateKey != nil {
		ae.Bytes(wgh.DeviceAPrivateKey, (*cfg.PrivateKey)[:])
	}

	if cfg.ListenPort != nil {
		ae.Uint16(wgh.DeviceAListenPort, uint16(*cfg.ListenPort))
	}

	if cfg.FirewallMark != nil {
		ae.Uint32(wgh.DeviceAFwmark, uint32(*cfg.FirewallMark))
	}

	if cfg.ReplacePeers {
		ae.Uint32(wgh.DeviceAFlags, wgh.DeviceFReplacePeers)
	}

	// Only apply peer attributes if necessary.
	if len(cfg.Peers) > 0 {
		ae.Nested(wgh.DeviceAPeers, func(nae *netlink.AttributeEncoder) error {
			// Netlink arrays use type as an array index.
			for i, p := range cfg.Peers {
				nae.Nested(uint16(i), encodePeer(p))
			}

			return nil
		})
	}

	return ae.Encode()
}

// ipBatchChunk is a tunable allowed IP batch limit per peer.
//
// Because we don't necessarily know how much space a given peer will occupy,
// we play it safe and use a reasonably small value.  Note that this constant
// is used both in this package and tests, so be aware when making changes.
const ipBatchChunk = 256

// peerBatchChunk specifies the number of peers that can appear in a
// configuration before we start splitting it into chunks.
const peerBatchChunk = 32

// shouldBatch determines if a configuration is sufficiently complex that it
// should be split into batches.
func shouldBatch(cfg wgtypes.Config) bool {
	if len(cfg.Peers) > peerBatchChunk {
		return true
	}

	var ips int
	for _, p := range cfg.Peers {
		ips += len(p.AllowedIPs)
	}

	return ips > ipBatchChunk
}

// buildBatches produces a batch of configs from a single config, if needed.
func buildBatches(cfg wgtypes.Config) []wgtypes.Config {
	// Is this a small configuration; no need to batch?
	if !shouldBatch(cfg) {
		return []wgtypes.Config{cfg}
	}

	// Use most fields of cfg for our "base" configuration, and only differ
	// peers in each batch.
	base := cfg
	base.Peers = nil

	// Track the known peers so that peer IPs are not replaced if a single
	// peer has its allowed IPs split into multiple batches.
	knownPeers := make(map[wgtypes.Key]struct{})

	batches := make([]wgtypes.Config, 0)
	for _, p := range cfg.Peers {
		batch := base

		// Iterate until no more allowed IPs.
		var done bool
		for !done {
			var tmp []net.IPNet
			if len(p.AllowedIPs) < ipBatchChunk {
				// IPs all fit within a batch; we are done.
				tmp = make([]net.IPNet, len(p.AllowedIPs))
				copy(tmp, p.AllowedIPs)
				done = true
			} else {
				// IPs are larger than a single batch, copy a batch out and
				// advance the cursor.
				tmp = make([]net.IPNet, ipBatchChunk)
				copy(tmp, p.AllowedIPs[:ipBatchChunk])

				p.AllowedIPs = p.AllowedIPs[ipBatchChunk:]

				if len(p.AllowedIPs) == 0 {
					// IPs ended on a batch boundary; no more IPs left so end
					// iteration after this loop.
					done = true
				}
			}

			pcfg := wgtypes.PeerConfig{
				// PublicKey denotes the peer and must be present.
				PublicKey: p.PublicKey,

				// Apply the update only flag to every chunk to ensure
				// consistency between batches when the kernel module processes
				// them.
				UpdateOnly: p.UpdateOnly,

				// It'd be a bit weird to have a remove peer message with many
				// IPs, but just in case, add this to every peer's message.
				Remove: p.Remove,

				// The IPs for this chunk.
				AllowedIPs: tmp,
			}

			// Only pass certain fields on the first occurrence of a peer, so
			// that subsequent IPs won't be wiped out and space isn't wasted.
			if _, ok := knownPeers[p.PublicKey]; !ok {
				knownPeers[p.PublicKey] = struct{}{}

				pcfg.PresharedKey = p.PresharedKey
				pcfg.Endpoint = p.Endpoint
				pcfg.PersistentKeepaliveInterval = p.PersistentKeepaliveInterval

				// Important: do not move or appending peers won't work.
				pcfg.ReplaceAllowedIPs = p.ReplaceAllowedIPs
			}

			// Add a peer configuration to this batch and keep going.
			batch.Peers = []wgtypes.PeerConfig{pcfg}
			batches = append(batches, batch)
		}
	}

	// Do not allow peer replacement beyond the first message in a batch,
	// so we don't overwrite our previous batch work.
	for i := range batches {
		if i > 0 {
			batches[i].ReplacePeers = false
		}
	}

	return batches
}

// encodePeer returns a function to encode PeerConfig nested attributes.
func encodePeer(p wgtypes.PeerConfig) func(ae *netlink.AttributeEncoder) error {
	return func(ae *netlink.AttributeEncoder) error {
		ae.Bytes(wgh.PeerAPublicKey, p.PublicKey[:])

		// Flags are stored in a single attribute.
		var flags uint32
		if p.Remove {
			flags |= wgh.PeerFRemoveMe
		}
		if p.ReplaceAllowedIPs {
			flags |= wgh.PeerFReplaceAllowedips
		}
		if p.UpdateOnly {
			flags |= wgh.PeerFUpdateOnly
		}
		if flags != 0 {
			ae.Uint32(wgh.PeerAFlags, flags)
		}

		if p.PresharedKey != nil {
			ae.Bytes(wgh.PeerAPresharedKey, (*p.PresharedKey)[:])
		}

		if p.Endpoint != nil {
			ae.Do(wgh.PeerAEndpoint, encodeSockaddr(*p.Endpoint))
		}

		if p.PersistentKeepaliveInterval != nil {
			ae.Uint16(wgh.PeerAPersistentKeepaliveInterval, uint16(p.PersistentKeepaliveInterval.Seconds()))
		}

		// Only apply allowed IPs if necessary.
		if len(p.AllowedIPs) > 0 {
			ae.Nested(wgh.PeerAAllowedips, encodeAllowedIPs(p.AllowedIPs))
		}

		return nil
	}
}

// encodeSockaddr returns a function which encodes a net.UDPAddr as raw
// sockaddr_in or sockaddr_in6 bytes.
func encodeSockaddr(endpoint net.UDPAddr) func() ([]byte, error) {
	return func() ([]byte, error) {
		if !isValidIP(endpoint.IP) {
			return nil, fmt.Errorf("wglinux: invalid endpoint IP: %s", endpoint.IP.String())
		}

		// Is this an IPv6 address?
		if isIPv6(endpoint.IP) {
			var addr [16]byte
			copy(addr[:], endpoint.IP.To16())

			sa := unix.RawSockaddrInet6{
				Family: unix.AF_INET6,
				Port:   sockaddrPort(endpoint.Port),
				Addr:   addr,
			}

			return (*(*[unix.SizeofSockaddrInet6]byte)(unsafe.Pointer(&sa)))[:], nil
		}

		// IPv4 address handling.
		var addr [4]byte
		copy(addr[:], endpoint.IP.To4())

		sa := unix.RawSockaddrInet4{
			Family: unix.AF_INET,
			Port:   sockaddrPort(endpoint.Port),
			Addr:   addr,
		}

		return (*(*[unix.SizeofSockaddrInet4]byte)(unsafe.Pointer(&sa)))[:], nil
	}
}

// encodeAllowedIPs returns a function to encode allowed IP nested attributes.
func encodeAllowedIPs(ipns []net.IPNet) func(ae *netlink.AttributeEncoder) error {
	return func(ae *netlink.AttributeEncoder) error {
		for i, ipn := range ipns {
			if !isValidIP(ipn.IP) {
				return fmt.Errorf("wglinux: invalid allowed IP: %s", ipn.IP.String())
			}

			family := uint16(unix.AF_INET6)
			if !isIPv6(ipn.IP) {
				// Make sure address is 4 bytes if IPv4.
				family = unix.AF_INET
				ipn.IP = ipn.IP.To4()
			}

			// Netlink arrays use type as an array index.
			ae.Nested(uint16(i), func(nae *netlink.AttributeEncoder) error {
				nae.Uint16(wgh.AllowedipAFamily, family)
				nae.Bytes(wgh.AllowedipAIpaddr, ipn.IP)

				ones, _ := ipn.Mask.Size()
				nae.Uint8(wgh.AllowedipACidrMask, uint8(ones))
				return nil
			})
		}

		return nil
	}
}

// isValidIP determines if IP is a valid IPv4 or IPv6 address.
func isValidIP(ip net.IP) bool {
	return ip.To16() != nil
}

// isIPv6 determines if IP is a valid IPv6 address.
func isIPv6(ip net.IP) bool {
	return isValidIP(ip) && ip.To4() == nil
}

// sockaddrPort interprets port as a big endian uint16 for use passing sockaddr
// structures to the kernel.
func sockaddrPort(port int) uint16 {
	return binary.BigEndian.Uint16(nlenc.Uint16Bytes(uint16(port)))
}
