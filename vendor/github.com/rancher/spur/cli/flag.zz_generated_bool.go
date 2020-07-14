package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Bool is a type alias for bool
type Bool = bool

// BoolFlag is a flag with type bool
type BoolFlag struct {
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

	Value       Bool
	Destination *Bool
}

// Apply populates the flag given the flag set and environment
func (f *BoolFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "bool", set)
}

// Bool looks up the value of a local BoolFlag, returns
// an empty value if not found
func (c *Context) Bool(name string) bool {
	return c.Lookup(name, *new(Bool)).(bool)
}
