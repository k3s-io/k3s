package pongo2

// Options allow you to change the behavior of template-engine.
// You can change the options before calling the Execute method.
type Options struct {
	// If this is set to true the first newline after a block is removed (block, not variable tag!). Defaults to false.
	TrimBlocks bool

	// If this is set to true leading spaces and tabs are stripped from the start of a line to a block. Defaults to false
	LStripBlocks bool
}

func newOptions() *Options {
	return &Options{
		TrimBlocks:   false,
		LStripBlocks: false,
	}
}

// Update updates this options from another options.
func (opt *Options) Update(other *Options) *Options {
	opt.TrimBlocks = other.TrimBlocks
	opt.LStripBlocks = other.LStripBlocks

	return opt
}
