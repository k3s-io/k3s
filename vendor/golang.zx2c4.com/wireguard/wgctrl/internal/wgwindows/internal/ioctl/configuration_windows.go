/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2017-2021 WireGuard LLC. All Rights Reserved.
 */

package ioctl

import "unsafe"

const (
	IoctlGet = 0xb098c506
	IoctlSet = 0xb098c509
)

type AllowedIP struct {
	Address       [16]byte
	AddressFamily AddressFamily
	Cidr          uint8
	_             [4]byte
}

type PeerFlag uint32

const (
	PeerHasPublicKey           PeerFlag = 1 << 0
	PeerHasPresharedKey        PeerFlag = 1 << 1
	PeerHasPersistentKeepalive PeerFlag = 1 << 2
	PeerHasEndpoint            PeerFlag = 1 << 3
	PeerHasProtocolVersion     PeerFlag = 1 << 4
	PeerReplaceAllowedIPs      PeerFlag = 1 << 5
	PeerRemove                 PeerFlag = 1 << 6
	PeerUpdate                 PeerFlag = 1 << 7
)

type Peer struct {
	Flags               PeerFlag
	ProtocolVersion     uint32
	PublicKey           [32]byte
	PresharedKey        [32]byte
	PersistentKeepalive uint16
	_                   uint16
	Endpoint            RawSockaddrInet
	TxBytes             uint64
	RxBytes             uint64
	LastHandshake       uint64
	AllowedIPsCount     uint32
	_                   [4]byte
}

type InterfaceFlag uint32

const (
	InterfaceHasPublicKey  InterfaceFlag = 1 << 0
	InterfaceHasPrivateKey InterfaceFlag = 1 << 1
	InterfaceHasListenPort InterfaceFlag = 1 << 2
	InterfaceReplacePeers  InterfaceFlag = 1 << 3
)

type Interface struct {
	Flags      InterfaceFlag
	ListenPort uint16
	PrivateKey [32]byte
	PublicKey  [32]byte
	PeerCount  uint32
	_          [4]byte
}

func (interfaze *Interface) FirstPeer() *Peer {
	return (*Peer)(unsafe.Pointer(uintptr(unsafe.Pointer(interfaze)) + unsafe.Sizeof(*interfaze)))
}

func (peer *Peer) NextPeer() *Peer {
	return (*Peer)(unsafe.Pointer(uintptr(unsafe.Pointer(peer)) + unsafe.Sizeof(*peer) + uintptr(peer.AllowedIPsCount)*unsafe.Sizeof(AllowedIP{})))
}

func (peer *Peer) FirstAllowedIP() *AllowedIP {
	return (*AllowedIP)(unsafe.Pointer(uintptr(unsafe.Pointer(peer)) + unsafe.Sizeof(*peer)))
}

func (allowedIP *AllowedIP) NextAllowedIP() *AllowedIP {
	return (*AllowedIP)(unsafe.Pointer(uintptr(unsafe.Pointer(allowedIP)) + unsafe.Sizeof(*allowedIP)))
}

type ConfigBuilder struct {
	buffer []byte
}

func (builder *ConfigBuilder) Preallocate(size uint32) {
	if builder.buffer == nil {
		builder.buffer = make([]byte, 0, size)
	}
}

func (builder *ConfigBuilder) AppendInterface(interfaze *Interface) {
	var newBytes []byte
	unsafeSlice(unsafe.Pointer(&newBytes), unsafe.Pointer(interfaze), int(unsafe.Sizeof(*interfaze)))
	builder.buffer = append(builder.buffer, newBytes...)
}

func (builder *ConfigBuilder) AppendPeer(peer *Peer) {
	var newBytes []byte
	unsafeSlice(unsafe.Pointer(&newBytes), unsafe.Pointer(peer), int(unsafe.Sizeof(*peer)))
	builder.buffer = append(builder.buffer, newBytes...)
}

func (builder *ConfigBuilder) AppendAllowedIP(allowedIP *AllowedIP) {
	var newBytes []byte
	unsafeSlice(unsafe.Pointer(&newBytes), unsafe.Pointer(allowedIP), int(unsafe.Sizeof(*allowedIP)))
	builder.buffer = append(builder.buffer, newBytes...)
}

func (builder *ConfigBuilder) Interface() (*Interface, uint32) {
	if builder.buffer == nil {
		return nil, 0
	}
	return (*Interface)(unsafe.Pointer(&builder.buffer[0])), uint32(len(builder.buffer))
}

// unsafeSlice updates the slice slicePtr to be a slice
// referencing the provided data with its length & capacity set to
// lenCap.
//
// TODO: whenGo 1.17 is the minimum supported version,
// update callers to use unsafe.Slice instead of this.
func unsafeSlice(slicePtr, data unsafe.Pointer, lenCap int) {
	type sliceHeader struct {
		Data unsafe.Pointer
		Len  int
		Cap  int
	}
	h := (*sliceHeader)(slicePtr)
	h.Data = data
	h.Len = lenCap
	h.Cap = lenCap
}
