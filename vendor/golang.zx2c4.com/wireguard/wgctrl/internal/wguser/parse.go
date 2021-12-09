package wguser

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// The WireGuard userspace configuration protocol is described here:
// https://www.wireguard.com/xplatform/#cross-platform-userspace-implementation.

// getDevice gathers device information from a device specified by its path
// and returns a Device.
func (c *Client) getDevice(device string) (*wgtypes.Device, error) {
	conn, err := c.dial(device)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Get information about this device.
	if _, err := io.WriteString(conn, "get=1\n\n"); err != nil {
		return nil, err
	}

	// Parse the device from the incoming data stream.
	d, err := parseDevice(conn)
	if err != nil {
		return nil, err
	}

	// TODO(mdlayher): populate interface index too?
	d.Name = deviceName(device)
	d.Type = wgtypes.Userspace

	return d, nil
}

// parseDevice parses a Device and its Peers from an io.Reader.
func parseDevice(r io.Reader) (*wgtypes.Device, error) {
	var dp deviceParser
	s := bufio.NewScanner(r)
	for s.Scan() {
		b := s.Bytes()
		if len(b) == 0 {
			// Empty line, done parsing.
			break
		}

		// All data is in key=value format.
		kvs := bytes.Split(b, []byte("="))
		if len(kvs) != 2 {
			return nil, fmt.Errorf("wguser: invalid key=value pair: %q", string(b))
		}

		dp.Parse(string(kvs[0]), string(kvs[1]))
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	return dp.Device()
}

// A deviceParser accumulates information about a Device and its Peers.
type deviceParser struct {
	d   wgtypes.Device
	err error

	parsePeers    bool
	peers         int
	hsSec, hsNano int
}

// Device returns a Device or any errors that were encountered while parsing
// a Device.
func (dp *deviceParser) Device() (*wgtypes.Device, error) {
	if dp.err != nil {
		return nil, dp.err
	}

	// Compute remaining fields of the Device now that all parsing is done.
	dp.d.PublicKey = dp.d.PrivateKey.PublicKey()

	return &dp.d, nil
}

// Parse parses a single key/value pair into fields of a Device.
func (dp *deviceParser) Parse(key, value string) {
	switch key {
	case "errno":
		// 0 indicates success, anything else returns an error number that matches
		// definitions from errno.h.
		if errno := dp.parseInt(value); errno != 0 {
			// TODO(mdlayher): return actual errno on Linux?
			dp.err = os.NewSyscallError("read", fmt.Errorf("wguser: errno=%d", errno))
			return
		}
	case "public_key":
		// We've either found the first peer or the next peer.  Stop parsing
		// Device fields and start parsing Peer fields, including the public
		// key indicated here.
		dp.parsePeers = true
		dp.peers++

		dp.d.Peers = append(dp.d.Peers, wgtypes.Peer{
			PublicKey: dp.parseKey(value),
		})
		return
	}

	// Are we parsing peer fields?
	if dp.parsePeers {
		dp.peerParse(key, value)
		return
	}

	// Device field parsing.
	switch key {
	case "private_key":
		dp.d.PrivateKey = dp.parseKey(value)
	case "listen_port":
		dp.d.ListenPort = dp.parseInt(value)
	case "fwmark":
		dp.d.FirewallMark = dp.parseInt(value)
	}
}

// curPeer returns the current Peer being parsed so its fields can be populated.
func (dp *deviceParser) curPeer() *wgtypes.Peer {
	return &dp.d.Peers[dp.peers-1]
}

// peerParse parses a key/value field into the current Peer.
func (dp *deviceParser) peerParse(key, value string) {
	p := dp.curPeer()
	switch key {
	case "preshared_key":
		p.PresharedKey = dp.parseKey(value)
	case "endpoint":
		p.Endpoint = dp.parseAddr(value)
	case "last_handshake_time_sec":
		dp.hsSec = dp.parseInt(value)
	case "last_handshake_time_nsec":
		dp.hsNano = dp.parseInt(value)

		// Assume that we've seen both seconds and nanoseconds and populate this
		// field now. However, if both fields were set to 0, assume we have never
		// had a successful handshake with this peer, and return a zero-value
		// time.Time to our callers.
		if dp.hsSec > 0 && dp.hsNano > 0 {
			p.LastHandshakeTime = time.Unix(int64(dp.hsSec), int64(dp.hsNano))
		}
	case "tx_bytes":
		p.TransmitBytes = dp.parseInt64(value)
	case "rx_bytes":
		p.ReceiveBytes = dp.parseInt64(value)
	case "persistent_keepalive_interval":
		p.PersistentKeepaliveInterval = time.Duration(dp.parseInt(value)) * time.Second
	case "allowed_ip":
		cidr := dp.parseCIDR(value)
		if cidr != nil {
			p.AllowedIPs = append(p.AllowedIPs, *cidr)
		}
	case "protocol_version":
		p.ProtocolVersion = dp.parseInt(value)
	}
}

// parseKey parses a Key from a hex string.
func (dp *deviceParser) parseKey(s string) wgtypes.Key {
	if dp.err != nil {
		return wgtypes.Key{}
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		dp.err = err
		return wgtypes.Key{}
	}

	key, err := wgtypes.NewKey(b)
	if err != nil {
		dp.err = err
		return wgtypes.Key{}
	}

	return key
}

// parseInt parses an integer from a string.
func (dp *deviceParser) parseInt(s string) int {
	if dp.err != nil {
		return 0
	}

	v, err := strconv.Atoi(s)
	if err != nil {
		dp.err = err
		return 0
	}

	return v
}

// parseInt64 parses an int64 from a string.
func (dp *deviceParser) parseInt64(s string) int64 {
	if dp.err != nil {
		return 0
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		dp.err = err
		return 0
	}

	return v
}

// parseAddr parses a UDP address from a string.
func (dp *deviceParser) parseAddr(s string) *net.UDPAddr {
	if dp.err != nil {
		return nil
	}

	addr, err := net.ResolveUDPAddr("udp", s)
	if err != nil {
		dp.err = err
		return nil
	}

	return addr
}

// parseInt parses an address CIDR from a string.
func (dp *deviceParser) parseCIDR(s string) *net.IPNet {
	if dp.err != nil {
		return nil
	}

	_, cidr, err := net.ParseCIDR(s)
	if err != nil {
		dp.err = err
		return nil
	}

	return cidr
}
