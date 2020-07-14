package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Int is a type alias for int
type Int = int

// IntFlag is a flag with type int
type IntFlag struct {
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

	Value       Int
	Destination *Int
}

// Apply populates the flag given the flag set and environment
func (f *IntFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "int", set)
}

// Int looks up the value of a local IntFlag, returns
// an empty value if not found
func (c *Context) Int(name string) int {
	return c.Lookup(name, *new(Int)).(int)
}
