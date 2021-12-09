//+build linux

package genetlink

import (
	"errors"
	"fmt"
	"math"

	"github.com/mdlayher/netlink"
	"github.com/mdlayher/netlink/nlenc"
	"golang.org/x/sys/unix"
)

// errInvalidFamilyVersion is returned when a family's version is greater
// than an 8-bit integer.
var errInvalidFamilyVersion = errors.New("invalid family version attribute")

// getFamily retrieves a generic netlink family with the specified name.
func (c *Conn) getFamily(name string) (Family, error) {
	b, err := netlink.MarshalAttributes([]netlink.Attribute{{
		Type: unix.CTRL_ATTR_FAMILY_NAME,
		Data: nlenc.Bytes(name),
	}})
	if err != nil {
		return Family{}, err
	}

	req := Message{
		Header: Header{
			Command: unix.CTRL_CMD_GETFAMILY,
			// TODO(mdlayher): grab nlctrl version?
			Version: 1,
		},
		Data: b,
	}

	msgs, err := c.Execute(req, unix.GENL_ID_CTRL, netlink.Request)
	if err != nil {
		return Family{}, err
	}

	// TODO(mdlayher): consider interpreting generic netlink header values

	families, err := buildFamilies(msgs)
	if err != nil {
		return Family{}, err
	}
	if len(families) != 1 {
		// If this were to ever happen, netlink must be in a state where
		// its answers cannot be trusted
		panic(fmt.Sprintf("netlink returned multiple families for name: %q", name))
	}

	return families[0], nil
}

// listFamilies retrieves all registered generic netlink families.
func (c *Conn) listFamilies() ([]Family, error) {
	req := Message{
		Header: Header{
			Command: unix.CTRL_CMD_GETFAMILY,
			// TODO(mdlayher): grab nlctrl version?
			Version: 1,
		},
	}

	flags := netlink.Request | netlink.Dump
	msgs, err := c.Execute(req, unix.GENL_ID_CTRL, flags)
	if err != nil {
		return nil, err
	}

	return buildFamilies(msgs)
}

// buildFamilies builds a slice of Families by parsing attributes from the
// input Messages.
func buildFamilies(msgs []Message) ([]Family, error) {
	families := make([]Family, 0, len(msgs))
	for _, m := range msgs {
		var f Family
		if err := (&f).parseAttributes(m.Data); err != nil {
			return nil, err
		}

		families = append(families, f)
	}

	return families, nil
}

// parseAttributes decodes netlink attributes into a Family's fields.
func (f *Family) parseAttributes(b []byte) error {
	ad, err := netlink.NewAttributeDecoder(b)
	if err != nil {
		return err
	}

	for ad.Next() {
		switch ad.Type() {
		case unix.CTRL_ATTR_FAMILY_ID:
			f.ID = ad.Uint16()
		case unix.CTRL_ATTR_FAMILY_NAME:
			f.Name = ad.String()
		case unix.CTRL_ATTR_VERSION:
			v := ad.Uint32()
			if v > math.MaxUint8 {
				return errInvalidFamilyVersion
			}

			f.Version = uint8(v)
		case unix.CTRL_ATTR_MCAST_GROUPS:
			ad.Nested(func(nad *netlink.AttributeDecoder) error {
				f.Groups = parseMulticastGroups(nad)
				return nil
			})
		}
	}

	return ad.Err()
}

// parseMulticastGroups parses an array of multicast group nested attributes
// into a slice of MulticastGroups.
func parseMulticastGroups(ad *netlink.AttributeDecoder) []MulticastGroup {
	groups := make([]MulticastGroup, 0, ad.Len())
	for ad.Next() {
		ad.Nested(func(nad *netlink.AttributeDecoder) error {
			var g MulticastGroup
			for nad.Next() {
				switch nad.Type() {
				case unix.CTRL_ATTR_MCAST_GRP_NAME:
					g.Name = nad.String()
				case unix.CTRL_ATTR_MCAST_GRP_ID:
					g.ID = nad.Uint32()
				}
			}

			groups = append(groups, g)
			return nil
		})
	}

	return groups
}
