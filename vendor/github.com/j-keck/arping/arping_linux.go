package arping

import (
	"net"
	"syscall"
	"time"
)

var sock int
var toSockaddr syscall.SockaddrLinklayer

func initialize(iface net.Interface) error {
	toSockaddr = syscall.SockaddrLinklayer{Ifindex: iface.Index}

	// 1544 = htons(ETH_P_ARP)
	const proto = 1544
	var err error
	sock, err = syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, proto)
	return err
}

func send(request arpDatagram) (time.Time, error) {
	return time.Now(), syscall.Sendto(sock, request.MarshalWithEthernetHeader(), 0, &toSockaddr)
}

func receive() (arpDatagram, time.Time, error) {
	buffer := make([]byte, 128)
	n, _, err := syscall.Recvfrom(sock, buffer, 0)
	if err != nil {
		return arpDatagram{}, time.Now(), err
	}
	// skip 14 bytes ethernet header
	return parseArpDatagram(buffer[14:n]), time.Now(), nil
}

func deinitialize() error {
	return syscall.Close(sock)
}
