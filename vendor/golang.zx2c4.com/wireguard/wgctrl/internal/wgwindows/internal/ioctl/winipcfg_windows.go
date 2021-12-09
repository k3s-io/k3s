/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2021 WireGuard LLC. All Rights Reserved.
 */

package ioctl

import (
	"encoding/binary"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

// AddressFamily enumeration specifies protocol family and is one of the windows.AF_* constants.
type AddressFamily uint16

// RawSockaddrInet union contains an IPv4, an IPv6 address, or an address family.
// https://docs.microsoft.com/en-us/windows/desktop/api/ws2ipdef/ns-ws2ipdef-_sockaddr_inet
type RawSockaddrInet struct {
	Family AddressFamily
	data   [26]byte
}

func ntohs(i uint16) uint16 {
	return binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&i))[:])
}

func htons(i uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, i)
	return *(*uint16)(unsafe.Pointer(&b[0]))
}

// SetIP method sets family, address, and port to the given IPv4 or IPv6 address and port.
// All other members of the structure are set to zero.
func (addr *RawSockaddrInet) SetIP(ip net.IP, port uint16) error {
	if v4 := ip.To4(); v4 != nil {
		addr4 := (*windows.RawSockaddrInet4)(unsafe.Pointer(addr))
		addr4.Family = windows.AF_INET
		copy(addr4.Addr[:], v4)
		addr4.Port = htons(port)
		for i := 0; i < 8; i++ {
			addr4.Zero[i] = 0
		}
		return nil
	}

	if v6 := ip.To16(); v6 != nil {
		addr6 := (*windows.RawSockaddrInet6)(unsafe.Pointer(addr))
		addr6.Family = windows.AF_INET6
		addr6.Port = htons(port)
		addr6.Flowinfo = 0
		copy(addr6.Addr[:], v6)
		addr6.Scope_id = 0
		return nil
	}

	return windows.ERROR_INVALID_PARAMETER
}

// IP returns IPv4 or IPv6 address, or nil if the address is neither.
func (addr *RawSockaddrInet) IP() net.IP {
	switch addr.Family {
	case windows.AF_INET:
		return (*windows.RawSockaddrInet4)(unsafe.Pointer(addr)).Addr[:]

	case windows.AF_INET6:
		return (*windows.RawSockaddrInet6)(unsafe.Pointer(addr)).Addr[:]
	}

	return nil
}

// Port returns the port if the address if IPv4 or IPv6, or 0 if neither.
func (addr *RawSockaddrInet) Port() uint16 {
	switch addr.Family {
	case windows.AF_INET:
		return ntohs((*windows.RawSockaddrInet4)(unsafe.Pointer(addr)).Port)

	case windows.AF_INET6:
		return ntohs((*windows.RawSockaddrInet6)(unsafe.Pointer(addr)).Port)
	}

	return 0
}
