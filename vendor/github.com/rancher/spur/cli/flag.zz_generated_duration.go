package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Duration is a type alias for time.Duration
type Duration = time.Duration

// DurationFlag is a flag with type time.Duration
type DurationFlag struct {
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

	Value       Duration
	Destination *Duration
}

// Apply populates the flag given the flag set and environment
func (f *DurationFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "duration", set)
}

// Duration looks up the value of a local DurationFlag, returns
// an empty value if not found
func (c *Context) Duration(name string) time.Duration {
	return c.Lookup(name, *new(Duration)).(time.Duration)
}
