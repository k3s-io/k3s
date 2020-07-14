package cli

import (
	"time"

	"github.com/rancher/spur/flag"
)

var _ = time.Time{}

// TimeSlice is a type alias for []time.Time
type TimeSlice = []time.Time

// TimeSliceFlag is a flag with type []time.Time
type TimeSliceFlag struct {
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

	Value       TimeSlice
	Destination *TimeSlice
}

// Apply populates the flag given the flag set and environment
func (f *TimeSliceFlag) Apply(set *flag.FlagSet) error {
	return Apply(f, "time slice", set)
}

// TimeSlice looks up the value of a local TimeSliceFlag, returns
// an empty value if not found
func (c *Context) TimeSlice(name string) []time.Time {
	return c.Lookup(name, *new(TimeSlice)).([]time.Time)
}
