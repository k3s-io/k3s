package api

// ContainerConsoleControl represents a message on the container console "control" socket
//
// API extension: console
type ContainerConsoleControl struct {
	Command string            `json:"command" yaml:"command"`
	Args    map[string]string `json:"args" yaml:"args"`
}

// ContainerConsolePost represents a LXD container console request
//
// API extension: console
type ContainerConsolePost struct {
	Width  int `json:"width" yaml:"width"`
	Height int `json:"height" yaml:"height"`
}
