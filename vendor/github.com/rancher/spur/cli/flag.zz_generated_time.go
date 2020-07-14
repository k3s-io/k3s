package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// Time is a type alias for time.Time
type Time = time.Time

// TimeFlag is a flag with type time.Time
type TimeFlag struct {
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

	Value       Time
	Destination *Time
}

// Apply populates the flag given the flag set and environment
func (f *TimeFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "time", set)
}

// Time looks up the value of a local TimeFlag, returns
// an empty value if not found
func (c *Context) Time(name string) time.Time {
	return c.Lookup(name, *new(Time)).(time.Time)
}
