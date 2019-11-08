package ioprogress

// The ProgressData struct represents new progress information on an operation
type ProgressData struct {
	// Preferred string repreentation of progress (always set)
	Text string

	// Progress in percent
	Percentage int

	// Number of bytes transferred (for files)
	TransferredBytes int64

	// Total number of bytes (for files)
	TotalBytes int64
}
