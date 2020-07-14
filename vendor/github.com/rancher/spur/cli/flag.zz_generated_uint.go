package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Uint is a type alias for uint
type Uint = uint

// UintFlag is a flag with type uint
type UintFlag struct {
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

	Value       Uint
	Destination *Uint
}

// Apply populates the flag given the flag set and environment
func (f *UintFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "uint", set)
}

// Uint looks up the value of a local UintFlag, returns
// an empty value if not found
func (c *Context) Uint(name string) uint {
	return c.Lookup(name, *new(Uint)).(uint)
}
