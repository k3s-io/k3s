package wgwindows

import (
	"net"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/wgctrl/internal/wginternal"
	"golang.zx2c4.com/wireguard/wgctrl/internal/wgwindows/internal/ioctl"
	"golang.zx2c4.com/wireguard/wgctrl/internal/wgwindows/internal/setupapi"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var _ wginternal.Client = &Client{}

// A Client provides access to WireGuardNT ioctl information.
type Client struct {
	cachedAdapters map[string]string
	lastLenGuess   uint32
}

var (
	deviceClassNetGUID     = windows.GUID{0x4d36e972, 0xe325, 0x11ce, [8]byte{0xbf, 0xc1, 0x08, 0x00, 0x2b, 0xe1, 0x03, 0x18}}
	deviceInterfaceNetGUID = windows.GUID{0xcac88484, 0x7515, 0x4c03, [8]byte{0x82, 0xe6, 0x71, 0xa8, 0x7a, 0xba, 0xc3, 0x61}}
	devpkeyWgName          = setupapi.DEVPROPKEY{
		FmtID: setupapi.DEVPROPGUID{0x65726957, 0x7547, 0x7261, [8]byte{0x64, 0x4e, 0x61, 0x6d, 0x65, 0x4b, 0x65, 0x79}},
		PID:   setupapi.DEVPROPID_FIRST_USABLE + 1,
	}
)

var enumerator = `SWD\WireGuard`

func init() {
	if maj, min, _ := windows.RtlGetNtVersionNumbers(); (maj == 6 && min <= 1) || maj < 6 {
		enumerator = `ROOT\WIREGUARD`
	}
}

func (c *Client) refreshInstanceIdCache() error {
	cachedAdapters := make(map[string]string, 5)
	devInfo, err := setupapi.SetupDiGetClassDevsEx(&deviceClassNetGUID, enumerator, 0, setupapi.DIGCF_PRESENT, 0, "")
	if err != nil {
		return err
	}
	defer setupapi.SetupDiDestroyDeviceInfoList(devInfo)
	for i := 0; ; i++ {
		devInfoData, err := setupapi.SetupDiEnumDeviceInfo(devInfo, i)
		if err != nil {
			if err == windows.ERROR_NO_MORE_ITEMS {
				break
			}
			continue
		}
		prop, err := setupapi.SetupDiGetDeviceProperty(devInfo, devInfoData, &devpkeyWgName)
		if err != nil {
			continue
		}
		adapterName, ok := prop.(string)
		if !ok {
			continue
		}
		var status, problemCode uint32
		ret := setupapi.CM_Get_DevNode_Status(&status, &problemCode, devInfoData.DevInst, 0)
		if ret != setupapi.CR_SUCCESS || (status&setupapi.DN_DRIVER_LOADED|setupapi.DN_STARTED) != setupapi.DN_DRIVER_LOADED|setupapi.DN_STARTED {
			continue
		}
		instanceId, err := setupapi.SetupDiGetDeviceInstanceId(devInfo, devInfoData)
		if err != nil {
			continue
		}
		cachedAdapters[adapterName] = instanceId
	}
	c.cachedAdapters = cachedAdapters
	return nil
}

func (c *Client) interfaceHandle(name string) (windows.Handle, error) {
	instanceId, ok := c.cachedAdapters[name]
	if !ok {
		err := c.refreshInstanceIdCache()
		if err != nil {
			return 0, err
		}
		instanceId, ok = c.cachedAdapters[name]
		if !ok {
			return 0, os.ErrNotExist
		}
	}
	interfaces, err := setupapi.CM_Get_Device_Interface_List(instanceId, &deviceInterfaceNetGUID, setupapi.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
	if err != nil {
		return 0, err
	}
	interface16, err := windows.UTF16PtrFromString(interfaces[0])
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(interface16, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
}

// Devices implements wginternal.Client.
func (c *Client) Devices() ([]*wgtypes.Device, error) {
	err := c.refreshInstanceIdCache()
	if err != nil {
		return nil, err
	}
	ds := make([]*wgtypes.Device, 0, len(c.cachedAdapters))
	for name := range c.cachedAdapters {
		d, err := c.Device(name)
		if err != nil {
			return nil, err
		}
		ds = append(ds, d)
	}
	return ds, nil
}

// New creates a new Client
func New() *Client {
	return &Client{}
}

// Close implements wginternal.Client.
func (c *Client) Close() error {
	return nil
}

// Device implements wginternal.Client.
func (c *Client) Device(name string) (*wgtypes.Device, error) {
	handle, err := c.interfaceHandle(name)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)

	size := c.lastLenGuess
	if size == 0 {
		size = 512
	}
	var buf []byte
	for {
		buf = make([]byte, size)
		err = windows.DeviceIoControl(handle, ioctl.IoctlGet, nil, 0, &buf[0], size, &size, nil)
		if err == windows.ERROR_MORE_DATA {
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}
	c.lastLenGuess = size
	interfaze := (*ioctl.Interface)(unsafe.Pointer(&buf[0]))

	device := wgtypes.Device{Type: wgtypes.WindowsKernel, Name: name}
	if interfaze.Flags&ioctl.InterfaceHasPrivateKey != 0 {
		device.PrivateKey = interfaze.PrivateKey
	}
	if interfaze.Flags&ioctl.InterfaceHasPublicKey != 0 {
		device.PublicKey = interfaze.PublicKey
	}
	if interfaze.Flags&ioctl.InterfaceHasListenPort != 0 {
		device.ListenPort = int(interfaze.ListenPort)
	}
	var p *ioctl.Peer
	for i := uint32(0); i < interfaze.PeerCount; i++ {
		if p == nil {
			p = interfaze.FirstPeer()
		} else {
			p = p.NextPeer()
		}
		peer := wgtypes.Peer{}
		if p.Flags&ioctl.PeerHasPublicKey != 0 {
			peer.PublicKey = p.PublicKey
		}
		if p.Flags&ioctl.PeerHasPresharedKey != 0 {
			peer.PresharedKey = p.PresharedKey
		}
		if p.Flags&ioctl.PeerHasEndpoint != 0 {
			peer.Endpoint = &net.UDPAddr{IP: p.Endpoint.IP(), Port: int(p.Endpoint.Port())}
		}
		if p.Flags&ioctl.PeerHasPersistentKeepalive != 0 {
			peer.PersistentKeepaliveInterval = time.Duration(p.PersistentKeepalive) * time.Second
		}
		if p.Flags&ioctl.PeerHasProtocolVersion != 0 {
			peer.ProtocolVersion = int(p.ProtocolVersion)
		}
		peer.TransmitBytes = int64(p.TxBytes)
		peer.ReceiveBytes = int64(p.RxBytes)
		if p.LastHandshake != 0 {
			peer.LastHandshakeTime = time.Unix(0, int64((p.LastHandshake-116444736000000000)*100))
		}
		var a *ioctl.AllowedIP
		for j := uint32(0); j < p.AllowedIPsCount; j++ {
			if a == nil {
				a = p.FirstAllowedIP()
			} else {
				a = a.NextAllowedIP()
			}
			var ip net.IP
			var bits int
			if a.AddressFamily == windows.AF_INET {
				ip = a.Address[:4]
				bits = 32
			} else if a.AddressFamily == windows.AF_INET6 {
				ip = a.Address[:16]
				bits = 128
			}
			peer.AllowedIPs = append(peer.AllowedIPs, net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(int(a.Cidr), bits),
			})
		}
		device.Peers = append(device.Peers, peer)
	}
	return &device, nil
}

// ConfigureDevice implements wginternal.Client.
func (c *Client) ConfigureDevice(name string, cfg wgtypes.Config) error {
	handle, err := c.interfaceHandle(name)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	preallocation := unsafe.Sizeof(ioctl.Interface{}) + uintptr(len(cfg.Peers))*unsafe.Sizeof(ioctl.Peer{})
	for i := range cfg.Peers {
		preallocation += uintptr(len(cfg.Peers[i].AllowedIPs)) * unsafe.Sizeof(ioctl.AllowedIP{})
	}
	var b ioctl.ConfigBuilder
	b.Preallocate(uint32(preallocation))
	interfaze := &ioctl.Interface{PeerCount: uint32(len(cfg.Peers))}
	if cfg.ReplacePeers {
		interfaze.Flags |= ioctl.InterfaceReplacePeers
	}
	if cfg.PrivateKey != nil {
		interfaze.PrivateKey = *cfg.PrivateKey
		interfaze.Flags |= ioctl.InterfaceHasPrivateKey
	}
	if cfg.ListenPort != nil {
		interfaze.ListenPort = uint16(*cfg.ListenPort)
		interfaze.Flags |= ioctl.InterfaceHasListenPort
	}
	b.AppendInterface(interfaze)
	for i := range cfg.Peers {
		peer := &ioctl.Peer{
			Flags:           ioctl.PeerHasPublicKey,
			PublicKey:       cfg.Peers[i].PublicKey,
			AllowedIPsCount: uint32(len(cfg.Peers[i].AllowedIPs)),
		}
		if cfg.Peers[i].ReplaceAllowedIPs {
			peer.Flags |= ioctl.PeerReplaceAllowedIPs
		}
		if cfg.Peers[i].UpdateOnly {
			peer.Flags |= ioctl.PeerUpdate
		}
		if cfg.Peers[i].Remove {
			peer.Flags |= ioctl.PeerRemove
		}
		if cfg.Peers[i].PresharedKey != nil {
			peer.Flags |= ioctl.PeerHasPresharedKey
			peer.PresharedKey = *cfg.Peers[i].PresharedKey
		}
		if cfg.Peers[i].Endpoint != nil {
			peer.Flags |= ioctl.PeerHasEndpoint
			peer.Endpoint.SetIP(cfg.Peers[i].Endpoint.IP, uint16(cfg.Peers[i].Endpoint.Port))
		}
		if cfg.Peers[i].PersistentKeepaliveInterval != nil {
			peer.Flags |= ioctl.PeerHasPersistentKeepalive
			peer.PersistentKeepalive = uint16(*cfg.Peers[i].PersistentKeepaliveInterval / time.Second)
		}
		b.AppendPeer(peer)
		for j := range cfg.Peers[i].AllowedIPs {
			var family ioctl.AddressFamily
			var ip net.IP
			if ip = cfg.Peers[i].AllowedIPs[j].IP.To4(); ip != nil {
				family = windows.AF_INET
			} else if ip = cfg.Peers[i].AllowedIPs[j].IP.To16(); ip != nil {
				family = windows.AF_INET6
			} else {
				ip = cfg.Peers[i].AllowedIPs[j].IP
			}
			cidr, _ := cfg.Peers[i].AllowedIPs[j].Mask.Size()
			a := &ioctl.AllowedIP{
				AddressFamily: family,
				Cidr:          uint8(cidr),
			}
			copy(a.Address[:], ip)
			b.AppendAllowedIP(a)
		}
	}
	interfaze, size := b.Interface()
	return windows.DeviceIoControl(handle, ioctl.IoctlSet, nil, 0, (*byte)(unsafe.Pointer(interfaze)), size, &size, nil)
}
