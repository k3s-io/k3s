package ioprogress

import (
	"io"
)

// ProgressWriter is a wrapper around WriteCloser which allows for progress tracking
type ProgressWriter struct {
	io.WriteCloser
	Tracker *ProgressTracker
}

// Write in ProgressWriter is the same as io.Write
func (pt *ProgressWriter) Write(p []byte) (int, error) {
	// Do normal writer tasks
	n, err := pt.WriteCloser.Write(p)

	// Do the actual progress tracking
	if pt.Tracker != nil {
		pt.Tracker.total += int64(n)
		pt.Tracker.update(n)
	}

	return n, err
}
