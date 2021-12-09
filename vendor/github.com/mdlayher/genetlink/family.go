package genetlink

// A Family is a generic netlink family.
type Family struct {
	ID      uint16
	Version uint8
	Name    string
	Groups  []MulticastGroup
}

// A MulticastGroup is a generic netlink multicast group, which can be joined
// for notifications from generic netlink families when specific events take
// place.
type MulticastGroup struct {
	ID   uint32
	Name string
}
