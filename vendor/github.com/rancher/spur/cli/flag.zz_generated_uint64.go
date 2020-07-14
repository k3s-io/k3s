package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Uint64 is a type alias for uint64
type Uint64 = uint64

// Uint64Flag is a flag with type uint64
type Uint64Flag struct {
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

	Value       Uint64
	Destination *Uint64
}

// Apply populates the flag given the flag set and environment
func (f *Uint64Flag) Apply(set *flag.FlagSet) error {
	return Apply(f, "uint64", set)
}

// Uint64 looks up the value of a local Uint64Flag, returns
// an empty value if not found
func (c *Context) Uint64(name string) uint64 {
	return c.Lookup(name, *new(Uint64)).(uint64)
}
