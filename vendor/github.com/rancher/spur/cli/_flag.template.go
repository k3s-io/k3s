package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Title__ is a type alias for Type__
type Title__ = Type__

// Title__Flag is a flag with type Type__
type Title__Flag struct {
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

	Value       Title__
	Destination *Title__
}

// Apply populates the flag given the flag set and environment
func (f *Title__Flag) Apply(set *flag.FlagSet) error {
	return Apply(f, "LongName__", set)
}

// Title__ looks up the value of a local Title__Flag, returns
// an empty value if not found
func (c *Context) Title__(name string) Type__ {
	return c.Lookup(name, *new(Title__)).(Type__)
}
