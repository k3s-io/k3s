/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2020-2021 WireGuard LLC. All Rights Reserved.
 */

package setupapi

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

//sys	setupDiCreateDeviceInfoListEx(classGUID *windows.GUID, hwndParent uintptr, machineName *uint16, reserved uintptr) (handle DevInfo, err error) [failretval==DevInfo(windows.InvalidHandle)] = setupapi.SetupDiCreateDeviceInfoListExW

// SetupDiCreateDeviceInfoListEx function creates an empty device information set on a remote or a local computer and optionally associates the set with a device setup class.
func SetupDiCreateDeviceInfoListEx(classGUID *windows.GUID, hwndParent uintptr, machineName string) (deviceInfoSet DevInfo, err error) {
	var machineNameUTF16 *uint16
	if machineName != "" {
		machineNameUTF16, err = windows.UTF16PtrFromString(machineName)
		if err != nil {
			return
		}
	}
	return setupDiCreateDeviceInfoListEx(classGUID, hwndParent, machineNameUTF16, 0)
}

//sys	setupDiGetDeviceInfoListDetail(deviceInfoSet DevInfo, deviceInfoSetDetailData *DevInfoListDetailData) (err error) = setupapi.SetupDiGetDeviceInfoListDetailW

// SetupDiGetDeviceInfoListDetail function retrieves information associated with a device information set including the class GUID, remote computer handle, and remote computer name.
func SetupDiGetDeviceInfoListDetail(deviceInfoSet DevInfo) (deviceInfoSetDetailData *DevInfoListDetailData, err error) {
	data := &DevInfoListDetailData{}
	data.size = sizeofDevInfoListDetailData

	return data, setupDiGetDeviceInfoListDetail(deviceInfoSet, data)
}

// DeviceInfoListDetail method retrieves information associated with a device information set including the class GUID, remote computer handle, and remote computer name.
func (deviceInfoSet DevInfo) DeviceInfoListDetail() (*DevInfoListDetailData, error) {
	return SetupDiGetDeviceInfoListDetail(deviceInfoSet)
}

//sys	setupDiCreateDeviceInfo(deviceInfoSet DevInfo, DeviceName *uint16, classGUID *windows.GUID, DeviceDescription *uint16, hwndParent uintptr, CreationFlags DICD, deviceInfoData *DevInfoData) (err error) = setupapi.SetupDiCreateDeviceInfoW

// SetupDiCreateDeviceInfo function creates a new device information element and adds it as a new member to the specified device information set.
func SetupDiCreateDeviceInfo(deviceInfoSet DevInfo, deviceName string, classGUID *windows.GUID, deviceDescription string, hwndParent uintptr, creationFlags DICD) (deviceInfoData *DevInfoData, err error) {
	deviceNameUTF16, err := windows.UTF16PtrFromString(deviceName)
	if err != nil {
		return
	}

	var deviceDescriptionUTF16 *uint16
	if deviceDescription != "" {
		deviceDescriptionUTF16, err = windows.UTF16PtrFromString(deviceDescription)
		if err != nil {
			return
		}
	}

	data := &DevInfoData{}
	data.size = uint32(unsafe.Sizeof(*data))

	return data, setupDiCreateDeviceInfo(deviceInfoSet, deviceNameUTF16, classGUID, deviceDescriptionUTF16, hwndParent, creationFlags, data)
}

// CreateDeviceInfo method creates a new device information element and adds it as a new member to the specified device information set.
func (deviceInfoSet DevInfo) CreateDeviceInfo(deviceName string, classGUID *windows.GUID, deviceDescription string, hwndParent uintptr, creationFlags DICD) (*DevInfoData, error) {
	return SetupDiCreateDeviceInfo(deviceInfoSet, deviceName, classGUID, deviceDescription, hwndParent, creationFlags)
}

//sys	setupDiEnumDeviceInfo(deviceInfoSet DevInfo, memberIndex uint32, deviceInfoData *DevInfoData) (err error) = setupapi.SetupDiEnumDeviceInfo

