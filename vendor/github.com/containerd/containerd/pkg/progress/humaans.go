/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package progress

import (
	"fmt"
	"time"

	units "github.com/docker/go-units"
)

// Bytes converts a regular int64 to human readable type.
type Bytes int64

// String returns the string representation of bytes
func (b Bytes) String() string {
	return units.CustomSize("%02.1f %s", float64(b), 1024.0, []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"})
}

// BytesPerSecond is the rate in seconds for byte operations
type BytesPerSecond int64

// NewBytesPerSecond returns the rate that n bytes were written in the provided duration
func NewBytesPerSecond(n int64, duration time.Duration) BytesPerSecond {
	return BytesPerSecond(float64(n) / duration.Seconds())
}

// String returns the string representation of the rate
func (bps BytesPerSecond) String() string {
	return fmt.Sprintf("%v/s", Bytes(bps))
}
