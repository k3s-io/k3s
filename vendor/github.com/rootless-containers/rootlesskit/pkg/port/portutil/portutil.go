package portutil

import (
	"net"
	"strconv"
	"strings"
	"text/scanner"

	"github.com/pkg/errors"

	"github.com/rootless-containers/rootlesskit/pkg/port"
)

// ParsePortSpec parses a Docker-like representation of PortSpec, but with
// support for both "parent IP" and "child IP" (optional);
// e.g. "127.0.0.1:8080:80/tcp", or "127.0.0.1:8080:10.0.2.100:80/tcp"
//
// Format is as follows:
//
//     <parent IP>:<parent port>[:<child IP>]:<child port>/<proto>
//
// Note that (child IP being optional) the format can either contain 5 or 4
// components. When using IPv6 IP addresses, addresses must use square brackets
// to prevent the colons being mistaken for delimiters. For example:
//
//     [::1]:8080:[::2]:80/udp
func ParsePortSpec(portSpec string) (*port.Spec, error) {
	const (
		parentIP   = iota
		parentPort = iota
		childIP    = iota
		childPort  = iota
		proto      = iota
	)

	var (
		s         scanner.Scanner
		err       error
		parts     = make([]string, 5)
		index     = parentIP
		delimiter = ':'
	)

	// First get the "proto" and "parent-port" at the end. These parts are
	// required, whereas "ParentIP" is optional. Removing them first makes
	// it easier to parse the remaining parts, as otherwise the third part
	// could be _either_ an IP-address _or_ a Port.

	// Get the proto
	protoPos := strings.LastIndex(portSpec, "/")
	if protoPos < 0 {
		return nil, errors.Errorf("missing proto in PortSpec string: %q", portSpec)
	}
	parts[proto] = portSpec[protoPos+1:]
	err = validateProto(parts[proto])
	if err != nil {
		return nil, errors.Wrapf(err, "invalid PortSpec string: %q", portSpec)
	}

	// Get the parent port
	portPos := strings.LastIndex(portSpec, ":")
	if portPos < 0 {
		return nil, errors.Errorf("unexpected PortSpec string: %q", portSpec)
	}
	parts[childPort] = portSpec[portPos+1 : protoPos]

	// Scan the remainder "<IP-address>:<port>[:<IP-address>]"
	s.Init(strings.NewReader(portSpec[:portPos]))

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if index > childPort {
			return nil, errors.Errorf("unexpected PortSpec string: %q", portSpec)
		}

		switch tok {
		case '[':
			// Start of IPv6 IP-address; value ends at closing bracket (])
			delimiter = ']'
			continue
		case delimiter:
			if delimiter == ']' {
				// End of IPv6 IP-address
				delimiter = ':'
				// Skip the next token, which should be a colon delimiter (:)
				tok = s.Scan()
			}
			index++
			continue
		default:
			parts[index] += s.TokenText()
		}
	}

	if parts[parentIP] != "" && net.ParseIP(parts[parentIP]) == nil {
		return nil, errors.Errorf("unexpected ParentIP in PortSpec string: %q", portSpec)
	}
	if parts[childIP] != "" && net.ParseIP(parts[childIP]) == nil {
		return nil, errors.Errorf("unexpected ParentIP in PortSpec string: %q", portSpec)
	}

	ps := &port.Spec{
		Proto:    parts[proto],
		ParentIP: parts[parentIP],
		ChildIP:  parts[childIP],
	}

	ps.ParentPort, err = strconv.Atoi(parts[parentPort])
	if err != nil {
		return nil, errors.Wrapf(err, "unexpected ChildPort in PortSpec string: %q", portSpec)
	}

	ps.ChildPort, err = strconv.Atoi(parts[childPort])
	if err != nil {
		return nil, errors.Wrapf(err, "unexpected ParentPort in PortSpec string: %q", portSpec)
	}

	return ps, nil
}

// ValidatePortSpec validates *port.Spec.
// existingPorts can be optionally passed for detecting conflicts.
func ValidatePortSpec(spec port.Spec, existingPorts map[int]*port.Status) error {
	if err := validateProto(spec.Proto); err != nil {
		return err
	}
	if spec.ParentIP != "" {
		if net.ParseIP(spec.ParentIP) == nil {
			return errors.Errorf("invalid ParentIP: %q", spec.ParentIP)
		}
	}
	if spec.ChildIP != "" {
		if net.ParseIP(spec.ChildIP) == nil {
			return errors.Errorf("invalid ChildIP: %q", spec.ChildIP)
		}
	}
	if spec.ParentPort <= 0 || spec.ParentPort > 65535 {
		return errors.Errorf("invalid ParentPort: %q", spec.ParentPort)
	}
	if spec.ChildPort <= 0 || spec.ChildPort > 65535 {
		return errors.Errorf("invalid ChildPort: %q", spec.ChildPort)
	}
	for id, p := range existingPorts {
		sp := p.Spec
		sameProto := sp.Proto == spec.Proto
		sameParent := sp.ParentIP == spec.ParentIP && sp.ParentPort == spec.ParentPort
		if sameProto && sameParent {
			return errors.Errorf("conflict with ID %d", id)
		}
	}
	return nil
}

func validateProto(proto string) error {
	switch proto {
	case
		"tcp", "tcp4", "tcp6",
		"udp", "udp4", "udp6",
		"sctp", "sctp4", "sctp6":
		return nil
	default:
		return errors.Errorf("unknown proto: %q", proto)
	}
}