// SetupDiEnumDeviceInfo function returns a DevInfoData structure that specifies a device information element in a device information set.
func SetupDiEnumDeviceInfo(deviceInfoSet DevInfo, memberIndex int) (*DevInfoData, error) {
	data := &DevInfoData{}
	data.size = uint32(unsafe.Sizeof(*data))

	return data, setupDiEnumDeviceInfo(deviceInfoSet, uint32(memberIndex), data)
}

// EnumDeviceInfo method returns a DevInfoData structure that specifies a device information element in a device information set.
func (deviceInfoSet DevInfo) EnumDeviceInfo(memberIndex int) (*DevInfoData, error) {
	return SetupDiEnumDeviceInfo(deviceInfoSet, memberIndex)
}

// SetupDiDestroyDeviceInfoList function deletes a device information set and frees all associated memory.
//sys	SetupDiDestroyDeviceInfoList(deviceInfoSet DevInfo) (err error) = setupapi.SetupDiDestroyDeviceInfoList

// Close method deletes a device information set and frees all associated memory.
func (deviceInfoSet DevInfo) Close() error {
	return SetupDiDestroyDeviceInfoList(deviceInfoSet)
}

//sys	SetupDiBuildDriverInfoList(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, driverType SPDIT) (err error) = setupapi.SetupDiBuildDriverInfoList

// BuildDriverInfoList method builds a list of drivers that is associated with a specific device or with the global class driver list for a device information set.
func (deviceInfoSet DevInfo) BuildDriverInfoList(deviceInfoData *DevInfoData, driverType SPDIT) error {
	return SetupDiBuildDriverInfoList(deviceInfoSet, deviceInfoData, driverType)
}

//sys	SetupDiCancelDriverInfoSearch(deviceInfoSet DevInfo) (err error) = setupapi.SetupDiCancelDriverInfoSearch

// CancelDriverInfoSearch method cancels a driver list search that is currently in progress in a different thread.
func (deviceInfoSet DevInfo) CancelDriverInfoSearch() error {
	return SetupDiCancelDriverInfoSearch(deviceInfoSet)
}

//sys	setupDiEnumDriverInfo(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, driverType SPDIT, memberIndex uint32, driverInfoData *DrvInfoData) (err error) = setupapi.SetupDiEnumDriverInfoW

// SetupDiEnumDriverInfo function enumerates the members of a driver list.
func SetupDiEnumDriverInfo(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, driverType SPDIT, memberIndex int) (*DrvInfoData, error) {
	data := &DrvInfoData{}
	data.size = uint32(unsafe.Sizeof(*data))

	return data, setupDiEnumDriverInfo(deviceInfoSet, deviceInfoData, driverType, uint32(memberIndex), data)
}

// EnumDriverInfo method enumerates the members of a driver list.
func (deviceInfoSet DevInfo) EnumDriverInfo(deviceInfoData *DevInfoData, driverType SPDIT, memberIndex int) (*DrvInfoData, error) {
	return SetupDiEnumDriverInfo(deviceInfoSet, deviceInfoData, driverType, memberIndex)
}

//sys	setupDiGetSelectedDriver(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, driverInfoData *DrvInfoData) (err error) = setupapi.SetupDiGetSelectedDriverW

// SetupDiGetSelectedDriver function retrieves the selected driver for a device information set or a particular device information element.
func SetupDiGetSelectedDriver(deviceInfoSet DevInfo, deviceInfoData *DevInfoData) (*DrvInfoData, error) {
	data := &DrvInfoData{}
	data.size = uint32(unsafe.Sizeof(*data))

	return data, setupDiGetSelectedDriver(deviceInfoSet, deviceInfoData, data)
}

// SelectedDriver method retrieves the selected driver for a device information set or a particular device information element.
func (deviceInfoSet DevInfo) SelectedDriver(deviceInfoData *DevInfoData) (*DrvInfoData, error) {
	return SetupDiGetSelectedDriver(deviceInfoSet, deviceInfoData)
}

//sys	SetupDiSetSelectedDriver(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, driverInfoData *DrvInfoData) (err error) = setupapi.SetupDiSetSelectedDriverW

// SetSelectedDriver method sets, or resets, the selected driver for a device information element or the selected class driver for a device information set.
func (deviceInfoSet DevInfo) SetSelectedDriver(deviceInfoData *DevInfoData, driverInfoData *DrvInfoData) error {
	return SetupDiSetSelectedDriver(deviceInfoSet, deviceInfoData, driverInfoData)
}

