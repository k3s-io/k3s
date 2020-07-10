package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// IntSlice is a type alias for []int
type IntSlice = []int

// IntSliceFlag is a flag with type []int
type IntSliceFlag struct {
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

	Value       IntSlice
	Destination *IntSlice
}

// Apply populates the flag given the flag set and environment
func (f *IntSliceFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "int slice", set)
}

// IntSlice looks up the value of a local IntSliceFlag, returns
// an empty value if not found
func (c *Context) IntSlice(name string) []int {
	return c.Lookup(name, *new(IntSlice)).([]int)
}
