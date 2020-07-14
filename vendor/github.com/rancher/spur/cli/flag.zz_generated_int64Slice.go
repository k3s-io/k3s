package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Int64Slice is a type alias for []int64
type Int64Slice = []int64

// Int64SliceFlag is a flag with type []int64
type Int64SliceFlag struct {
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

	Value       Int64Slice
	Destination *Int64Slice
}

// Apply populates the flag given the flag set and environment
func (f *Int64SliceFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "int64 slice", set)
}

// Int64Slice looks up the value of a local Int64SliceFlag, returns
// an empty value if not found
func (c *Context) Int64Slice(name string) []int64 {
	return c.Lookup(name, *new(Int64Slice)).([]int64)
}