//sys	setupDiGetDriverInfoDetail(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, driverInfoData *DrvInfoData, driverInfoDetailData *DrvInfoDetailData, driverInfoDetailDataSize uint32, requiredSize *uint32) (err error) = setupapi.SetupDiGetDriverInfoDetailW

// SetupDiGetDriverInfoDetail function retrieves driver information detail for a device information set or a particular device information element in the device information set.
func SetupDiGetDriverInfoDetail(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, driverInfoData *DrvInfoData) (*DrvInfoDetailData, error) {
	reqSize := uint32(2048)
	for {
		buf := make([]byte, reqSize)
		data := (*DrvInfoDetailData)(unsafe.Pointer(&buf[0]))
		data.size = sizeofDrvInfoDetailData
		err := setupDiGetDriverInfoDetail(deviceInfoSet, deviceInfoData, driverInfoData, data, uint32(len(buf)), &reqSize)
		if err == windows.ERROR_INSUFFICIENT_BUFFER {
			continue
		}
		if err != nil {
			return nil, err
		}
		data.size = reqSize
		return data, nil
	}
}

// DriverInfoDetail method retrieves driver information detail for a device information set or a particular device information element in the device information set.
func (deviceInfoSet DevInfo) DriverInfoDetail(deviceInfoData *DevInfoData, driverInfoData *DrvInfoData) (*DrvInfoDetailData, error) {
	return SetupDiGetDriverInfoDetail(deviceInfoSet, deviceInfoData, driverInfoData)
}

//sys	SetupDiDestroyDriverInfoList(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, driverType SPDIT) (err error) = setupapi.SetupDiDestroyDriverInfoList

// DestroyDriverInfoList method deletes a driver list.
func (deviceInfoSet DevInfo) DestroyDriverInfoList(deviceInfoData *DevInfoData, driverType SPDIT) error {
	return SetupDiDestroyDriverInfoList(deviceInfoSet, deviceInfoData, driverType)
}

//sys	setupDiGetClassDevsEx(classGUID *windows.GUID, Enumerator *uint16, hwndParent uintptr, Flags DIGCF, deviceInfoSet DevInfo, machineName *uint16, reserved uintptr) (handle DevInfo, err error) [failretval==DevInfo(windows.InvalidHandle)] = setupapi.SetupDiGetClassDevsExW

// SetupDiGetClassDevsEx function returns a handle to a device information set that contains requested device information elements for a local or a remote computer.
func SetupDiGetClassDevsEx(classGUID *windows.GUID, enumerator string, hwndParent uintptr, flags DIGCF, deviceInfoSet DevInfo, machineName string) (handle DevInfo, err error) {
	var enumeratorUTF16 *uint16
	if enumerator != "" {
		enumeratorUTF16, err = windows.UTF16PtrFromString(enumerator)
		if err != nil {
			return
		}
	}
	var machineNameUTF16 *uint16
	if machineName != "" {
		machineNameUTF16, err = windows.UTF16PtrFromString(machineName)
		if err != nil {
			return
		}
	}
	return setupDiGetClassDevsEx(classGUID, enumeratorUTF16, hwndParent, flags, deviceInfoSet, machineNameUTF16, 0)
}

// SetupDiCallClassInstaller function calls the appropriate class installer, and any registered co-installers, with the specified installation request (DIF code).
//sys	SetupDiCallClassInstaller(installFunction DI_FUNCTION, deviceInfoSet DevInfo, deviceInfoData *DevInfoData) (err error) = setupapi.SetupDiCallClassInstaller

// CallClassInstaller member calls the appropriate class installer, and any registered co-installers, with the specified installation request (DIF code).
func (deviceInfoSet DevInfo) CallClassInstaller(installFunction DI_FUNCTION, deviceInfoData *DevInfoData) error {
	return SetupDiCallClassInstaller(installFunction, deviceInfoSet, deviceInfoData)
}

//sys	setupDiOpenDevRegKey(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, Scope DICS_FLAG, HwProfile uint32, KeyType DIREG, samDesired uint32) (key windows.Handle, err error) [failretval==windows.InvalidHandle] = setupapi.SetupDiOpenDevRegKey

