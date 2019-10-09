package netlink

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// LinkAttrs represents data shared by most link types
type RdmaLinkAttrs struct {
	Index           uint32
	Name            string
	FirmwareVersion string
	NodeGuid        string
	SysImageGuid    string
}

// Link represents a rdma device from netlink.
type RdmaLink struct {
	Attrs RdmaLinkAttrs
}

func getProtoField(clientType int, op int) int {
	return ((clientType << nl.RDMA_NL_GET_CLIENT_SHIFT) | op)
}

func uint64ToGuidString(guid uint64) string {
	//Convert to byte array
	sysGuidBytes := new(bytes.Buffer)
	binary.Write(sysGuidBytes, binary.LittleEndian, guid)

	//Convert to HardwareAddr
	sysGuidNet := net.HardwareAddr(sysGuidBytes.Bytes())

	//Get the String
	return sysGuidNet.String()
}

func executeOneGetRdmaLink(data []byte) (*RdmaLink, error) {

	link := RdmaLink{}

	reader := bytes.NewReader(data)
	for reader.Len() >= 4 {
		_, attrType, len, value := parseNfAttrTLV(reader)

		switch attrType {
		case nl.RDMA_NLDEV_ATTR_DEV_INDEX:
			var Index uint32
			r := bytes.NewReader(value)
			binary.Read(r, nl.NativeEndian(), &Index)
			link.Attrs.Index = Index
		case nl.RDMA_NLDEV_ATTR_DEV_NAME:
			link.Attrs.Name = string(value[0 : len-1])
		case nl.RDMA_NLDEV_ATTR_FW_VERSION:
			link.Attrs.FirmwareVersion = string(value[0 : len-1])
		case nl.RDMA_NLDEV_ATTR_NODE_GUID:
			var guid uint64
			r := bytes.NewReader(value)
			binary.Read(r, nl.NativeEndian(), &guid)
			link.Attrs.NodeGuid = uint64ToGuidString(guid)
		case nl.RDMA_NLDEV_ATTR_SYS_IMAGE_GUID:
			var sysGuid uint64
			r := bytes.NewReader(value)
			binary.Read(r, nl.NativeEndian(), &sysGuid)
			link.Attrs.SysImageGuid = uint64ToGuidString(sysGuid)
		}
		if (len % 4) != 0 {
			// Skip pad bytes
			reader.Seek(int64(4-(len%4)), seekCurrent)
		}
	}
	return &link, nil
}

func execRdmaGetLink(req *nl.NetlinkRequest, name string) (*RdmaLink, error) {

	msgs, err := req.Execute(unix.NETLINK_RDMA, 0)
	if err != nil {
		return nil, err
	}
	for _, m := range msgs {
		link, err := executeOneGetRdmaLink(m)
		if err != nil {
			return nil, err
		}
		if link.Attrs.Name == name {
			return link, nil
		}
	}
	return nil, fmt.Errorf("Rdma device %v not found", name)
}

func execRdmaSetLink(req *nl.NetlinkRequest) error {

	_, err := req.Execute(unix.NETLINK_RDMA, 0)
	return err
}

// RdmaLinkByName finds a link by name and returns a pointer to the object if
// found and nil error, otherwise returns error code.
func RdmaLinkByName(name string) (*RdmaLink, error) {
	return pkgHandle.RdmaLinkByName(name)
}

// RdmaLinkByName finds a link by name and returns a pointer to the object if
// found and nil error, otherwise returns error code.
func (h *Handle) RdmaLinkByName(name string) (*RdmaLink, error) {

	proto := getProtoField(nl.RDMA_NL_NLDEV, nl.RDMA_NLDEV_CMD_GET)
	req := h.newNetlinkRequest(proto, unix.NLM_F_ACK|unix.NLM_F_DUMP)

	return execRdmaGetLink(req, name)
}

// RdmaLinkSetName sets the name of the rdma link device. Return nil on success
// or error otherwise.
// Equivalent to: `rdma dev set $old_devname name $name`
func RdmaLinkSetName(link *RdmaLink, name string) error {
	return pkgHandle.RdmaLinkSetName(link, name)
}

// RdmaLinkSetName sets the name of the rdma link device. Return nil on success
// or error otherwise.
// Equivalent to: `rdma dev set $old_devname name $name`
func (h *Handle) RdmaLinkSetName(link *RdmaLink, name string) error {
	proto := getProtoField(nl.RDMA_NL_NLDEV, nl.RDMA_NLDEV_CMD_SET)
	req := h.newNetlinkRequest(proto, unix.NLM_F_ACK)

	b := make([]byte, 4)
	native.PutUint32(b, uint32(link.Attrs.Index))
	data := nl.NewRtAttr(nl.RDMA_NLDEV_ATTR_DEV_INDEX, b)
	req.AddData(data)

	b = make([]byte, len(name)+1)
	copy(b, name)
	data = nl.NewRtAttr(nl.RDMA_NLDEV_ATTR_DEV_NAME, b)
	req.AddData(data)

	return execRdmaSetLink(req)
}
