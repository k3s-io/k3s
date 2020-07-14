package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// StringSlice is a type alias for []string
type StringSlice = []string

// StringSliceFlag is a flag with type []string
type StringSliceFlag struct {
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

	Value       StringSlice
	Destination *StringSlice
}

// Apply populates the flag given the flag set and environment
func (f *StringSliceFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "string slice", set)
}

// StringSlice looks up the value of a local StringSliceFlag, returns
// an empty value if not found
func (c *Context) StringSlice(name string) []string {
	return c.Lookup(name, *new(StringSlice)).([]string)
}