// SetupDiOpenDevRegKey function opens a registry key for device-specific configuration information.
func SetupDiOpenDevRegKey(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, scope DICS_FLAG, hwProfile uint32, keyType DIREG, samDesired uint32) (registry.Key, error) {
	handle, err := setupDiOpenDevRegKey(deviceInfoSet, deviceInfoData, scope, hwProfile, keyType, samDesired)
	return registry.Key(handle), err
}

// OpenDevRegKey method opens a registry key for device-specific configuration information.
func (deviceInfoSet DevInfo) OpenDevRegKey(DeviceInfoData *DevInfoData, Scope DICS_FLAG, HwProfile uint32, KeyType DIREG, samDesired uint32) (registry.Key, error) {
	return SetupDiOpenDevRegKey(deviceInfoSet, DeviceInfoData, Scope, HwProfile, KeyType, samDesired)
}

//sys	setupDiGetDeviceProperty(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, propertyKey *DEVPROPKEY, propertyType *DEVPROPTYPE, propertyBuffer *byte, propertyBufferSize uint32, requiredSize *uint32, flags uint32) (err error) = setupapi.SetupDiGetDevicePropertyW

// SetupDiGetDeviceProperty function retrieves a specified device instance property.
func SetupDiGetDeviceProperty(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, propertyKey *DEVPROPKEY) (value interface{}, err error) {
	reqSize := uint32(256)
	for {
		var dataType DEVPROPTYPE
		buf := make([]byte, reqSize)
		err = setupDiGetDeviceProperty(deviceInfoSet, deviceInfoData, propertyKey, &dataType, &buf[0], uint32(len(buf)), &reqSize, 0)
		if err == windows.ERROR_INSUFFICIENT_BUFFER {
			continue
		}
		if err != nil {
			return
		}
		switch dataType {
		case DEVPROP_TYPE_STRING:
			ret := windows.UTF16ToString(bufToUTF16(buf))
			runtime.KeepAlive(buf)
			return ret, nil
		}
		return nil, errors.New("unimplemented property type")
	}
}

//sys	setupDiGetDeviceRegistryProperty(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, property SPDRP, propertyRegDataType *uint32, propertyBuffer *byte, propertyBufferSize uint32, requiredSize *uint32) (err error) = setupapi.SetupDiGetDeviceRegistryPropertyW

// SetupDiGetDeviceRegistryProperty function retrieves a specified Plug and Play device property.
func SetupDiGetDeviceRegistryProperty(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, property SPDRP) (value interface{}, err error) {
	reqSize := uint32(256)
	for {
		var dataType uint32
		buf := make([]byte, reqSize)
		err = setupDiGetDeviceRegistryProperty(deviceInfoSet, deviceInfoData, property, &dataType, &buf[0], uint32(len(buf)), &reqSize)
		if err == windows.ERROR_INSUFFICIENT_BUFFER {
			continue
		}
		if err != nil {
			return
		}
		return getRegistryValue(buf[:reqSize], dataType)
	}
}

func getRegistryValue(buf []byte, dataType uint32) (interface{}, error) {
	switch dataType {
	case windows.REG_SZ:
		ret := windows.UTF16ToString(bufToUTF16(buf))
		runtime.KeepAlive(buf)
		return ret, nil
	case windows.REG_EXPAND_SZ:
		ret, err := registry.ExpandString(windows.UTF16ToString(bufToUTF16(buf)))
		runtime.KeepAlive(buf)
		return ret, err
	case windows.REG_BINARY:
		return buf, nil
	case windows.REG_DWORD_LITTLE_ENDIAN:
		return binary.LittleEndian.Uint32(buf), nil
	case windows.REG_DWORD_BIG_ENDIAN:
		return binary.BigEndian.Uint32(buf), nil
	case windows.REG_MULTI_SZ:
		bufW := bufToUTF16(buf)
		a := []string{}
		for i := 0; i < len(bufW); {
			j := i + wcslen(bufW[i:])
			if i < j {
				a = append(a, windows.UTF16ToString(bufW[i:j]))
			}
			i = j + 1
		}
		runtime.KeepAlive(buf)
		return a, nil
	case windows.REG_QWORD_LITTLE_ENDIAN:
		return binary.LittleEndian.Uint64(buf), nil
	default:
		return nil, fmt.Errorf("Unsupported registry value type: %v", dataType)
	}
}

