package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Int64 is a type alias for int64
type Int64 = int64

// Int64Flag is a flag with type int64
type Int64Flag struct {
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

	Value       Int64
	Destination *Int64
}

// Apply populates the flag given the flag set and environment
func (f *Int64Flag) Apply(set *flag.FlagSet) error {
	return Apply(f, "int64", set)
}

// Int64 looks up the value of a local Int64Flag, returns
// an empty value if not found
func (c *Context) Int64(name string) int64 {
	return c.Lookup(name, *new(Int64)).(int64)
}
