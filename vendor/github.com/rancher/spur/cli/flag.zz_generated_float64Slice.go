package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Float64Slice is a type alias for []float64
type Float64Slice = []float64

// Float64SliceFlag is a flag with type []float64
type Float64SliceFlag struct {
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

	Value       Float64Slice
	Destination *Float64Slice
}

// Apply populates the flag given the flag set and environment
func (f *Float64SliceFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "float64 slice", set)
}

// Float64Slice looks up the value of a local Float64SliceFlag, returns
// an empty value if not found
func (c *Context) Float64Slice(name string) []float64 {
	return c.Lookup(name, *new(Float64Slice)).([]float64)
}
