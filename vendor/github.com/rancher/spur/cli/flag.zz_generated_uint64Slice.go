package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Uint64Slice is a type alias for []uint64
type Uint64Slice = []uint64

// Uint64SliceFlag is a flag with type []uint64
type Uint64SliceFlag struct {
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

	Value       Uint64Slice
	Destination *Uint64Slice
}

// Apply populates the flag given the flag set and environment
func (f *Uint64SliceFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "uint64 slice", set)
}

// Uint64Slice looks up the value of a local Uint64SliceFlag, returns
// an empty value if not found
func (c *Context) Uint64Slice(name string) []uint64 {
	return c.Lookup(name, *new(Uint64Slice)).([]uint64)
}
