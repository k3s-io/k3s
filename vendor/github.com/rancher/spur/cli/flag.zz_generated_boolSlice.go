package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// BoolSlice is a type alias for []bool
type BoolSlice = []bool

// BoolSliceFlag is a flag with type []bool
type BoolSliceFlag struct {
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

	Value       BoolSlice
	Destination *BoolSlice
}

// Apply populates the flag given the flag set and environment
func (f *BoolSliceFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "bool slice", set)
}

// BoolSlice looks up the value of a local BoolSliceFlag, returns
// an empty value if not found
func (c *Context) BoolSlice(name string) []bool {
	return c.Lookup(name, *new(BoolSlice)).([]bool)
}
