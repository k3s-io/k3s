package cli

import (
	"github.com/rancher/spur/flag"
)

// Generic is a type alias for flag.Value
type Generic = flag.Value

// GenericFlag is a flag with type flag.Value
type GenericFlag struct {
	Name        string
	Aliases     []string
	EnvVars     []string
	Usage       string
	DefaultText string
	FilePath    string
	Required    bool
	Hidden      bool
	TakesFile   bool
	SkipAltSrc  bool

	Value       Generic
	Destination Generic
}

// Apply populates the flag given the flag set and environment
func (f *GenericFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "generic", set)
}

// Generic looks up the value of a local GenericFlag, returns
// an empty value if not found
func (c *Context) Generic(name string) interface{} {
	return c.Lookup(name, nil)
}