// bufToUTF16 function reinterprets []byte buffer as []uint16
func bufToUTF16(buf []byte) []uint16 {
	sl := struct {
		addr *uint16
		len  int
		cap  int
	}{(*uint16)(unsafe.Pointer(&buf[0])), len(buf) / 2, cap(buf) / 2}
	return *(*[]uint16)(unsafe.Pointer(&sl))
}

// utf16ToBuf function reinterprets []uint16 as []byte
func utf16ToBuf(buf []uint16) []byte {
	sl := struct {
		addr *byte
		len  int
		cap  int
	}{(*byte)(unsafe.Pointer(&buf[0])), len(buf) * 2, cap(buf) * 2}
	return *(*[]byte)(unsafe.Pointer(&sl))
}

func wcslen(str []uint16) int {
	for i := 0; i < len(str); i++ {
		if str[i] == 0 {
			return i
		}
	}
	return len(str)
}

// DeviceRegistryProperty method retrieves a specified Plug and Play device property.
func (deviceInfoSet DevInfo) DeviceRegistryProperty(deviceInfoData *DevInfoData, property SPDRP) (interface{}, error) {
	return SetupDiGetDeviceRegistryProperty(deviceInfoSet, deviceInfoData, property)
}

//sys	setupDiSetDeviceRegistryProperty(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, property SPDRP, propertyBuffer *byte, propertyBufferSize uint32) (err error) = setupapi.SetupDiSetDeviceRegistryPropertyW

// SetupDiSetDeviceRegistryProperty function sets a Plug and Play device property for a device.
func SetupDiSetDeviceRegistryProperty(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, property SPDRP, propertyBuffers []byte) error {
	return setupDiSetDeviceRegistryProperty(deviceInfoSet, deviceInfoData, property, &propertyBuffers[0], uint32(len(propertyBuffers)))
}

// SetDeviceRegistryProperty function sets a Plug and Play device property for a device.
func (deviceInfoSet DevInfo) SetDeviceRegistryProperty(deviceInfoData *DevInfoData, property SPDRP, propertyBuffers []byte) error {
	return SetupDiSetDeviceRegistryProperty(deviceInfoSet, deviceInfoData, property, propertyBuffers)
}

// SetDeviceRegistryPropertyString method sets a Plug and Play device property string for a device.
func (deviceInfoSet DevInfo) SetDeviceRegistryPropertyString(deviceInfoData *DevInfoData, property SPDRP, str string) error {
	str16, err := windows.UTF16FromString(str)
	if err != nil {
		return err
	}
	err = SetupDiSetDeviceRegistryProperty(deviceInfoSet, deviceInfoData, property, utf16ToBuf(append(str16, 0)))
	runtime.KeepAlive(str16)
	return err
}

//sys	setupDiGetDeviceInstallParams(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, deviceInstallParams *DevInstallParams) (err error) = setupapi.SetupDiGetDeviceInstallParamsW

// SetupDiGetDeviceInstallParams function retrieves device installation parameters for a device information set or a particular device information element.
func SetupDiGetDeviceInstallParams(deviceInfoSet DevInfo, deviceInfoData *DevInfoData) (*DevInstallParams, error) {
	params := &DevInstallParams{}
	params.size = uint32(unsafe.Sizeof(*params))

	return params, setupDiGetDeviceInstallParams(deviceInfoSet, deviceInfoData, params)
}

// DeviceInstallParams method retrieves device installation parameters for a device information set or a particular device information element.
func (deviceInfoSet DevInfo) DeviceInstallParams(deviceInfoData *DevInfoData) (*DevInstallParams, error) {
	return SetupDiGetDeviceInstallParams(deviceInfoSet, deviceInfoData)
}

//sys	setupDiGetDeviceInstanceId(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, instanceId *uint16, instanceIdSize uint32, instanceIdRequiredSize *uint32) (err error) = setupapi.SetupDiGetDeviceInstanceIdW

