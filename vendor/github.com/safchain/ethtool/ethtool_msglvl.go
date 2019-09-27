/*
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

// Package ethtool  aims to provide a library giving a simple access to the
// Linux SIOCETHTOOL ioctl operations. It can be used to retrieve informations
// from a network device like statistics, driver related informations or
// even the peer of a VETH interface.
package ethtool

import (
	"syscall"
	"unsafe"
)

type ethtoolValue struct { /* ethtool.c: struct ethtool_value */
	cmd  uint32
	data uint32
}

// MsglvlGet returns the msglvl of the given interface.
func (e *Ethtool) MsglvlGet(intf string) (uint32, error) {
	edata := ethtoolValue{
		cmd: ETHTOOL_GMSGLVL,
	}

	var name [IFNAMSIZ]byte
	copy(name[:], []byte(intf))

	ifr := ifreq{
		ifr_name: name,
		ifr_data: uintptr(unsafe.Pointer(&edata)),
	}

	_, _, ep := syscall.Syscall(syscall.SYS_IOCTL, uintptr(e.fd),
		SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return 0, syscall.Errno(ep)
	}

	return edata.data, nil
}

// MsglvlSet returns the read-msglvl, post-set-msglvl of the given interface.
func (e *Ethtool) MsglvlSet(intf string, valset uint32) (uint32, uint32, error) {
	edata := ethtoolValue{
		cmd: ETHTOOL_GMSGLVL,
	}

	var name [IFNAMSIZ]byte
	copy(name[:], []byte(intf))

	ifr := ifreq{
		ifr_name: name,
		ifr_data: uintptr(unsafe.Pointer(&edata)),
	}

	_, _, ep := syscall.Syscall(syscall.SYS_IOCTL, uintptr(e.fd),
		SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return 0, 0, syscall.Errno(ep)
	}

	readval := edata.data

	edata.cmd = ETHTOOL_SMSGLVL
	edata.data = valset

	_, _, ep = syscall.Syscall(syscall.SYS_IOCTL, uintptr(e.fd),
		SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return 0, 0, syscall.Errno(ep)
	}

	return readval, edata.data, nil
}

// MsglvlGet returns the msglvl of the given interface.
func MsglvlGet(intf string) (uint32, error) {
	e, err := NewEthtool()
	if err != nil {
		return 0, err
	}
	defer e.Close()
	return e.MsglvlGet(intf)
}

// MsglvlSet returns the read-msglvl, post-set-msglvl of the given interface.
func MsglvlSet(intf string, valset uint32) (uint32, uint32, error) {
	e, err := NewEthtool()
	if err != nil {
		return 0, 0, err
	}
	defer e.Close()
	return e.MsglvlSet(intf, valset)
}
