package protocol

import (
	"time"
)

// Config holds various configuration parameters for a dqlite client.
type Config struct {
	Dial           DialFunc      // Network dialer.
	DialTimeout    time.Duration // Timeout for establishing a network connection .
	AttemptTimeout time.Duration // Timeout for each individual attempt to probe a server's leadership.
	BackoffFactor  time.Duration // Exponential backoff factor for retries.
	BackoffCap     time.Duration // Maximum connection retry backoff value,
	RetryLimit     uint          // Maximum number of retries, or 0 for unlimited.
}