// SetupDiGetDeviceInstanceId function retrieves the instance ID of the device.
func SetupDiGetDeviceInstanceId(deviceInfoSet DevInfo, deviceInfoData *DevInfoData) (string, error) {
	reqSize := uint32(1024)
	for {
		buf := make([]uint16, reqSize)
		err := setupDiGetDeviceInstanceId(deviceInfoSet, deviceInfoData, &buf[0], uint32(len(buf)), &reqSize)
		if err == windows.ERROR_INSUFFICIENT_BUFFER {
			continue
		}
		if err != nil {
			return "", err
		}
		return windows.UTF16ToString(buf), nil
	}
}

// DeviceInstanceID method retrieves the instance ID of the device.
func (deviceInfoSet DevInfo) DeviceInstanceID(deviceInfoData *DevInfoData) (string, error) {
	return SetupDiGetDeviceInstanceId(deviceInfoSet, deviceInfoData)
}

// SetupDiGetClassInstallParams function retrieves class installation parameters for a device information set or a particular device information element.
//sys	SetupDiGetClassInstallParams(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, classInstallParams *ClassInstallHeader, classInstallParamsSize uint32, requiredSize *uint32) (err error) = setupapi.SetupDiGetClassInstallParamsW

// ClassInstallParams method retrieves class installation parameters for a device information set or a particular device information element.
func (deviceInfoSet DevInfo) ClassInstallParams(deviceInfoData *DevInfoData, classInstallParams *ClassInstallHeader, classInstallParamsSize uint32, requiredSize *uint32) error {
	return SetupDiGetClassInstallParams(deviceInfoSet, deviceInfoData, classInstallParams, classInstallParamsSize, requiredSize)
}

//sys	SetupDiSetDeviceInstallParams(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, deviceInstallParams *DevInstallParams) (err error) = setupapi.SetupDiSetDeviceInstallParamsW

// SetDeviceInstallParams member sets device installation parameters for a device information set or a particular device information element.
func (deviceInfoSet DevInfo) SetDeviceInstallParams(deviceInfoData *DevInfoData, deviceInstallParams *DevInstallParams) error {
	return SetupDiSetDeviceInstallParams(deviceInfoSet, deviceInfoData, deviceInstallParams)
}

// SetupDiSetClassInstallParams function sets or clears class install parameters for a device information set or a particular device information element.
//sys	SetupDiSetClassInstallParams(deviceInfoSet DevInfo, deviceInfoData *DevInfoData, classInstallParams *ClassInstallHeader, classInstallParamsSize uint32) (err error) = setupapi.SetupDiSetClassInstallParamsW

// SetClassInstallParams method sets or clears class install parameters for a device information set or a particular device information element.
func (deviceInfoSet DevInfo) SetClassInstallParams(deviceInfoData *DevInfoData, classInstallParams *ClassInstallHeader, classInstallParamsSize uint32) error {
	return SetupDiSetClassInstallParams(deviceInfoSet, deviceInfoData, classInstallParams, classInstallParamsSize)
}

//sys	setupDiClassNameFromGuidEx(classGUID *windows.GUID, className *uint16, classNameSize uint32, requiredSize *uint32, machineName *uint16, reserved uintptr) (err error) = setupapi.SetupDiClassNameFromGuidExW

// SetupDiClassNameFromGuidEx function retrieves the class name associated with a class GUID. The class can be installed on a local or remote computer.
func SetupDiClassNameFromGuidEx(classGUID *windows.GUID, machineName string) (className string, err error) {
	var classNameUTF16 [MAX_CLASS_NAME_LEN]uint16

	var machineNameUTF16 *uint16
	if machineName != "" {
		machineNameUTF16, err = windows.UTF16PtrFromString(machineName)
		if err != nil {
			return
		}
	}

	err = setupDiClassNameFromGuidEx(classGUID, &classNameUTF16[0], MAX_CLASS_NAME_LEN, nil, machineNameUTF16, 0)
	if err != nil {
		return
	}

	className = windows.UTF16ToString(classNameUTF16[:])
	return
}

//sys	setupDiClassGuidsFromNameEx(className *uint16, classGuidList *windows.GUID, classGuidListSize uint32, requiredSize *uint32, machineName *uint16, reserved uintptr) (err error) = setupapi.SetupDiClassGuidsFromNameExW

