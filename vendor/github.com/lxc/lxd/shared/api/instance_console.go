package api

// InstanceConsoleControl represents a message on the instance console "control" socket.
//
// API extension: instances
type InstanceConsoleControl struct {
	Command string            `json:"command" yaml:"command"`
	Args    map[string]string `json:"args" yaml:"args"`
}

// InstanceConsolePost represents a LXD instance console request.
//
// API extension: instances
type InstanceConsolePost struct {
	Width  int `json:"width" yaml:"width"`
	Height int `json:"height" yaml:"height"`
}
