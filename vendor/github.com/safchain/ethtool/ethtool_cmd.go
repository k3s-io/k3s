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
	"math"
	"reflect"
	"syscall"
	"unsafe"
)

type EthtoolCmd struct { /* ethtool.c: struct ethtool_cmd */
	Cmd            uint32
	Supported      uint32
	Advertising    uint32
	Speed          uint16
	Duplex         uint8
	Port           uint8
	Phy_address    uint8
	Transceiver    uint8
	Autoneg        uint8
	Mdio_support   uint8
	Maxtxpkt       uint32
	Maxrxpkt       uint32
	Speed_hi       uint16
	Eth_tp_mdix    uint8
	Reserved2      uint8
	Lp_advertising uint32
	Reserved       [2]uint32
}

// CmdGet returns the interface settings in the receiver struct
// and returns speed
func (ecmd *EthtoolCmd) CmdGet(intf string) (uint32, error) {
	e, err := NewEthtool()
	if err != nil {
		return 0, err
	}
	defer e.Close()
	return e.CmdGet(ecmd, intf)
}

// CmdSet sets and returns the settings in the receiver struct
// and returns speed
func (ecmd *EthtoolCmd) CmdSet(intf string) (uint32, error) {
	e, err := NewEthtool()
	if err != nil {
		return 0, err
	}
	defer e.Close()
	return e.CmdSet(ecmd, intf)
}

func (f *EthtoolCmd) reflect(retv *map[string]uint64) {
	val := reflect.ValueOf(f).Elem()

	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)

		t := valueField.Interface()
		//tt := reflect.TypeOf(t)
		//fmt.Printf(" t %T %v  tt %T %v\n", t, t, tt, tt)
		switch t.(type) {
		case uint32:
			//fmt.Printf("    t is uint32\n")
			(*retv)[typeField.Name] = uint64(t.(uint32))
		case uint16:
			(*retv)[typeField.Name] = uint64(t.(uint16))
		case uint8:
			(*retv)[typeField.Name] = uint64(t.(uint8))
		case int32:
			(*retv)[typeField.Name] = uint64(t.(int32))
		case int16:
			(*retv)[typeField.Name] = uint64(t.(int16))
		case int8:
			(*retv)[typeField.Name] = uint64(t.(int8))
		default:
			(*retv)[typeField.Name+"_unknown_type"] = 0
		}

		//tag := typeField.Tag
		//fmt.Printf("Field Name: %s,\t Field Value: %v,\t Tag Value: %s\n",
		//	typeField.Name, valueField.Interface(), tag.Get("tag_name"))
	}
}

// CmdGet returns the interface settings in the receiver struct
// and returns speed
func (e *Ethtool) CmdGet(ecmd *EthtoolCmd, intf string) (uint32, error) {
	ecmd.Cmd = ETHTOOL_GSET

	var name [IFNAMSIZ]byte
	copy(name[:], []byte(intf))

	ifr := ifreq{
		ifr_name: name,
		ifr_data: uintptr(unsafe.Pointer(ecmd)),
	}

	_, _, ep := syscall.Syscall(syscall.SYS_IOCTL, uintptr(e.fd),
		SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return 0, syscall.Errno(ep)
	}

	var speedval uint32 = (uint32(ecmd.Speed_hi) << 16) |
		(uint32(ecmd.Speed) & 0xffff)
	if speedval == math.MaxUint16 {
		speedval = math.MaxUint32
	}

	return speedval, nil
}

// CmdSet sets and returns the settings in the receiver struct
// and returns speed
func (e *Ethtool) CmdSet(ecmd *EthtoolCmd, intf string) (uint32, error) {
	ecmd.Cmd = ETHTOOL_SSET

	var name [IFNAMSIZ]byte
	copy(name[:], []byte(intf))

	ifr := ifreq{
		ifr_name: name,
		ifr_data: uintptr(unsafe.Pointer(ecmd)),
	}

	_, _, ep := syscall.Syscall(syscall.SYS_IOCTL, uintptr(e.fd),
		SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return 0, syscall.Errno(ep)
	}

	var speedval uint32 = (uint32(ecmd.Speed_hi) << 16) |
		(uint32(ecmd.Speed) & 0xffff)
	if speedval == math.MaxUint16 {
		speedval = math.MaxUint32
	}

	return speedval, nil
}

// CmdGetMapped returns the interface settings in a map
func (e *Ethtool) CmdGetMapped(intf string) (map[string]uint64, error) {
	ecmd := EthtoolCmd{
		Cmd: ETHTOOL_GSET,
	}

	var name [IFNAMSIZ]byte
	copy(name[:], []byte(intf))

	ifr := ifreq{
		ifr_name: name,
		ifr_data: uintptr(unsafe.Pointer(&ecmd)),
	}

	_, _, ep := syscall.Syscall(syscall.SYS_IOCTL, uintptr(e.fd),
		SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return nil, syscall.Errno(ep)
	}

	var result = make(map[string]uint64)

	// ref https://gist.github.com/drewolson/4771479
	// Golang Reflection Example
	ecmd.reflect(&result)

	var speedval uint32 = (uint32(ecmd.Speed_hi) << 16) |
		(uint32(ecmd.Speed) & 0xffff)
	result["speed"] = uint64(speedval)

	return result, nil
}

func CmdGetMapped(intf string) (map[string]uint64, error) {
	e, err := NewEthtool()
	if err != nil {
		return nil, err
	}
	defer e.Close()
	return e.CmdGetMapped(intf)
}