// SetupDiClassGuidsFromNameEx function retrieves the GUIDs associated with the specified class name. This resulting list contains the classes currently installed on a local or remote computer.
func SetupDiClassGuidsFromNameEx(className string, machineName string) ([]windows.GUID, error) {
	classNameUTF16, err := windows.UTF16PtrFromString(className)
	if err != nil {
		return nil, err
	}

	var machineNameUTF16 *uint16
	if machineName != "" {
		machineNameUTF16, err = windows.UTF16PtrFromString(machineName)
		if err != nil {
			return nil, err
		}
	}

	reqSize := uint32(4)
	for {
		buf := make([]windows.GUID, reqSize)
		err = setupDiClassGuidsFromNameEx(classNameUTF16, &buf[0], uint32(len(buf)), &reqSize, machineNameUTF16, 0)
		if err == windows.ERROR_INSUFFICIENT_BUFFER {
			continue
		}
		if err != nil {
			return nil, err
		}
		return buf[:reqSize], nil
	}
}

//sys	setupDiGetSelectedDevice(deviceInfoSet DevInfo, deviceInfoData *DevInfoData) (err error) = setupapi.SetupDiGetSelectedDevice

// SetupDiGetSelectedDevice function retrieves the selected device information element in a device information set.
func SetupDiGetSelectedDevice(deviceInfoSet DevInfo) (*DevInfoData, error) {
	data := &DevInfoData{}
	data.size = uint32(unsafe.Sizeof(*data))

	return data, setupDiGetSelectedDevice(deviceInfoSet, data)
}

// SelectedDevice method retrieves the selected device information element in a device information set.
func (deviceInfoSet DevInfo) SelectedDevice() (*DevInfoData, error) {
	return SetupDiGetSelectedDevice(deviceInfoSet)
}

// SetupDiSetSelectedDevice function sets a device information element as the selected member of a device information set. This function is typically used by an installation wizard.
//sys	SetupDiSetSelectedDevice(deviceInfoSet DevInfo, deviceInfoData *DevInfoData) (err error) = setupapi.SetupDiSetSelectedDevice

// SetSelectedDevice method sets a device information element as the selected member of a device information set. This function is typically used by an installation wizard.
func (deviceInfoSet DevInfo) SetSelectedDevice(deviceInfoData *DevInfoData) error {
	return SetupDiSetSelectedDevice(deviceInfoSet, deviceInfoData)
}

//sys cm_Get_Device_Interface_List_Size(len *uint32, interfaceClass *windows.GUID, deviceID *uint16, flags uint32) (ret uint32) = CfgMgr32.CM_Get_Device_Interface_List_SizeW
//sys cm_Get_Device_Interface_List(interfaceClass *windows.GUID, deviceID *uint16, buffer *uint16, bufferLen uint32, flags uint32) (ret uint32) = CfgMgr32.CM_Get_Device_Interface_ListW

func CM_Get_Device_Interface_List(deviceID string, interfaceClass *windows.GUID, flags uint32) ([]string, error) {
	deviceID16, err := windows.UTF16PtrFromString(deviceID)
	if err != nil {
		return nil, err
	}
	var buf []uint16
	var buflen uint32
	for {
		if ret := cm_Get_Device_Interface_List_Size(&buflen, interfaceClass, deviceID16, flags); ret != CR_SUCCESS {
			return nil, fmt.Errorf("CfgMgr error: 0x%x", ret)
		}
		buf = make([]uint16, buflen)
		if ret := cm_Get_Device_Interface_List(interfaceClass, deviceID16, &buf[0], buflen, flags); ret == CR_SUCCESS {
			break
		} else if ret != CR_BUFFER_SMALL {
			return nil, fmt.Errorf("CfgMgr error: 0x%x", ret)
		}
	}
	var interfaces []string
	for i := 0; i < len(buf); {
		j := i + wcslen(buf[i:])
		if i < j {
			interfaces = append(interfaces, windows.UTF16ToString(buf[i:j]))
		}
		i = j + 1
	}
	if interfaces == nil {
		return nil, fmt.Errorf("no interfaces found")
	}
	return interfaces, nil
}

//sys CM_Get_DevNode_Status(status *uint32, problemNumber *uint32, devInst uint32, flags uint32) (ret uint32) = CfgMgr32.CM_Get_DevNode_Status
