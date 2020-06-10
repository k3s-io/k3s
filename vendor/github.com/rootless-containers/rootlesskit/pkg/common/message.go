package common

// Message is sent from the parent to the child
// as JSON, with uint32le length header.
type Message struct {
	Stage int // 0 for Message 0, 1 for Message 1
	Message0
	Message1
}

// Message0 is sent after setting up idmap
type Message0 struct {
}

// Message 1 is sent after setting up other stuff
type Message1 struct {
	// StateDir cannot be empty
	StateDir string
	Network  NetworkMessage
	Port     PortMessage
}

// NetworkMessage is empty for HostNetwork.
type NetworkMessage struct {
	Dev     string
	IP      string
	Netmask int
	Gateway string
	DNS     string
	MTU     int
	// Opaque strings are specific to driver
	Opaque map[string]string
}

type PortMessage struct {
	Opaque map[string]string
}
